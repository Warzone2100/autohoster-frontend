package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
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
}
type DbGamePreview struct {
	ID          int
	Finished    bool
	TimeStarted string
	TimeEnded   string
	GameTime    int
	MapName     string
	MapHash     string
	Players     [11]DbGamePlayerPreview
	BaseLevel   int
	PowerLevel  int
	Scavengers  bool
	Alliances   int
	Researchlog string
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
		array_agg(row_to_json(p))::text[] as pnames,
		score, kills, power, units, unitloss, unitslost, unitbuilt, structs, structbuilt, structurelost, rescount, coalesce(researchlog, '{}'), coalesce(elodiff, '{0,0,0,0,0,0,0,0,0,0,0}')
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
	for rows.Next() {
		var plid []int
		var plteam []int
		var plcolour []int
		var plusertype []string
		var plsj []string
		var dsscore []int
		var dskills []int
		var dspower []int
		var dsdroid []int
		var dsdroidloss []int
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
			&dsscore, &dskills, &dspower, &dsdroid, &dsdroidloss, &dsdroidlost, &dsdroidbuilt, &dsstruct, &dsstructbuilt, &dsstructlost, &dsrescount, &g.Researchlog, &dselodiff)
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
				g.Players[slot].DroidLoss = dsdroidloss[slot]
				g.Players[slot].DroidLost = dsdroidlost[slot]
				g.Players[slot].DroidBuilt = dsdroidbuilt[slot]
				g.Players[slot].Kills = dskills[slot]
				g.Players[slot].Power = dspower[slot]
				g.Players[slot].Struct = dsstruct[slot]
				g.Players[slot].StructBuilt = dsstructbuilt[slot]
				g.Players[slot].StructLost = dsstructlost[slot]
				g.Players[slot].ResearchCount = dsrescount[slot]
				g.Players[slot].EloDiff = dselodiff[slot]
			} else {
				g.Players[slot].Usertype = "fighter"
			}
		}
	}
	// return g, nil
	basicLayoutLookupRespond("gamedetails2", w, r, map[string]interface{}{"Game": g})
}

func DDDbGameDetailsHandler(w http.ResponseWriter, r *http.Request) {
	// params := mux.Vars(r)
	// gid := params["id"]
	// gidn, err := strconv.Atoi(gid)
	// if err != nil {
	// 	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error: " + err.Error()})
	// 	return
	// }
	// g, err := GetGameFromDatabase(gidn)
	// if err != nil {
	// 	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error: " + err.Error()})
	// } else {
	// 	basicLayoutLookupRespond("gamedetails2", w, r, map[string]interface{}{"Game": g})
	// }
}

func listDbGamesHandler(w http.ResponseWriter, r *http.Request) {
	rows, derr := dbpool.Query(context.Background(), `
	SELECT
		games.id as gid, finished, to_char(timestarted, 'YYYY-MM-DD HH24:MI'), coalesce(to_char(timestarted, 'YYYY-MM-DD HH24:MI'), '==='), gametime,
		players, teams, colour, usertype,
		mapname, maphash,
		baselevel, powerlevel, scavs, alliancetype,
		array_agg(row_to_json(p))::text[] as pnames, kills, coalesce(elodiff, '{0,0,0,0,0,0,0,0,0,0,0}')
	FROM games
	JOIN players as p ON p.id = any(games.players)
	WHERE deleted = false AND hidden = false
	GROUP BY gid
	ORDER BY timestarted DESC
	LIMIT 100;`)
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
			&dskills, &dselodiff)
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
				g.Players[slot].EloDiff = dselodiff[slot]
			} else {
				g.Players[slot].Usertype = "fighter"
				g.Players[slot].Kills = 0
			}
		}
		// spew.Dump(g)
		gms = append(gms, g)
	}
	basicLayoutLookupRespond("games2", w, r, map[string]interface{}{"Games": gms})
}

func listGamesHandler(w http.ResponseWriter, r *http.Request) {
	rows, derr := dbpool.Query(context.Background(), `
	SELECT
		id,
		to_char(time_finished, 'YYYY-MM-DD HH24:MI'),
		game
	FROM jgames
	WHERE cast(game as text) != 'null' AND (game->>'gameTime')::int/1000 > 60
	ORDER BY time_finished DESC
	LIMIT 100;`) //
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "No games played"})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	defer rows.Close()
	type GamePrototype struct {
		Id       int
		Date     string
		Gametime string
		Map      map[string]interface{}
		Json     string
	}
	var games []GamePrototype
	for rows.Next() {
		var id int
		var date string
		var jsonf string
		err := rows.Scan(&id, &date, &jsonf)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database scan error: " + err.Error()})
			return
		}
		m := map[string]interface{}{}
		if err := json.Unmarshal([]byte(jsonf), &m); err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json parse error: " + err.Error()})
			return
		}
		gtstr := "?"
		if m != nil {
			gt, ex := m["gameTime"]
			if ex {
				gtstr = (time.Duration(int(gt.(float64)/1000)) * time.Second).String()
			}
		}
		n := GamePrototype{id, date, gtstr, m, jsonf}
		games = append(games, n)
	}
	basicLayoutLookupRespond("games", w, r, map[string]interface{}{
		"Games": games,
	})
}

