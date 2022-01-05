package main

import (
	"context"
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
	Team          int
	Colour        int
	Usertype      string
	Position      int
	Score         int
	Droid         int
	DroidLoss     int
	DroidLost     int
	DroidBuilt    int
	Kills         int
	Power         int
	Struct        int
	StructBuilt   int
	StructLost    int
	ResearchCount int
	EloDiff       int
	Autoplayed    int `json:"autoplayed"`
	Autolost      int `json:"autolost"`
	Autowon       int `json:"autowon"`
	Elo           int `json:"elo"`
	Elo2          int `json:"elo2"`
	Userid        int `json:"userid"`
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
	Researchlog    string
	Gamedir        string
	Hidden         bool
	Calculated     bool
	DebugTriggered bool
}

func DbGameDetailsHandler(w http.ResponseWriter, r *http.Request) {
	var g DbGamePreview
	params := mux.Vars(r)
	gid := params["id"]
	gidn, _ := strconv.Atoi(gid)
	rows, derr := dbpool.Query(context.Background(), `
	SELECT
		games.id as gid, finished, to_char(timestarted, 'YYYY-MM-DD HH24:MI'), coalesce(to_char(timestarted, 'YYYY-MM-DD HH24:MI'), '==='), gametime,
		players, teams, colour, usertype,
		mapname, maphash,
		baselevel, powerlevel, scavs, alliancetype,
		array_agg(to_json(p)::jsonb || json_build_object('userid', coalesce((SELECT id AS userid FROM users WHERE p.id = users.wzprofile2), -1))::jsonb)::text[] as pnames,
		score, kills, power, units, unitslost, unitbuilt, structs, structbuilt, structurelost, rescount, coalesce(researchlog, '{}'), coalesce(elodiff, '{0,0,0,0,0,0,0,0,0,0,0}'),
		coalesce(gamedir), calculated, hidden, debugtriggered
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
		err := rows.Scan(&g.ID, &g.Finished, &g.TimeStarted, &g.TimeEnded, &g.GameTime,
			&plid, &plteam, &plcolour, &plusertype,
			&g.MapName, &g.MapHash, &g.BaseLevel, &g.PowerLevel, &g.Scavengers, &g.Alliances, &plsj,
			&dsscore, &dskills, &dspower, &dsdroid, &dsdroidlost, &dsdroidbuilt, &dsstruct, &dsstructbuilt, &dsstructlost, &dsrescount, &g.Researchlog, &dselodiff,
			&g.Gamedir, &g.Calculated, &g.Hidden, &g.DebugTriggered)
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
			} else {
				g.Players[slot].Usertype = "fighter"
			}
		}
	}
	// return g, nil
	if count == 0 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Game not found"})
	} else {
		basicLayoutLookupRespond("gamedetails2", w, r, map[string]interface{}{"Game": g})
	}
}

func listDbGamesHandler(w http.ResponseWriter, r *http.Request) {
	var gamesTotal int
	derr := dbpool.QueryRow(context.Background(), `SELECT count(*) FROM games WHERE hidden = false AND deleted = false;`).Scan(&gamesTotal)
	if derr != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		return
	}
	wherecase := "WHERE deleted = false AND hidden = false"
	if sessionGetUsername(r) == "Flex seal" {
		wherecase = ""
	}
	limiter := "LIMIT 100"
	limiterparam, limiterparamok := r.URL.Query()["all"]
	if limiterparamok && len(limiterparam) >= 1 && limiterparam[0] == "true" {
		limiter = ""
	}
	rows, derr := dbpool.Query(context.Background(), `
	SELECT
		games.id as gid, finished, to_char(timestarted, 'YYYY-MM-DD HH24:MI'), coalesce(to_char(timestarted, 'YYYY-MM-DD HH24:MI'), '==='), gametime,
		players, teams, colour, usertype,
		mapname, maphash,
		baselevel, powerlevel, scavs, alliancetype,
		array_agg(to_json(p)::jsonb || json_build_object('userid', coalesce((SELECT id AS userid FROM users WHERE p.id = users.wzprofile2), -1))::jsonb)::text[] as pnames, kills, coalesce(elodiff, '{0,0,0,0,0,0,0,0,0,0,0}'),
		hidden, calculated, debugtriggered
	FROM games
	JOIN players as p ON p.id = any(games.players)
	`+wherecase+`
	GROUP BY gid
	ORDER BY timestarted DESC
	`+limiter+`;`)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "No games played"})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	defer rows.Close()
	var gms []DbGamePreview
	for rows.Next() {
		var g DbGamePreview
		var plid []int
		var plteam []int
		var plcolour []int
		var plusertype []string
		var plsj []string
		var dskills []int
		var dselodiff []int
		err := rows.Scan(&g.ID, &g.Finished, &g.TimeStarted, &g.TimeEnded, &g.GameTime,
			&plid, &plteam, &plcolour, &plusertype,
			&g.MapName, &g.MapHash, &g.BaseLevel, &g.PowerLevel, &g.Scavengers, &g.Alliances, &plsj,
			&dskills, &dselodiff, &g.Hidden, &g.Calculated, &g.DebugTriggered)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database scan error: " + err.Error()})
			return
		}
		var np [11]DbGamePlayerPreview
		for pi, pv := range plsj {
			err := json.Unmarshal([]byte(pv), &np[pi])
			if err != nil {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json unpack error: " + err.Error()})
				return
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
				if (plusertype[slot] == "winner" || plusertype[slot] == "loser") && len(dselodiff) < slot && len(g.Players) < slot {
					g.Players[slot].EloDiff = dselodiff[slot]
				}
			} else {
				g.Players[slot].Usertype = "fighter"
				g.Players[slot].Kills = 0
			}
		}
		// spew.Dump(g)
		gms = append(gms, g)
	}
	basicLayoutLookupRespond("games2", w, r, map[string]interface{}{"Games": gms, "GamesCount": gamesTotal})
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
