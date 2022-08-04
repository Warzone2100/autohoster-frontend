package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect"

	//lint:ignore ST1019 warning
	"strconv"
	_ "strconv"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	scs "github.com/alexedwards/scs/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/imdario/mergo"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
	"github.com/natefinch/lumberjack"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/v3/mem"
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
var layoutFuncs = template.FuncMap{
	"noescape": func(s string) template.HTML {
		return template.HTML(s)
	},
	"inc": func(i int) int {
		return i + 1
	},
	"dec": func(i int) int {
		return i - 1
	},
	"decf64": func(i float64) float64 {
		return i - 1
	},
	"sum": func(a int, b int) int {
		return a + b
	},
	"sub": func(a int, b int) int {
		return a - b
	},
	"div": func(a int, b int) int {
		return a / b
	},
	"divf64": func(a float64, b float64) float64 {
		return a / b
	},
	"mult": func(a int, b int) int {
		return a * b
	},
	"rem": func(a int, b int) int {
		return a % b
	},
	"allianceToClass": func(a float64) float64 {
		if a == 3 {
			return 1
		} else {
			return a
		}
	},
	"allianceToClassI": func(a int) int {
		if a == 3 {
			return 1
		} else {
			return a
		}
	},
	"boolto10": func(a bool) int {
		if !a {
			return 0
		} else {
			return 1
		}
	},
	"f64tostring": func(a float64) string {
		return fmt.Sprintf("%.2f", a)
	},
	"avail": func(name string, data interface{}) bool {
		v := reflect.ValueOf(data)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		m, ok := data.(map[string]interface{})
		if ok {
			_, ok := m[name]
			return ok
		}
		if v.Kind() != reflect.Struct {
			return false
		}
		return v.FieldByName(name).IsValid()
	},
	"GameTimeToString":  GameTimeToString,
	"GameTimeToStringI": GameTimeToStringI,
	"strcut": func(str string, num int) string { // https://play.golang.org/p/EzvhWMljku
		bnoden := str
		if len(str) > num {
			if num > 3 {
				num -= 3
			}
			bnoden = str[0:num] + "..."
		}
		return bnoden
	},
	"FormatBytes":   ByteCountIEC,
	"FormatPercent": FormatPercent,
	"tostr": func(val interface{}) string {
		v := reflect.ValueOf(val)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if d, ok := val.(uint32); ok {
			return fmt.Sprint(d)
		}
		if d, ok := val.(float64); ok {
			return fmt.Sprint(d)
		}
		return "snan"
	},
}

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

func getWzProfile(id int, table string) map[string]interface{} {
	var name string
	var hash string
	var played int
	var wins int
	var losses int
	var elo int
	var pl map[string]interface{}
	var derr error
	req := "SELECT name, hash, autoplayed, autowon, autolost, elo FROM " + table + " WHERE id = $1"
	derr = dbpool.QueryRow(context.Background(), req, id).
		Scan(&name, &hash, &played, &wins, &losses, &elo)
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
	}
	return pl
}

func sessionAppendUser(r *http.Request, a *map[string]interface{}) *map[string]interface{} {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		return nil
	}
	var sessid int
	var sessuname string
	var sessfname string
	var sesslname string
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

	if sessionManager.Exists(r.Context(), "User.Username") {
		sessuname = sessionManager.GetString(r.Context(), "User.Username")
		log.Printf("User: [%s]", sessuname)
		derr := dbpool.QueryRow(context.Background(), `
			SELECT id, email, fname, lname,
			coalesce(extract(epoch from email_confirmed), 0)::text,
			coalesce(discord_token, ''),
			coalesce(discord_refresh, ''),
			coalesce(extract(epoch from discord_refresh_date), 0)::int,
			coalesce(wzprofile, -1), coalesce(wzprofile2, -1),
			coalesce(vk_token, ''), coalesce(vk_uid, -1)
			FROM users WHERE username = $1`, sessuname).
			Scan(&sessid, &sessemail, &sessfname, &sesslname, &sesseconf,
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
		if sessvktoken == "" {
			sessvkstate := generateRandomString(32)
			sessionManager.Put(r.Context(), "User.VK.State", sessvkstate)
			sessvkurl = VKGetUrl(sessvkstate)
		} else {
			sessvk = VKGetUInfo(sessvktoken)
		}
	}
	usermap := map[string]interface{}{
		"Username":   sessuname,
		"Id":         sessid,
		"Fname":      sessfname,
		"Lname":      sesslname,
		"Email":      sessemail,
		"Econf":      sesseconf,
		"WzProfile":  getWzProfile(sesswzprofile, "old_players3"),
		"WzProfile2": getWzProfile(sesswzprofile2, "players"),
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
	mergo.Merge(a, map[string]interface{}{
		"UserAuthorized": "True",
		"User":           usermap,
	})
	return a
}
func indexHandler(w http.ResponseWriter, r *http.Request) {
	load, _ := load.Avg()
	virtmem, _ := mem.VirtualMemory()
	uptime, _ := host.Uptime()
	uptimetime, _ := time.ParseDuration(strconv.Itoa(int(uptime)) + "s")
	basicLayoutLookupRespond("index", w, r, map[string]interface{}{"LoadAvg": load, "VirtMem": virtmem, "Uptime": uptimetime})
}
func autohosterControllHandler(w http.ResponseWriter, r *http.Request) {
	basicLayoutLookupRespond("autohoster-controll", w, r, map[string]interface{}{})
}
func robotsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "User-agent: *\nDisallow: /\n\n\n")
}
func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/favicon.ico")
}

