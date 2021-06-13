package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

func listGamesHandler(w http.ResponseWriter, r *http.Request) {
	rows, derr := dbpool.Query(context.Background(), `
	SELECT
		id,
		to_char(time_finished, 'YYYY-MM-DD HH24:MI'),
		game
	FROM jgames
	WHERE cast(game as text) != 'null' AND (game->>'gameTime')::int/1000 > 60
	ORDER BY time_finished DESC
	LIMIT 10;`) //
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
	LIMIT 10;`, gid).Scan(&ddate, &djson)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Game not found"})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	m := map[string]interface{}{}
	if err := json.Unmarshal([]byte(djson), &m); err != nil {
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
	/*
		extendedPlayerData: [
			{
				researchComplite: [
					{...}
				]
			},
			{
				researchComplite: [
					{...}
				]
			}
		]



		researchTimes: [
			{
				name: R-Cyborg-Armor-Heat01
				tiems: [
					4m48s, 	4m54s, 	4m54s, 	4m54s, 	4m48s, 	4m48s
				]
			},
		]
	*/
	// WARNING SHITCODE AHEAD
	res := map[string][11]string{}
	for _, bbb := range m["extendedPlayerData"].([]interface{}) {
		rrr := bbb.(map[string]interface{})["researchComplite"]
		for _, bb := range rrr.([]interface{}) {
			rr := bb.(map[string]interface{})
			var b [11]string
			b = res[rr["name"].(string)]
			// bindex, _ := strconv.Atoi(rr["player"].(float64))
			// b[int(rr["player"].(float64))] = rr["time"].(float64)
			b[int(rr["player"].(float64))] = (time.Duration(int(rr["time"].(float64)/1000)) * time.Second).String()
			res[rr["name"].(string)] = b
		}
	}
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
		if res[resval][0] != "0s" {
			resSorted[resval] = res[resval]
		}
	}
	basicLayoutLookupRespond("gamedetails", w, r, map[string]interface{}{
		"ID":        gid,
		"Date":      ddate,
		"JMap":      m,
		"Gametime":  gtstr,
		"Res":       res,
		"ResSorted": resSorted,
		"IsNull":    djson,
	})
}
