package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v4"
)

func usersHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
		return
	}
	if sessionManager.GetString(r.Context(), "User.Username") != "Flex seal" {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Forbiden"})
		return
	}
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Form parse error: " + err.Error()})
			return
		}
	} else {
		rows, derr := dbpool.Query(context.Background(), `select to_json(users) from users order by id desc;`)
		if derr != nil {
			if derr == pgx.ErrNoRows {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "No games played"})
			} else {
					basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
			}
			return
		}
		defer rows.Close()
		var users []map[string]interface{}
		for rows.Next() {
			var j string
			err := rows.Scan(&j)
			if err != nil {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database scan error: " + err.Error()})
				return
			}
			m := map[string]interface{}{}
			if err := json.Unmarshal([]byte(j), &m); err != nil {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json parse error: " + err.Error()})
				return
			}
			users = append(users, m)
		}
		basicLayoutLookupRespond("users", w, r, map[string]interface{}{
			"Users": users,
		})
	}
}
