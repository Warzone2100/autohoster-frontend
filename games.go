package main

import (
	"encoding/json"
	"net/http"
	_ "sort"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type DbGamePlayerPreview struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Hash          string `json:"hash"`
	Team          int    `json:",omitempty"`
	Colour        int    `json:",omitempty"`
	Usertype      string `json:",omitempty"`
	Position      int    `json:",omitempty"`
	Score         int    `json:",omitempty"`
	Droid         int    `json:",omitempty"`
	DroidLoss     int    `json:",omitempty"`
	DroidLost     int    `json:",omitempty"`
	DroidBuilt    int    `json:",omitempty"`
	Kills         int    `json:",omitempty"`
	Power         int    `json:",omitempty"`
	Struct        int    `json:",omitempty"`
	StructBuilt   int    `json:",omitempty"`
	StructLost    int    `json:",omitempty"`
	ResearchCount int    `json:",omitempty"`
	EloDiff       int    `json:",omitempty"`
	RatingDiff    int    `json:",omitempty"`
	Autoplayed    int    `json:"autoplayed"`
	Autolost      int    `json:"autolost"`
	Autowon       int    `json:"autowon"`
	Elo           int    `json:"elo"`
	Elo2          int    `json:"elo2"`
	Userid        int    `json:"userid"`
}
type DbGamePreview struct {
	ID             int
	Finished       bool
	TimeStarted    string
	TimeEnded      string
	GameTime       int
	MapName        string
	MapHash        string
	Players        [11]DbGamePlayerPreview
	BaseLevel      int
	PowerLevel     int
	Scavengers     bool
	Alliances      int
	Researchlog    string `json:",omitempty"`
	Gamedir        string `json:",omitempty"`
	Hidden         bool
	Calculated     bool
	DebugTriggered bool
	ReplayFound    bool
	GameVersion    string `json:",omitempty"`
	Mod            string `json:",omitempty"`
}

