package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v4"
)

func gamesHandler(w http.ResponseWriter, r *http.Request) {
	// if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
	// 	basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
	// 	return
	// }
	rows, derr := dbpool.Query(context.Background(), `select id, to_char(time_finished, 'YYYY-MM-DD HH24:MI'), game from jgames where cast(game as text) != 'null' order by time_finished desc limit 10;`)
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
		Id   int
		Date string
		Map  map[string]interface{}
		Json string
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
		n := GamePrototype{id, date, m, jsonf}
		games = append(games, n)
	}
	basicLayoutLookupRespond("games", w, r, map[string]interface{}{
		"Games": games,
	})
}