type PlayerView struct {
	Name          string
	Hash          string
	Position      int
	Team          int
	PlayNum       int
	Colour        int
	Usertype      string
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
}
type ResearchView struct {
	Name    string
	Player  int
	Time    int
	TimeStr string
}
type GameView struct {
	MapName   string
	MapHash   string
	GameTime  float64
	Alliances float64
	Base      float64
	Power     float64
	Scav      bool
	Version   string
}

func GameTimeToString(t float64) string {
	return (time.Duration(int(t/1000)) * time.Second).String()
}
func GameTimeToStringI(t int) string {
	return (time.Duration(t/1000) * time.Second).String()
}

func GameTimeInterToString(t interface{}) string {
	tt, k := t.(float64)
	if k {
		return (time.Duration(int(tt/1000)) * time.Second).String()
	} else {
		return "invalid"
	}
}

func SecondsToString(t float64) string {
	return (time.Duration(int(t)) * time.Second).String()
}
func SecondsInterToString(t interface{}) string {
	tt, k := t.(float64)
	if k {
		return (time.Duration(int(tt)) * time.Second).String()
	} else {
		return "invalid"
	}
}

func gameViewHandler(w http.ResponseWriter, r *http.Request) {
	// if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
	// 	basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
	// 	return
	// }
	params := mux.Vars(r)
	gid := params["id"]
	var ddate string
	var djson string
	derr := dbpool.QueryRow(context.Background(), `
	SELECT
		to_char(time_finished, 'YYYY-MM-DD HH24:MI'),
		game
	FROM jgames
	WHERE id = $1
	ORDER BY time_finished DESC
	LIMIT 100;`, gid).Scan(&ddate, &djson)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Game not found"})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	if djson == "nil" {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json is nil"})
		return
	}
	m := map[string]interface{}{}
	if err := json.Unmarshal([]byte(djson), &m); err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json parse error: " + err.Error()})
		return
	}
	_, ok := m["JSONversion"].(float64)
	if !ok {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json version is incorrect"})
		return
	}
	gtstr := "?"
	if m != nil {
		gt, ex := m["gameTime"]
		if ex {
			gtstr = (time.Duration(int(gt.(float64)/1000)) * time.Second).String()
		}
	}
	_ = gtstr
	// WARNING SHITCODE AHEAD
	res := map[string][11]string{}
	// for _, bbb := range m["extendedPlayerData"].([]interface{}) {
	// rrr := m["researchComplite"].(map[string]interface{})
	for _, bb := range m["researchComplete"].([]interface{}) {
		rr := bb.(map[string]interface{})
		var b [11]string
		b = res[rr["name"].(string)]
		// bindex, _ := strconv.Atoi(rr["player"].(float64))
		// b[int(rr["player"].(float64))] = rr["time"].(float64)
		// app := ""
		// if rr["ideal"].(float64) > 0 {
		// app := "(" + SecondsInterToString(rr["ideal"]) + ")(" + strconv.Itoa(int(rr["ideal"].(float64)))+")"
		// }
		b[int(rr["player"].(float64))] = (time.Duration(int(rr["time"].(float64)/1000)) * time.Second).String() // + app
		res[rr["name"].(string)] = b
	}
	// }
	// (time.Duration(int(rr["time"].(float64)/1000)) * time.Second).String()
	reskeys := make([]string, len(res))
	keysi := 0
	for k := range res {
		reskeys[keysi] = k
		keysi++
	}
	sort.Strings(reskeys)
	resSorted := map[string][11]string{}
	for _, resval := range reskeys {
		// if res[resval][0] != "0s" {
		resSorted[resval] = res[resval]
		// }
	}
	// w.WriteHeader(http.StatusNotImplemented)
	// basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Not implemented."})
	// return
	basicLayoutLookupRespond("gamedetails", w, r, map[string]interface{}{
		"ID":   gid,
		"Date": ddate,
		"JMap": m,
		"Game": m["game"].(map[string]interface{}),
		// "Game":        MSItoGameViewV1(m["game"].(map[string]interface{})),
		"GameTimeStr": gtstr,
		"IsNull":      djson,
		"ResSorted":   resSorted,
	})
}

