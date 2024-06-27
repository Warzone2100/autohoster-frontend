package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"

	_ "strconv"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	scs "github.com/alexedwards/scs/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/maxsupermanhd/lac"
	"github.com/natefinch/lumberjack"
)

var (
	BuildTime  = "00000000.000000"
	CommitHash = "0000000"
	GoVersion  = "0.0"
	GitTag     = "0.0"
	BuildType  = "dev"
)

var (
	LobbyWSHub     *WSHub
	GamesWSHub     *WSHub
	layouts        *template.Template
	sessionManager *scs.SessionManager
	dbpool         *pgxpool.Pool
	cfg            *lac.Conf
)

func FormatPercent(p float64) string {
	return fmt.Sprintf("%.1f%%", p)
}

func ByteCountIEC(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func sessionAppendUser(r *http.Request, a map[string]any) map[string]any {
	if !checkUserAuthorized(r) {
		return nil
	}
	var sessid int
	var sessuname, sessemail, sesseconf string
	var hasIdentities bool
	sessuname = sessionGetUsername(r)
	err := dbpool.QueryRow(context.Background(), `SELECT id, email,
	coalesce(extract(epoch from email_confirmed), 0)::text
	FROM accounts WHERE username = $1`, sessuname).
		Scan(&sessid, &sessemail, &sesseconf)

	if err != nil {
		log.Println("sessionAppendUser (accounts): " + err.Error())
	} else {
		err = dbpool.QueryRow(context.Background(), `SELECT count(*) > 0 FROM identities WHERE account = $1`, sessid).Scan(&hasIdentities)
		if err != nil {
			log.Println("sessionAppendUser (identities): " + err.Error())
		}
	}
	a["User"] = map[string]any{
		"Username":    sessuname,
		"Id":          sessid,
		"Email":       sessemail,
		"Econf":       sesseconf,
		"HasIdentity": hasIdentities,
	}
	a["UserAuthorized"] = "True"
	a["IsSuperadmin"] = isSuperadmin(r.Context(), sessionGetUsername(r))
	return a
}

func robotsHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, "User-agent: *\nDisallow: /\n\n\n")
}
func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/favicon.ico")
}

//lint:ignore U1000 used
type statusRespWr struct {
	http.ResponseWriter
	status int
}

func (w *statusRespWr) writeHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func customLogger(_ io.Writer, params handlers.LogFormatterParams) {
	r := params.Request
	ip := r.Header.Get("CF-Connecting-IP")
	if ip == "" {
		ip = r.RemoteAddr
	}
	geo := r.Header.Get("CF-IPCountry")
	if geo == "" {
		geo = "??"
	}
	ua := r.Header.Get("user-agent")
	hash := r.Header.Get("WZ-Player-Hash")
	username := sessionGetUsername(r)
	log.Println("["+geo+" "+ip+"]", username, r.Method, params.StatusCode, r.RequestURI, "["+ua+"]", hash)
}

func shouldCache(maxage int, h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", fmt.Sprintf("public, max-age=%d", maxage))
		h.ServeHTTP(w, r)
	}
}

func accountMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !checkUserAuthorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		u := sessionGetUsername(r)
		go func() {
			tag, err := dbpool.Exec(context.Background(), "UPDATE accounts SET last_seen = now() WHERE username = $1", u)
			if err != nil {
				log.Printf("Failed to set last seen on user [%q]", u)
				return
			}
			if !tag.Update() || tag.RowsAffected() != 1 {
				log.Printf("Last seen update for [%q] is sus (%s)", u, tag.String())
			}
		}()
		var t bool
		err := dbpool.QueryRow(r.Context(), "SELECT terminated FROM accounts WHERE username = $1", u).Scan(&t)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("Error checking account terminated username [%q]: %s", u, err.Error())
				terminatedHandler(w, r)
				return
			}
		}
		if t {
			log.Printf("Terminated user performed request, username [%q]", u)
			terminatedHandler(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	rand.Seed(time.Now().UTC().UnixNano())
	var err error
	cfg, err = lac.FromFileJSON("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %s", err.Error())
	}
	DiscordVerifyEnv()

	log.SetOutput(io.MultiWriter(os.Stdout, &lumberjack.Logger{
		Filename: cfg.GetDSString("./logs/"+BuildType+".log", "logFile"),
		MaxSize:  10,   // megabytes
		Compress: true, // disabled by default
	}))

	log.Println()
	log.Println("TacticalPepe web server is starting up...")
	log.Printf("Built %s, Ver %s (%s) Go %s\n", BuildTime, GitTag, CommitHash, GoVersion)
	log.Println()

	researchClassification, err = LoadClassification()
	if err != nil {
		researchClassification = []map[string]string{}
		log.Println("Failed to load research classification: ", err)
	}

	log.Println("Loading layouts")
	layoutsDir := "layouts/"
	if dirstat, err := os.Stat("layouts-" + BuildType); !os.IsNotExist(err) && dirstat.IsDir() {
		layoutsDir = "layouts-" + BuildType + "/"
		log.Println("Using build-specific layouts directory (" + layoutsDir + ")")
	}
	layouts, err = template.New("main").Funcs(layoutFuncs).ParseGlob(layoutsDir + "*.gohtml")
	if err != nil {
		panic(err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					log.Println("Updating templates")
					nlayouts, err := template.New("main").Funcs(layoutFuncs).ParseGlob(layoutsDir + "*.gohtml")
					if err != nil {
						log.Println("Error while parsing templates:", err.Error())
					} else {
						layouts = nlayouts.Funcs(layoutFuncs)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()
	err = watcher.Add(layoutsDir)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Connecting to database")
	dbpool, err = pgxpool.Connect(context.Background(), cfg.GetDString("", "databaseConnString"))
	if err != nil {
		log.Fatal(err)
	}
	defer dbpool.Close()

	log.Println("Starting session manager")
	sessionManager = scs.New()
	store := pgxstore.New(dbpool)
	sessionManager.Store = store
	sessionManager.Lifetime = time.Hour * 24 * 60
	defer store.StopCleanup()

	log.Println("Starting websocket hubs")
	LobbyWSHub = NewWSHub()
	GamesWSHub = NewWSHub()
	go LobbyWSHub.Run()
	go GamesWSHub.Run()

	log.Println("Starting lobby poller")
	loadLobbyIgnores(cfg.GetDSString("./lobbyIgnores.txt", "lobbyIgnores"))
	go lobbyPoller()

	log.Println("Loading research names")
	prepareStatNames()

	log.Println("Adding routes")
	router := mux.NewRouter()
	router.NotFoundHandler = myNotFoundHandler()
	router.PathPrefix("/static").Handler(http.StripPrefix("/static/", shouldCache(604800, http.FileServer(http.Dir("./static")))))
	router.HandleFunc("/favicon.ico", faviconHandler)
	router.HandleFunc("/robots.txt", robotsHandler)
	router.HandleFunc("/", indexHandler)

	router.HandleFunc("/legal", basicLayoutHandler("legal"))
	router.HandleFunc("/about", basicLayoutHandler("about"))
	router.HandleFunc("/rules", basicLayoutHandler("rules"))
	router.HandleFunc("/rules/ru", basicLayoutHandler("rulesRU"))
	router.HandleFunc("/login", loginHandler)
	router.HandleFunc("/logout", logoutHandler)
	router.HandleFunc("/register", registerHandler)
	router.HandleFunc("/account", basicLayoutHandler("account"))
	router.HandleFunc("/activate", emailconfHandler)
	router.HandleFunc("/recover", recoverPasswordHandler)
	router.HandleFunc("/oauth/discord", DiscordCallbackHandler)
	router.HandleFunc("/report", basicLayoutHandler("report")).Methods("GET")
	router.HandleFunc("/report", reportHandler).Methods("POST")

	router.HandleFunc("/hoster", hosterHandler)
	router.HandleFunc("/request", hostRequestHandler)
	router.HandleFunc("/wzlink", wzlinkHandler)
	router.HandleFunc("/wzlinkcheck", wzlinkCheckHandler)
	router.HandleFunc("/wzrecover", wzProfileRecoveryHandlerGET)
	router.HandleFunc("/autohoster", basicLayoutHandler("autohoster-control"))

	// moderation endpoints
	router.HandleFunc("/moderation/accounts", basicSuperadminHandler("modAccounts")).Methods("GET")
	router.HandleFunc("/moderation/accounts", SuperadminCheck(modAccountsPOST)).Methods("POST")
	router.HandleFunc("/moderation/accounts/resendEmail/{id:[0-9]+}", APIcall(APISuperadminCheck(APIresendEmailConfirm)))
	router.HandleFunc("/api/accounts", APIcall(APISuperadminCheck(APIgetAccounts))).Methods("GET", "OPTIONS")

	router.HandleFunc("/moderation/news", basicSuperadminHandler("modNews")).Methods("GET")
	router.HandleFunc("/moderation/news", SuperadminCheck(modNewsPOST)).Methods("POST")

	router.HandleFunc("/moderation/logs", basicSuperadminHandler("modLogs")).Methods("GET")
	router.HandleFunc("/api/logs", APIcall(APISuperadminCheck(APIgetLogs))).Methods("GET", "OPTIONS")

	router.HandleFunc("/moderation/bans", basicSuperadminHandler("modBans")).Methods("GET")
	router.HandleFunc("/moderation/bans", SuperadminCheck(modBansPOST))
	router.HandleFunc("/api/bans", APIcall(APIgetBans)).Methods("GET", "OPTIONS")

	router.HandleFunc("/moderation/identities", basicSuperadminHandler("modIdentities")).Methods("GET")
	router.HandleFunc("/moderation/identities", modIdentitiesHandler).Methods("POST")
	router.HandleFunc("/api/identities", APIcall(APIgetIdentities)).Methods("GET", "OPTIONS")

	router.HandleFunc("/moderation/ratingCategories", basicSuperadminHandler("modRatingCategories")).Methods("GET")
	router.HandleFunc("/api/ratingCategories", APIcall(APIgetRatingCategories)).Methods("GET", "OPTIONS")

	router.HandleFunc("/rating/{hash:[0-9a-z]+}", ratingHandler)
	router.HandleFunc("/rating/", ratingHandler)
	router.HandleFunc("/lobby", lobbyHandler)

	router.HandleFunc("/games", DbGamesHandler)
	router.HandleFunc("/games/{id:[0-9]+}", DbGameDetailsHandler)
	router.HandleFunc("/api/games", APIcall(APIgetGames)).Methods("GET", "OPTIONS")

	router.HandleFunc("/leaderboards", LeaderboardsHandler)
	router.HandleFunc("/leaderboards/{category:[0-9]+}", LeaderboardHandler).Methods("GET")
	router.HandleFunc("/api/leaderboards/{category:[0-9]+}", APIcall(APIgetLeaderboard)).Methods("GET", "OPTIONS")
	// router.HandleFunc("/account/{id:[0-9]+}", PlayerHandler)
	// router.HandleFunc("/stats", statsHandler)
	// router.HandleFunc("/resstat", resstatHandler)
	router.HandleFunc("/bans", bansHandler)

	// router.HandleFunc("/b/begin", GameAcceptCreateHandler)
	// router.HandleFunc("/b/frame/{gid:[0-9]+}", GameAcceptFrameHandler)
	// router.HandleFunc("/b/end/{gid:[0-9]+}", GameAcceptEndHandler)

	router.HandleFunc("/api/ws/lobby", func(w http.ResponseWriter, r *http.Request) {
		APIWSHub(LobbyWSHub, w, r)
	})
	// router.HandleFunc("/api/ws/games", func(w http.ResponseWriter, r *http.Request) {
	// 	APIWSHub(GamesWSHub, w, r)
	// })

	router.HandleFunc("/api/multihoster/alive", APItryReachMultihoster).Methods("GET")

	router.HandleFunc("/api/graph/{gid:[0-9]+}", APIcall(APIgetGraphData)).Methods("GET")
	router.HandleFunc("/api/classify/game/{gid:[0-9]+}", APIcall(APIgetClassChartGame)).Methods("GET")
	router.HandleFunc("/api/classify/player/{pid:[0-9]+}", APIcall(APIresearchClassification)).Methods("GET")
	router.HandleFunc("/api/reslog/{gid:[0-9]+}", APIgetResearchlogData).Methods("GET")
	router.HandleFunc("/api/replay/{gid:[0-9]+}", APIcall(APIgetReplayFile)).Methods("GET")
	router.HandleFunc("/api/heatmap/{gid:[0-9]+}", APIcall(APIgetReplayHeatmap)).Methods("GET")
	router.HandleFunc("/api/animatedheatmap/{gid:[0-9]+}", APIcall(APIgetAnimatedReplayHeatmap)).Methods("GET")
	router.HandleFunc("/api/animatedheatmap/{gid:[0-9]+}", APIcall(APIheadAnimatedReplayHeatmap)).Methods("HEAD")

	router.HandleFunc("/api/gamecount/{interval}", APIcall(APIgetDatesGraphData)).Methods("GET")
	router.HandleFunc("/api/dayavg", APIcall(APIgetDayAverageByHour)).Methods("GET")
	router.HandleFunc("/api/playersavg", APIcall(APIgetUniquePlayersPerDay)).Methods("GET")
	router.HandleFunc("/api/mapcount", APIcall(APIgetMapNameCount)).Methods("GET")

	router.HandleFunc("/elo/calc", EloRecalcHandler)

	// handlers.CompressHandler(router1)
	// handlers.RecoveryHandler()(router3)
	routerMiddle := sessionManager.LoadAndSave(handlers.CustomLoggingHandler(os.Stdout, handlers.ProxyHeaders(accountMiddleware(router)), customLogger))
	listenAddr := ":" + cfg.GetDSString("3001", "httpPort")
	log.Printf("Started web server at %s", listenAddr)
	log.Panic(http.ListenAndServe(listenAddr, routerMiddle))
}
