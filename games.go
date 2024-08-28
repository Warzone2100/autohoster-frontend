package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"slices"
	_ "sort"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type PlayerRating struct {
	Elo        int
	Played     int
	Won        int
	Lost       int
	TimePlayed int
	Account    int
	Category   int
}

type Player struct {
	Position       int
	Name           string
	Team           int
	Color          int
	Identity       int
	IdentityPubKey string
	Usertype       string
	Rating         *PlayerRating
	Account        int
	DisplayName    string
	Props          map[string]any
}

type Game struct {
	ID              int
	Version         string
	Instance        int
	TimeStarted     time.Time
	TimeEnded       *time.Time
	GameTime        *int
	SettingScavs    int
	SettingAlliance int
	SettingPower    int
	SettingBase     int
	MapName         string
	MapHash         string
	Mods            string
	Deleted         bool
	Hidden          bool
	Calculated      bool
	DebugTriggered  bool
	Players         []Player
	ReplayFound     bool
	DisplayCategory int
}

func DbGameDetailsHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Invalid id: " + err.Error()})
		return
	}
	req := `select
	g.id, g.version, g.instance, g.time_started, g.time_ended, g.game_time,
	g.setting_scavs, g.setting_alliance, g.setting_power, g.setting_base,
	g.map_name, g.map_hash, g.mods, g.deleted, g.hidden, g.calculated, g.debug_triggered,
	g.display_category,
	jsonb_pretty(json_agg(json_build_object(
		'Position', p.position,
		'Name', i.name,
		'Team', p.team,
		'Usertype', p.usertype,
		'Color', p.color,
		'Identity', i.id,
		'IdentityPubKey', encode(i.pkey, 'base64'),
		'Account', a.id,
		'DisplayName', coalesce(i.name, a.display_name),
		'Rating', (select r from rating as r where r.category = g.display_category and r.account = i.account),
		'Props', p.props
	))::jsonb) as players
from games as g
join players as p on p.game = g.id
join identities as i on i.id = p.identity
left join accounts as a on a.id = i.account
where g.id = $1
group by g.id`
	g := Game{}
	g.Players = []Player{}
	playersJSON := ""
	err = dbpool.QueryRow(r.Context(), req, id).Scan(&g.ID, &g.Version, &g.Instance, &g.TimeStarted, &g.TimeEnded, &g.GameTime,
		&g.SettingScavs, &g.SettingAlliance, &g.SettingPower, &g.SettingBase,
		&g.MapName, &g.MapHash, &g.Mods, &g.Deleted, &g.Hidden, &g.Calculated, &g.DebugTriggered, &g.DisplayCategory,
		&playersJSON)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + err.Error()})
		return
	}
	err = json.Unmarshal([]byte(playersJSON), &g.Players)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Json unmarshal error: " + err.Error()})
		return
	}
	g.ReplayFound = checkReplayExistsInStorage(id)
	// slices.SortFunc(gmsStage[0].Players, func(a Player, b Player) int {
	// 	return a.Position - b.Position
	// })
	basicLayoutLookupRespond("gamedetails2", w, r, map[string]any{"Game": g})
}

func DbGamesHandler(w http.ResponseWriter, r *http.Request) {
	dmapsc := make(chan []string)
	var dmaps []string
	dmapspresent := false
	dtotalc := make(chan int)
	var dtotal int
	dtotalpresent := false
	errc := make(chan error)
	go func() {
		var mapnames []string
		derr := dbpool.QueryRow(r.Context(), `select array_agg(distinct map_name) from games where hidden = false and deleted = false;`).Scan(&mapnames)
		if derr != nil && derr != pgx.ErrNoRows {
			errc <- derr
			return
		}
		dmapsc <- mapnames
	}()
	go func() {
		var c int
		derr := dbpool.QueryRow(r.Context(), `select count(games) from games where hidden = false and deleted = false;`).Scan(&c)
		if derr != nil && derr != pgx.ErrNoRows {
			errc <- derr
			return
		}
		dtotalc <- c
	}()
	for !(dmapspresent && dtotalpresent) {
		select {
		case derr := <-errc:
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + derr.Error()})
			return
		case dmaps = <-dmapsc:
			dmapspresent = true
		case dtotal = <-dtotalc:
			dtotalpresent = true
		}
	}
	basicLayoutLookupRespond("games2", w, r, map[string]any{"Total": dtotal, "Maps": dmaps})
}

