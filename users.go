package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/jackc/pgx/v4"
)

func usersHandler(w http.ResponseWriter, r *http.Request) {
	if !sessionManager.Exists(r.Context(), "User.Username") || sessionManager.Get(r.Context(), "UserAuthorized") != "True" {
		basicLayoutLookupRespond("noauth", w, r, map[string]interface{}{})
		return
	}
	if !stringOneOf(sessionManager.GetString(r.Context(), "User.Username"), "Flex seal", "vaut") {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Forbiden"})
		return
	}
	if r.Method == "POST" {
		err := r.ParseMultipartForm(4096)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("Failed to parse form"))
			return
		}
		if !stringOneOf(r.FormValue("param"), "allow_preset_request", "allow_host_request", "terminated", "norequest_reason") {
			w.WriteHeader(403)
			w.Write([]byte("Param is bad (" + r.FormValue("param") + ")"))
			return
		}
		if stringOneOf(r.FormValue("param"), "allow_preset_request", "allow_host_request", "terminated") {
			if !stringOneOf(r.FormValue("val"), "true", "false") {
				w.WriteHeader(400)
				w.Write([]byte("Val is bad"))
				return
			}
		}
		if r.FormValue("name") == "" {
			w.WriteHeader(400)
			w.Write([]byte("Name is missing"))
			return
		}
		tag, derr := dbpool.Exec(context.Background(), "UPDATE users SET "+r.FormValue("param")+" = $1 WHERE username = $2", r.FormValue("val"), r.FormValue("name"))
		if derr != nil {
			w.WriteHeader(500)
			log.Println("Database query error: " + derr.Error())
			w.Write([]byte("Database query error: " + derr.Error()))
			return
		}
		if !tag.Update() || tag.RowsAffected() != 1 {
			w.WriteHeader(500)
			log.Println("Sus result " + tag.String())
			w.Write([]byte("Sus result " + tag.String()))
			return
		}
		w.WriteHeader(200)
		// basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msggreen": true, "msg": "Success"})
		// w.Header().Set("Refresh", "1; /users")
	} else {
		rows, derr := dbpool.Query(context.Background(), `select to_json(users) from users order by id asc;`)
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