func DbGameViewHandler(w http.ResponseWriter, r *http.Request) {
	// if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
	// 	basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
	// 	return
	// }
	params := mux.Vars(r)
	gid := params["id"]
	var dtimestarted string
	var dtimeended string
	var dgametime int
	var dplayers []int
	var dplname []string
	var dplhash []string
	var dpltype []string
	var dteams []int
	var dcolour []int
	var delodiff []int
	var dmapname string
	var dmaphash string
	var dbase int
	var dpower int
	var dscav bool
	var dalliances int
	var dsscore []int
	var dskills []int
	var dspower []int
	var dsdroid []int
	var dsdroidloss []int
	var dsdroidlost []int
	var dsdroidbuilt []int
	var dsstruct []int
	var dsstructbuilt []int
	var dsstructlost []int
	var dsrescount []int
	var dselodiff []int
	derr := dbpool.QueryRow(context.Background(), `
	SELECT
		to_char(timestarted, 'YYYY-MM-DD HH24:MI') as ga, coalesce(to_char(timeended, 'YYYY-MM-DD HH24:MI'), 'Wha?') as gb, gametime as gc,
		mapname as gd, maphash as ge,
		players as gf, teams as gg, colour as gh, coalesce(elodiff, '{}') as gi, array_agg(players.name), array_agg(players.hash), coalesce(usertype, '{}') as gy,
		baselevel as gj, powerlevel as gk, alliancetype as gl, scavs as gm,
		score as gn, kills as go, power as gp, units as gq, unitloss as gr, unitslost as gs, unitbuilt as gt,
		structs as gu, structbuilt as gv, structurelost as gw, rescount as gx, elodiff as gz
	FROM games
	JOIN players on coalesce(players.id = any(players), 'No')
	WHERE games.id = $1
	GROUP BY ga, gb, gc, gd, ge, gf, gg, gh, gi, gj, gk, gl, gm, gn, go, gp, gq, gr, gs, gt, gu, gv, gw, gx, gy, gz
		`, gid).Scan(&dtimestarted, &dtimeended, &dgametime, &dmapname, &dmaphash,
		&dplayers, &dteams, &dcolour, &delodiff, &dplname, &dplhash, &dpltype,
		&dbase, &dpower, &dalliances, &dscav,
		&dsscore, &dskills, &dspower, &dsdroid, &dsdroidloss, &dsdroidlost, &dsdroidbuilt, &dsstruct, &dsstructbuilt, &dsstructlost, &dsrescount, &dselodiff)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Game not found"})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	gtstr := GameTimeToString(float64(dgametime))
	// WARNING SHITCODE AHEAD
	// res := map[string][11]string{}
	// // for _, bbb := range m["extendedPlayerData"].([]interface{}) {
	// // rrr := m["researchComplite"].(map[string]interface{})
	// for _, bb := range m["researchComplete"].([]interface{}) {
	// 	rr := bb.(map[string]interface{})
	// 	var b [11]string
	// 	b = res[rr["name"].(string)]
	// 	// bindex, _ := strconv.Atoi(rr["player"].(float64))
	// 	// b[int(rr["player"].(float64))] = rr["time"].(float64)
	// 	// app := ""
	// 	// if rr["ideal"].(float64) > 0 {
	// 	// app := "(" + SecondsInterToString(rr["ideal"]) + ")(" + strconv.Itoa(int(rr["ideal"].(float64)))+")"
	// 	// }
	// 	b[int(rr["player"].(float64))] = (time.Duration(int(rr["time"].(float64)/1000)) * time.Second).String() // + app
	// 	res[rr["name"].(string)] = b
	// }
	// // }
	// // (time.Duration(int(rr["time"].(float64)/1000)) * time.Second).String()
	// reskeys := make([]string, len(res))
	// keysi := 0
	// for k := range res {
	// 	reskeys[keysi] = k
	// 	keysi++
	// }
	// sort.Strings(reskeys)
	// resSorted := map[string][11]string{}
	// for _, resval := range reskeys {
	// 	// if res[resval][0] != "0s" {
	// 	resSorted[resval] = res[resval]
	// 	// }
	// }
	// w.WriteHeader(http.StatusNotImplemented)
	// basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Not implemented."})
	// return
	basicLayoutLookupRespond("gamedetails2", w, r, map[string]interface{}{
		"ID":             gid,
		"DateStarted":    dtimestarted,
		"DateEnded":      dtimeended,
		"GameTimeStr":    gtstr,
		"Players":        dplayers,
		"PName":          dplname,
		"PHash":          dplhash,
		"PType":          dpltype,
		"Teams":          dteams,
		"Colours":        dcolour,
		"MapName":        dmapname,
		"MapHash":        dmaphash,
		"LevelBase":      dbase,
		"LevelPower":     dpower,
		"LevelAlliances": dalliances,
		"LevelScav":      dscav,
		"StaScore":       dsscore,
		"StaKills":       dskills,
		"StaPower":       dspower,
		"StaDroid":       dsdroid,
		"StaDroidLoss":   dsdroidloss,
		"StaDroidLost":   dsdroidlost,
		"StaDroidBuilt":  dsdroidbuilt,
		"StaStruct":      dsstruct,
		"StaStructBuilt": dsstructbuilt,
		"StaStructLost":  dsstructlost,
		"StaResCount":    dsrescount,
		"StaEloDiff":     dsrescount,
	})
}