func APIgetGames(_ http.ResponseWriter, r *http.Request) (int, any) {
	reqLimit := parseQueryInt(r, "limit", 50)
	if reqLimit > 200 {
		reqLimit = 200
	}
	if reqLimit <= 0 {
		reqLimit = 1
	}
	reqOffset := parseQueryInt(r, "offset", 0)
	if reqOffset < 0 {
		reqOffset = 0
	}
	reqSortOrder := parseQueryStringFiltered(r, "order", "desc", "asc")
	fieldmappings := map[string]string{
		"TimeStarted": "time_started",
		"TimeEnded":   "time_ended",
		"ID":          "id",
		"MapName":     "map_name",
		"GameTime":    "game_time",
	}
	reqSortField := parseQueryStringMapped(r, "sort", "time_started", fieldmappings)

	reqFilterJ := parseQueryString(r, "filter", "")
	reqFilterFields := map[string]string{}
	reqDoFilters := false
	if reqFilterJ != "" {
		err := json.Unmarshal([]byte(reqFilterJ), &reqFilterFields)
		if err == nil && len(reqFilterFields) > 0 {
			reqDoFilters = true
		}
	}

	wherecase := "WHERE deleted = false AND hidden = false"
	if sessionGetUsername(r) == "Flex seal" {
		wherecase = ""
	}
	pid := parseQueryInt(r, "player", -1)
	if pid > 0 {
		if wherecase == "" {
			wherecase = fmt.Sprintf("WHERE %d = p.id", pid)
		} else {
			wherecase += fmt.Sprintf(" AND %d = p.id", pid)
		}
	}
	whereargs := []any{}
	if reqDoFilters {
		val, ok := reqFilterFields["MapName"]
		if ok {
			whereargs = append(whereargs, val)
			if wherecase == "" {
				wherecase = "WHERE g.map_name = $1"
			} else {
				wherecase += " AND g.map_name = $1"
			}
		}
	}

	reqSearch := parseQueryString(r, "search", "")

	ordercase := fmt.Sprintf("ORDER BY %s %s", reqSortField, reqSortOrder)
	orderargs := []any{}

	if reqSearch != "" {
		orderargs = append(orderargs, reqSearch)
		argnum := len(whereargs) + 1
		ordercase = fmt.Sprintf("ORDER BY max(similarity(coalesce(i.name, a.display_name), $%d::text)) desc, %s %s", argnum, reqSortField, reqSortOrder)
	}

	limiter := fmt.Sprintf("LIMIT %d", reqLimit)
	offset := fmt.Sprintf("OFFSET %d", reqOffset)

	totalsc := make(chan int)
	var totals int
	totalspresent := false

	totalsNoFilterc := make(chan int)
	var totalsNoFilter int
	totalsNoFilterpresent := false

	growsc := make(chan []Game)
	var gms []Game
	gpresent := false

	echan := make(chan error)
	go func() {
		var c int
		derr := dbpool.QueryRow(r.Context(), `select count(games) from games where hidden = false and deleted = false;`).Scan(&c)
		if derr != nil {
			log.Println(derr)
			echan <- derr
			return
		}
		totalsNoFilterc <- c
	}()
	go func() {
		var c int
		req := `select count(g.id) from games as g ` + wherecase + `;`
		log.Printf("req %s args %#+v", req, whereargs)
		derr := dbpool.QueryRow(r.Context(), req, whereargs...).Scan(&c)
		if derr != nil {
			log.Println(derr)
			echan <- derr
			return
		}
		totalsc <- c
	}()

	go func() {
		req := `select
	g.id, g.version, g.time_started, g.time_ended, g.game_time,
	g.setting_scavs, g.setting_alliance, g.setting_power, g.setting_base,
	g.map_name, g.map_hash, g.mods, g.deleted, g.hidden, g.calculated, g.debug_triggered,
	g.display_category,
	jsonb_pretty(json_agg(json_build_object(
		'Position', p.position,
		'Name', i.name,
		'Team', p.team,
		'Usertype', p.usertype,
		'Color', p.color,
		'Identity', i.id,
		'IdentityPubKey', encode(i.pkey, 'base64'),
		'Account', a.id,
		'DisplayName', coalesce(i.name, a.display_name),
		'Rating', (select r from rating as r where r.category = g.display_category and r.account = i.account)
	))::jsonb) as players
from games as g
join players as p on p.game = g.id
join identities as i on i.id = p.identity
left join accounts as a on a.id = i.account
` + wherecase + `
group by g.id
` + ordercase + `
` + limiter + `
` + offset
		args := append(whereargs, orderargs...)
		log.Printf("req %s args %#+v", req, args)
		gmsStage := []Game{}
		rows, err := dbpool.Query(r.Context(), req, args...)
		if err != nil {
			log.Println(err)
			echan <- err
			return
		}
		for rows.Next() {
			g := Game{}
			playersJSON := ""
			err = rows.Scan(&g.ID, &g.Version, &g.TimeStarted, &g.TimeEnded, &g.GameTime,
				&g.SettingScavs, &g.SettingAlliance, &g.SettingPower, &g.SettingBase,
				&g.MapName, &g.MapHash, &g.Mods, &g.Deleted, &g.Hidden, &g.Calculated, &g.DebugTriggered, &g.DisplayCategory,
				&playersJSON)
			if err != nil {
				echan <- err
				return
			}
			g.Players = []Player{}
			err = json.Unmarshal([]byte(playersJSON), &g.Players)
			if err != nil {
				echan <- err
				return
			}
			slices.SortFunc(g.Players, func(a Player, b Player) int {
				return a.Position - b.Position
			})
			gmsStage = append(gmsStage, g)
		}
		if err != nil {
			echan <- err
			return
		}
		growsc <- gmsStage
	}()
	for !(gpresent && totalspresent && totalsNoFilterpresent) {
		select {
		case derr := <-echan:
			if derr == pgx.ErrNoRows {
				return 200, []byte(`{"total": 0, "totalNotFiltered": 0, "rows": []}`)
			}
			return 500, derr
		case gms = <-growsc:
			gpresent = true
		case totals = <-totalsc:
			totalspresent = true
		case totalsNoFilter = <-totalsNoFilterc:
			totalsNoFilterpresent = true
		}
	}
	return 200, map[string]any{
		"total":            totals,
		"totalNotFiltered": totalsNoFilter,
		"rows":             gms,
	}
}