type Ra struct {
	Dummy      bool   `json:"dummy"`
	Autohoster bool   `json:"autohoster"`
	Star       [3]int `json:"star"`
	Medal      int    `json:"medal"`
	Level      int    `json:"level"`
	Elo        string `json:"elo"`
	Details    string `json:"details"`
}

func ratingHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	hash, ok := params["hash"]
	if !ok {
		hash = r.Header.Get("WZ-Player-Hash")
	}
	w.Header().Set("Content-Type", "application/json")
	if hash == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{\"error\": \"Empty hash.\"}"))
		return
	}
	m := ratingLookup(hash)
	ad, adok := r.URL.Query()["ad"]
	if adok && len(ad[0]) >= 1 && string(ad[0][0]) == "true" {
		m.Elo = "Play with me in Autohoster"
	}
	j, err := json.Marshal(m)
	if err != nil {
		log.Println(err.Error())
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, string(j))
}
func ratingLookup(hash string) Ra {
	isautohoster := false
	if hash == "a0c124533ddcaf5a19cc7d593c33d750680dc428b0021672e0b86a9b0dcfd711" {
		isautohoster = true
	}
	elo := ""
	if isautohoster {
		elo = "Visit wz2100-autohost.net"
	}
	if hash == "7bade06ad15023640093ced192db5082641b625f74a72193142453a9ad742d93" {
		elo = "Dirty manque cheater"
	}
	m := Ra{true, isautohoster, [3]int{0, 0, 0}, 0, -1, elo, ""}
	var de, de2, dap, daw, dal, dui, dep, drp, dpi int
	var dname string
	derr := dbpool.QueryRow(context.Background(), `select elo, elo2, autoplayed, autowon, autolost,
		coalesce((SELECT id FROM users WHERE result.id = users.wzprofile2), -1),
		elo_position, rating_position, id, name
from (
   select *,
        row_number() over(
           order by elo2 desc
        ) as rating_position,
        row_number() over(
           order by elo desc
        ) as elo_position
   from players
) result
where hash = $1`, hash).Scan(&de, &de2, &dap, &daw, &dal, &dui, &dep, &drp, &dpi, &dname)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			if elo == "" {
				m.Elo = "Unknown player"
			}
		} else {
			log.Print(derr)
		}
	} else {
		m.Details = fmt.Sprintf("Played: %d\n", dap)
		m.Details += fmt.Sprintf("Won: %d Lost: %d\n", daw, dal)
		if dui != -1 && dui != 0 {
			m.Details += fmt.Sprintf("Rating: %d (#%d)\n", de2, drp)
		} else {
			m.Details += "Not registered user.\n"
		}
		m.Details += fmt.Sprintf("Elo: %d (#%d)\n", de, dep)
		m.Details += "\nRating lookup from\nhttps://wz2100-autohost.net/"
		if elo == "" {
			var pc string
			if dap > 0 {
				pc = fmt.Sprintf("%.1f%%", 100*(float64(daw)/float64(dap)))
			} else {
				pc = "-"
			}
			if dui != -1 && dui != 0 {
				m.Elo = fmt.Sprintf("R[%d] E[%d] %d %s", de2, de, dap, pc)
			} else {
				m.Elo = fmt.Sprintf("unapproved E[%d] %d %s", de, dap, pc)
			}
		}
		if dap < 5 {
			m.Dummy = true
		} else {
			m.Dummy = false
			if dal == 0 {
				dal = 1
			}
			if daw >= 24 && daw/dal > 12 {
				m.Medal = 1
			} else if daw >= 12 && daw/dal > 6 {
				m.Medal = 2
			} else if daw >= 6 && daw/dal > 3 {
				m.Medal = 3
			}
			if de > 1800 {
				m.Star[0] = 1
			} else if de > 1550 {
				m.Star[0] = 2
			} else if de > 1400 {
				m.Star[0] = 3
			}
			if dap > 60 {
				m.Star[1] = 1
			} else if dap > 30 {
				m.Star[1] = 2
			} else if dap > 10 {
				m.Star[1] = 3
			}
			if daw > 60 {
				m.Star[2] = 1
			} else if daw > 30 {
				m.Star[2] = 2
			} else if daw > 10 {
				m.Star[2] = 3
			}
		}
	}
	return m
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

