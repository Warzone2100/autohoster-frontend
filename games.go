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
	LIMIT 10;`, gid).Scan(&ddate, &djson)
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