func GameTimeToString(t any) string {
	switch v := t.(type) {
	case int:
		return (time.Duration(int(v/1000)) * time.Second).String()
	case *int:
		if v == nil {
			return "nil gametime"
		}
		return (time.Duration(int(*v/1000)) * time.Second).String()
	default:
		return "not float64 gametime"
	}
}
func GameTimeToStringI(t any) string {
	switch v := t.(type) {
	case int:
		return (time.Duration(v/1000) * time.Second).String()
	case *int:
		if v == nil {
			return "nil gametime"
		}
		return (time.Duration(*v/1000) * time.Second).String()
	default:
		return "not int gametime"
	}
}

//lint:ignore U1000 for later
func GameTimeInterToString(t any) string {
	tt, k := t.(float64)
	if k {
		return (time.Duration(int(tt/1000)) * time.Second).String()
	} else {
		return "invalid"
	}
}

//lint:ignore U1000 for later
func SecondsToString(t float64) string {
	return (time.Duration(int(t)) * time.Second).String()
}

//lint:ignore U1000 for later
func SecondsInterToString(t any) string {
	tt, k := t.(float64)
	if k {
		return (time.Duration(int(tt)) * time.Second).String()
	} else {
		return "invalid"
	}
}

var GameDirRegex = regexp.MustCompile(`\./tmp/wz-(\d+)/`)

func GameDirToWeek(p string) int {
	matches := GameDirRegex.FindStringSubmatch(p)
	if len(matches) != 2 {
		log.Println("No match for game directory")
		return -1
	}
	num, err := strconv.Atoi(matches[1])
	if err != nil {
		log.Printf("Error atoi: %#+v %#+v", matches, err)
		return -1
	}
	return num / (7 * 24 * 60 * 60)
}

func InstanceIDToWeek(num int) int {
	return num / (7 * 24 * 60 * 60)
}
