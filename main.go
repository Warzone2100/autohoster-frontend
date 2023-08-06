package main

import (
	"context"
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
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
	"github.com/natefinch/lumberjack"
	"golang.org/x/oauth2"
)

var (
	BuildTime  = "00000000.000000"
	CommitHash = "0000000"
	GoVersion  = "0.0"
	GitTag     = "0.0"
	BuildType  = "dev"
)

var (
	LobbyWSHub *WSHub
	GamesWSHub *WSHub
)

var layouts *template.Template
var sessionManager *scs.SessionManager
var dbpool *pgxpool.Pool

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

func getWzProfile(context context.Context, id int, table string) map[string]interface{} {
	var name, hash string
	var played, wins, losses, elo, elo2 int
	var pl map[string]interface{}
	var derr error
	req := "SELECT name, hash, autoplayed, autowon, autolost, elo, coalesce(elo2, 1400) FROM " + table + " WHERE id = $1"
	derr = dbpool.QueryRow(context, req, id).
		Scan(&name, &hash, &played, &wins, &losses, &elo, &elo2)
	if derr != nil {
		if derr != pgx.ErrNoRows {
			log.Println("getWzProfile: " + derr.Error())
		}
		return pl
	}
	pl = map[string]interface{}{
		"ID":         id,
		"Name":       name,
		"Hash":       hash,
		"Autoplayed": played,
		"Autowon":    wins,
		"Autolost":   losses,
		"Elo":        elo,
		"Elo2":       elo2,
	}
	return pl
}

func sessionAppendUser(r *http.Request, a map[string]interface{}) map[string]interface{} {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		return nil
	}
	var sessid int
	var sessuname string
	var sessemail string
	var sesseconf string
	var sessdisctoken string
	var sessdiscrefreshtoken string
	var sessdiscrefreshwhenepoch int
	var sessdiscstate string
	var sessdiscurl string
	var sesswzprofile int
	var sesswzprofile2 int
	var sessdisc map[string]interface{}
	var sessvktoken string
	var sessvkurl string
	var sessvkuid int
	var sessvk map[string]interface{}

	sessuname = sessionManager.GetString(r.Context(), "User.Username")
	log.Printf("User: [%s]", sessuname)
	derr := dbpool.QueryRow(context.Background(), `
		SELECT id, email,
		coalesce(extract(epoch from email_confirmed), 0)::text,
		coalesce(discord_token, ''),
		coalesce(discord_refresh, ''),
		coalesce(extract(epoch from discord_refresh_date), 0)::int,
		coalesce(wzprofile, -1), coalesce(wzprofile2, -1),
		coalesce(vk_token, ''), coalesce(vk_uid, -1)
		FROM users WHERE username = $1`, sessuname).
		Scan(&sessid, &sessemail, &sesseconf,
			&sessdisctoken, &sessdiscrefreshtoken, &sessdiscrefreshwhenepoch,
			&sesswzprofile, &sesswzprofile2,
			&sessvktoken, &sessvkuid)
	if derr != nil {
		log.Println("sessionAppendUser: " + derr.Error())
	}
	sessdiscrefreshwhen := time.Unix(int64(sessdiscrefreshwhenepoch), 0)
	if sessdisctoken == "" || sessdiscrefreshtoken == "" {
		sessdiscstate = generateRandomString(32)
		sessdiscurl = DiscordGetUrl(sessdiscstate)
		sessionManager.Put(r.Context(), "User.Discord.State", sessdiscstate)
	} else {
		token := oauth2.Token{AccessToken: sessdisctoken, RefreshToken: sessdiscrefreshtoken, Expiry: sessdiscrefreshwhen}
		tokenold := token
		sessdisc = DiscordGetUInfo(&token)
		if token.AccessToken != tokenold.AccessToken || token.RefreshToken != tokenold.RefreshToken || token.Expiry != tokenold.Expiry {
			log.Println("Discord token refreshed")
			tag, derr := dbpool.Exec(context.Background(), "UPDATE users SET discord_token = $1, discord_refresh = $2, discord_refresh_date = $3 WHERE username = $4", token.AccessToken, token.RefreshToken, token.Expiry, sessionManager.Get(r.Context(), "User.Username"))
			if derr != nil {
				log.Println("Database call error: " + derr.Error())
			}
			if tag.RowsAffected() != 1 {
				log.Println("Database update error, rows affected " + string(tag))
			}
		}
		if token.AccessToken == "" {
			sessdiscstate = generateRandomString(32)
			sessdiscurl = DiscordGetUrl(sessdiscstate)
			sessionManager.Put(r.Context(), "User.Discord.State", sessdiscstate)
		}
		sessdisctoken = token.AccessToken
	}
	wzprofile := getWzProfile(r.Context(), sesswzprofile, "old_players3")
	if wzprofile != nil {
		wzprofile["Userid"] = sessid
	}
	wzprofile2 := getWzProfile(r.Context(), sesswzprofile2, "players")
	if wzprofile2 != nil {
		wzprofile2["Userid"] = sessid
	}
	a["User"] = map[string]interface{}{
		"Username":   sessuname,
		"Id":         sessid,
		"Email":      sessemail,
		"Econf":      sesseconf,
		"WzProfile":  wzprofile,
		"WzProfile2": wzprofile2,
		"Discord": map[string]interface{}{
			"Token":   sessdisctoken,
			"AuthUrl": sessdiscurl,
			"Data":    sessdisc,
		},
		"VK": map[string]interface{}{
			"Token":   sessvktoken,
			"AuthUrl": sessvkurl,
			"Data":    sessvk,
		},
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
	geo := r.Header.Get("CF-IPCountry")
	ua := r.Header.Get("user-agent")
	hash := r.Header.Get("WZ-Player-Hash")
	if hash != "" {
		log.Println("["+geo+" "+ip+"]", r.Method, params.StatusCode, r.RequestURI, "["+ua+"]", hash)
	} else {
		log.Println("["+geo+" "+ip+"]", r.Method, params.StatusCode, r.RequestURI, "["+ua+"]")
	}
}

