package main

import (
	"log"
	"net/http"
	"regexp"
	"slices"
	_ "sort"
	"strconv"
	"time"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type Player struct {
	Position int
	Name     string
	Team     int
	Color    int
	Identity int
	Usertype string
	Account  int
	Elo      int
	Played   int
	Won      int
	Lost     int
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
	map_name, g.map_hash, g.mods, g.deleted, g.hidden, g.calculated, g.debug_triggered,
	json_agg(json_build_object(
		'Position', players.position,
		'Name', identities.name,
		'Team', players.team,
		'Usertype', players.usertype,
		'Color', players.color,
		'Account', coalesce(accounts.id, 0),
		'Elo', coalesce(rating.elo, 0),
		'Played', coalesce(rating.played, 0),
		'Won', coalesce(rating.won, 0),
		'Lost', coalesce(rating.lost, 0)
	)) as players
from games as g
join players on game = g.id
join identities on identity = identities.id
left join accounts on identities.account = accounts.id
full outer join rating on identities.account = rating.account
where rating.category = $1 and g.id = $2
group by g.id
order by time_started desc`
	var gmsStage []*Game
	ratingCategory := 2
	err = pgxscan.Select(r.Context(), dbpool, &gmsStage, req, ratingCategory, id)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database error: " + err.Error()})
		return
	}
	if len(gmsStage) == 0 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msg": "Game not found"})
		return
	}
	gmsStage[0].ReplayFound = checkReplayExistsInStorage(id)
	slices.SortFunc(gmsStage[0].Players, func(a Player, b Player) int {
		return a.Position - b.Position
	})
	basicLayoutLookupRespond("gamedetails2", w, r, map[string]any{"Game": gmsStage[0]})
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