func customLogger(writer io.Writer, params handlers.LogFormatterParams) {
	r := params.Request
	ip := r.Header.Get("CF-Connecting-IP")
	geo := r.Header.Get("CF-IPCountry")
	ua := r.Header.Get("user-agent")
	log.Println("["+geo+" "+ip+"]", r.Method, params.StatusCode, r.RequestURI, "["+ua+"]")
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
				if event.Op&fsnotify.Write == fsnotify.Write {
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
	sessionManager.Lifetime = 14 * 24 * time.Hour
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
	router.PathPrefix("/static").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	router.HandleFunc("/favicon.ico", faviconHandler)
	router.HandleFunc("/robots.txt", robotsHandler)
	router.HandleFunc("/", indexHandler)

	router.HandleFunc("/legal", basicLayoutHandler("legal"))
	router.HandleFunc("/about", basicLayoutHandler("about"))
	router.HandleFunc("/login", loginHandler)
	router.HandleFunc("/logout", logoutHandler)
	router.HandleFunc("/register", registerHandler)
	router.HandleFunc("/account", basicLayoutHandler("account"))
	router.HandleFunc("/users", usersHandler)
	router.HandleFunc("/activate", emailconfHandler)
	router.HandleFunc("/recover", recoverPasswordHandler)
	router.HandleFunc("/oauth/discord", DiscordCallbackHandler)
	router.HandleFunc("/oauth/vk", VKCallbackHandler)

	router.HandleFunc("/hoster", hosterHandler)
	router.HandleFunc("/request", hostRequestHandler)
	router.HandleFunc("/wzlink", wzlinkHandler)
	router.HandleFunc("/wzlinkcheck", wzlinkCheckHandler)
	router.HandleFunc("/autohoster", basicLayoutHandler("autohoster-control"))
	router.HandleFunc("/preset-edit", presetEditorHandler)

	router.HandleFunc("/rating/{hash:[0-9a-z]+}", ratingHandler)
	router.HandleFunc("/rating", ratingHandler)
	router.HandleFunc("/lobby", lobbyHandler)
	router.HandleFunc("/games", basicLayoutHandler("games2"))
	router.HandleFunc("/games/{id:[0-9]+}", DbGameDetailsHandler)
	router.HandleFunc("/players", basicLayoutHandler("players"))
	router.HandleFunc("/players/{id:[0-9]+}", PlayersHandler)
	router.HandleFunc("/stats", basicLayoutHandler("stats"))
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
	router.HandleFunc("/api/graph/{gid:[0-9]+}", APIgetGraphData).Methods("GET")
	router.HandleFunc("/api/classify/game/{gid:[0-9]+}", APIgetClassChartGame).Methods("GET")
	router.HandleFunc("/api/classify/player/{pid:[0-9]+}/{category:[0-9]+}", APIgetClassChartPlayer).Methods("GET")
	router.HandleFunc("/api/reslog/{gid:[0-9]+}", APIgetResearchlogData).Methods("GET")
	router.HandleFunc("/api/gamecount/{interval}", APIgetDatesGraphData).Methods("GET")
	router.HandleFunc("/api/multihoster/alive", APItryReachMultihoster).Methods("GET")
	router.HandleFunc("/api/dayavg", APIgetDayAverageByHour).Methods("GET")
	router.HandleFunc("/api/playersavg", APIgetUniquePlayersPerDay).Methods("GET")
	router.HandleFunc("/api/mapcount", APIgetMapNameCount).Methods("GET")
	router.HandleFunc("/api/replay/{gid:[0-9]+}", APIgetReplayFile).Methods("GET")
	router.HandleFunc("/api/allowjoining/{hash:[0-9a-z]+}", APIgetPlayerAllowedJoining).Methods("GET")
	router.HandleFunc("/api/approvedhashes", APIgetAllowedModerators).Methods("GET")
	router.HandleFunc("/api/elohistory/{pid:[0-9]+}", APIgetElodiffChartPlayer).Methods("GET")
	router.HandleFunc("/api/players", APIgetLeaderboard).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/games", APIgetGames).Methods("GET", "OPTIONS")

	router.HandleFunc("/elo/calc", EloRecalcHandler)

	router0 := sessionManager.LoadAndSave(router)
	router1 := handlers.ProxyHeaders(router0)
	//	router2 := handlers.CompressHandler(router1)
	router3 := handlers.CustomLoggingHandler(os.Stdout, router1, customLogger)
	// router4 := handlers.RecoveryHandler()(router3)
	log.Println("Started!")
	log.Panic(http.ListenAndServe(":"+port, router3))
}