func shouldCache(maxage int, h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "public, max-age=604800")
		h.ServeHTTP(w, r)
	}
}

func accountMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if checkUserAuthorized(r) {
			u := sessionGetUsername(r)
			go func() {
				tag, err := dbpool.Exec(context.Background(), "UPDATE users SET last_seen = now() WHERE username = $1", u)
				if err != nil {
					log.Println("Failed to set last seen on user [", u, "]")
					return
				}
				if !tag.Update() || tag.RowsAffected() != 1 {
					log.Println("Last seen update for [", u, "] is sus (", tag.String(), ")")
				}
			}()
			var t bool
			err := dbpool.QueryRow(r.Context(), "SELECT terminated FROM users WHERE username = $1", u).Scan(&t)
			if err != nil {
				log.Println("Error checking account terminated username [", u, "]:", err)
				terminatedHandler(w, r)
				return
			}
			if t {
				log.Println("Terminated user performed request, username [", u, "]")
				terminatedHandler(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	rand.Seed(time.Now().UTC().UnixNano())
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	DiscordVerifyEnv()

	logsLocation := "./logs/" + BuildType + ".log"
	if os.Getenv("TPWSLOGFILE") != "" {
		logsLocation = os.Getenv("LOGFILE")
	}
	log.SetOutput(io.MultiWriter(os.Stdout, &lumberjack.Logger{
		Filename: logsLocation,
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
	dbpool, err = pgxpool.Connect(context.Background(), os.Getenv("DB"))
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

	log.Println("Starting lobby pooler")
	loadLobbyIgnores(os.Getenv("LOBBYIGNORES"))
	go lobbyPooler()

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
	router.HandleFunc("/preset-edit", presetEditorHandler)

	router.HandleFunc("/moderation/users", modUsersHandler)
	router.HandleFunc("/moderation/users/resendEmail/{id:[0-9]+}", APIcall(APIresendEmailConfirm))
	router.HandleFunc("/moderation/merge", modMergeHandler)
	router.HandleFunc("/moderation/news", modNewsHandler)
	router.HandleFunc("/moderation/logs", basicLayoutHandler("modLogs"))
	router.HandleFunc("/moderation/bans", modBansHandler)

	router.HandleFunc("/rating/{hash:[0-9a-z]+}", ratingHandler)
	router.HandleFunc("/rating/", ratingHandler)
	router.HandleFunc("/lobby", lobbyHandler)
	router.HandleFunc("/games", DbGamesHandler)
	router.HandleFunc("/games/{id:[0-9]+}", DbGameDetailsHandler)
	router.HandleFunc("/players", basicLayoutHandler("players"))
	router.HandleFunc("/players/{id:[0-f]+}", PlayersHandler)
	router.HandleFunc("/stats", statsHandler)
	router.HandleFunc("/resstat", resstatHandler)

	router.HandleFunc("/b/begin", GameAcceptCreateHandler)
	router.HandleFunc("/b/frame/{gid:[0-9]+}", GameAcceptFrameHandler)
	router.HandleFunc("/b/end/{gid:[0-9]+}", GameAcceptEndHandler)

	router.HandleFunc("/api/ws/lobby", func(w http.ResponseWriter, r *http.Request) {
		APIWSHub(LobbyWSHub, w, r)
	})
	router.HandleFunc("/api/ws/games", func(w http.ResponseWriter, r *http.Request) {
		APIWSHub(GamesWSHub, w, r)
	})
	router.HandleFunc("/api/graph/{gid:[0-9]+}", APIcall(APIgetGraphData)).Methods("GET")
	router.HandleFunc("/api/classify/game/{gid:[0-9]+}", APIcall(APIgetClassChartGame)).Methods("GET")
	router.HandleFunc("/api/classify/player/{pid:[0-9]+}", APIcall(APIresearchClassification)).Methods("GET")
	router.HandleFunc("/api/reslog/{gid:[0-9]+}", APIgetResearchlogData).Methods("GET")
	router.HandleFunc("/api/gamecount/{interval}", APIcall(APIgetDatesGraphData)).Methods("GET")
	router.HandleFunc("/api/multihoster/alive", APItryReachMultihoster).Methods("GET")
	router.HandleFunc("/api/dayavg", APIcall(APIgetDayAverageByHour)).Methods("GET")
	router.HandleFunc("/api/playersavg", APIcall(APIgetUniquePlayersPerDay)).Methods("GET")
	router.HandleFunc("/api/mapcount", APIcall(APIgetMapNameCount)).Methods("GET")
	router.HandleFunc("/api/replay/{gid:[0-9]+}", APIcall(APIgetReplayFile)).Methods("GET")
	router.HandleFunc("/api/heatmap/{gid:[0-9]+}", APIcall(APIgetReplayHeatmap)).Methods("GET")
	router.HandleFunc("/api/animatedheatmap/{gid:[0-9]+}", APIcall(APIgetAnimatedReplayHeatmap)).Methods("GET")
	router.HandleFunc("/api/animatedheatmap/{gid:[0-9]+}", APIcall(APIheadAnimatedReplayHeatmap)).Methods("HEAD")
	router.HandleFunc("/api/hashinfo/{hash:[0-9a-z]+}", APIcall(APIgetHashInfo)).Methods("GET")
	router.HandleFunc("/api/allowjoining/{hash:[0-9a-z]+}", APIcall(APIgetPlayerAllowedJoining)).Methods("GET")
	router.HandleFunc("/api/approvedhashes", APIcall(APIgetAllowedModerators)).Methods("GET")
	router.HandleFunc("/api/linkedhashes", APIcall(APIgetLinkedPlayers)).Methods("GET")
	router.HandleFunc("/api/islinked/{hash:[0-9a-z]+}", APIcall(APIgetPlayerLinked)).Methods("GET")
	router.HandleFunc("/api/ispbypasshashes", APIcall(APIgetISPbypassHashes)).Methods("GET")
	router.HandleFunc("/api/ispbypass/{hash:[0-9a-z]+}", APIcall(APIgetISPbypassHash)).Methods("GET")
	router.HandleFunc("/api/elohistory/{pid:[0-9]+}", APIcall(APIgetElodiffChartPlayer)).Methods("GET")
	router.HandleFunc("/api/players", APIcall(APIgetLeaderboard)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/games", APIcall(APIgetGames)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/users", APIcall(APIgetUsers)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/logs", APIcall(APIgetLogs)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/bans", APIcall(APIgetBans)).Methods("GET", "OPTIONS")

	router.HandleFunc("/elo/calc", EloRecalcHandler)

	// handlers.CompressHandler(router1)
	// handlers.RecoveryHandler()(router3)
	routerMiddle := sessionManager.LoadAndSave(handlers.CustomLoggingHandler(os.Stdout, handlers.ProxyHeaders(accountMiddleware(router)), customLogger))
	log.Println("Started!")
	log.Panic(http.ListenAndServe(":"+port, routerMiddle))
}
