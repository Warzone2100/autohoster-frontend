package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

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
		if r.PostFormValue("param") != "allow_preset_request" && r.PostFormValue("param") != "allow_host_request" && r.PostFormValue("param") != "norequest_reason" {
			basicLayoutLookupRespond("error403", w, r, map[string]interface{}{})
			return
		}
		if r.PostFormValue("param") == "allow_preset_request" || r.PostFormValue("param") == "allow_host_request" {
			if r.PostFormValue("val") != "true" && r.PostFormValue("val") != "false" {
				basicLayoutLookupRespond("error403", w, r, map[string]interface{}{})
				return
			}
		}
		if r.PostFormValue("name") == "" {
			basicLayoutLookupRespond("error403", w, r, map[string]interface{}{})
			return
		}
		tag, derr := dbpool.Exec(context.Background(), "UPDATE users SET "+r.PostFormValue("param")+" = $1 WHERE username = $2", r.PostFormValue("val"), r.PostFormValue("name"))
		if derr != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
			return
		}
		if tag.RowsAffected() != 1 {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Rows affected " + strconv.Itoa(int(tag.RowsAffected()))})
			return
		}
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msggreen": true, "msg": "Success"})
		w.Header().Set("Refresh", "1; /users")
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