func DbGameDetailsHandler(w http.ResponseWriter, r *http.Request) {
	var g DbGamePreview
	params := mux.Vars(r)
	gid := params["id"]
	gidn, _ := strconv.Atoi(gid)
	rows, derr := dbpool.Query(r.Context(), `
	SELECT
		games.id as gid, finished, to_char(timestarted, 'YYYY-MM-DD HH24:MI'), coalesce(to_char(timeended, 'YYYY-MM-DD HH24:MI'), 'in-game'), gametime,
		players, teams, colour, usertype,
		mapname, maphash,
		baselevel, powerlevel, scavs, alliancetype,
		array_agg(to_json(p)::jsonb || json_build_object('userid', coalesce((SELECT id AS userid FROM users WHERE p.id = users.wzprofile2), -1))::jsonb)::text[] as pnames,
		score, kills, power, units, unitslost, unitbuilt, structs, structbuilt, structurelost, rescount, coalesce(researchlog, '{}'),
		coalesce(elodiff, '{0,0,0,0,0,0,0,0,0,0,0}'), coalesce(ratingdiff, '{0,0,0,0,0,0,0,0,0,0,0}'),
		coalesce(gamedir), calculated, hidden, debugtriggered, coalesce(version, '???'), mod
	FROM games
	JOIN players as p ON p.id = any(games.players)
	WHERE deleted = false AND hidden = false AND games.id = $1
	GROUP BY gid`, gidn)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Game not found"})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database scan error: " + derr.Error()})
		}
		return
		// return g, derr
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
		var plid []int
		var plteam []int
		var plcolour []int
		var plusertype []string
		var plsj []string
		var dsscore []int
		var dskills []int
		var dspower []int
		var dsdroid []int
		var dsdroidlost []int
		var dsdroidbuilt []int
		var dsstruct []int
		var dsstructbuilt []int
		var dsstructlost []int
		var dsrescount []int
		var dselodiff []int
		var dsratingdiff []int
		err := rows.Scan(&g.ID, &g.Finished, &g.TimeStarted, &g.TimeEnded, &g.GameTime,
			&plid, &plteam, &plcolour, &plusertype,
			&g.MapName, &g.MapHash, &g.BaseLevel, &g.PowerLevel, &g.Scavengers, &g.Alliances, &plsj,
			&dsscore, &dskills, &dspower, &dsdroid, &dsdroidlost, &dsdroidbuilt, &dsstruct, &dsstructbuilt, &dsstructlost, &dsrescount, &g.Researchlog,
			&dselodiff, &dsratingdiff,
			&g.Gamedir, &g.Calculated, &g.Hidden, &g.DebugTriggered, &g.GameVersion, &g.Mod)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database scan error: " + err.Error()})
			return
			// return g, derr
		}
		var np [11]DbGamePlayerPreview
		for pi, pv := range plsj {
			err := json.Unmarshal([]byte(pv), &np[pi])
			if err != nil {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json error: " + err.Error()})
				return
				// return g, err
			}
		}
		for slot, nid := range plid {
			gpi := -1
			for pi, pv := range np {
				if pv.ID == nid {
					gpi = pi
					break
				}
			}
			if gpi == -1 {
				// log.Print("Failed to find player " + strconv.Itoa(slot) + " for game " + strconv.Itoa(g.Id))
				continue
			}
			g.Players[slot] = np[gpi]
			g.Players[slot].Team = plteam[slot]
			g.Players[slot].Colour = plcolour[slot]
			g.Players[slot].Position = slot
			if g.Finished {
				g.Players[slot].Usertype = plusertype[slot]
				g.Players[slot].Kills = dskills[slot]
				g.Players[slot].Score = dsscore[slot]
				g.Players[slot].Droid = dsdroid[slot]
				g.Players[slot].DroidLost = dsdroidlost[slot]
				g.Players[slot].DroidBuilt = dsdroidbuilt[slot]
				g.Players[slot].Kills = dskills[slot]
				g.Players[slot].Power = dspower[slot]
				g.Players[slot].Struct = dsstruct[slot]
				g.Players[slot].StructBuilt = dsstructbuilt[slot]
				g.Players[slot].StructLost = dsstructlost[slot]
				g.Players[slot].ResearchCount = dsrescount[slot]
				if len(dselodiff) > slot {
					g.Players[slot].EloDiff = dselodiff[slot]
				} else {
					g.Players[slot].EloDiff = 0
				}
				if len(dsratingdiff) > slot {
					g.Players[slot].RatingDiff = dsratingdiff[slot]
				} else {
					g.Players[slot].RatingDiff = 0
				}
			} else {
				g.Players[slot].Usertype = "fighter"
			}
		}
	}
	g.ReplayFound = checkReplayExistsInStorage(gidn)
	if count == 0 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Game not found"})
	} else {
		basicLayoutLookupRespond("gamedetails2", w, r, map[string]interface{}{"Game": g})
	}
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
		derr := dbpool.QueryRow(r.Context(), `select array_agg(distinct mapname) from games where hidden = false and deleted = false;`).Scan(&mapnames)
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
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database error: " + derr.Error()})
			return
		case dmaps = <-dmapsc:
			dmapspresent = true
		case dtotal = <-dtotalc:
			dtotalpresent = true
		}
	}
	basicLayoutLookupRespond("games2", w, r, map[string]interface{}{"Total": dtotal, "Maps": dmaps})
}

func GameTimeToString(t float64) string {
	return (time.Duration(int(t/1000)) * time.Second).String()
}
func GameTimeToStringI(t int) string {
	return (time.Duration(t/1000) * time.Second).String()
}

//lint:ignore U1000 for later
func GameTimeInterToString(t interface{}) string {
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
func SecondsInterToString(t interface{}) string {
	tt, k := t.(float64)
	if k {
		return (time.Duration(int(tt)) * time.Second).String()
	} else {
		return "invalid"
	}
}
