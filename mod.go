package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v4"
)

func isSuperadmin(context context.Context, username string) bool {
	ret := false
	derr := dbpool.QueryRow(context, "SELECT superadmin FROM accounts WHERE username = $1", username).Scan(&ret)
	if derr != nil {
		if errors.Is(derr, pgx.ErrNoRows) {
			return false
		}
		log.Printf("Error checking superadmin: %v", derr)
	}
	return ret
}

func modAccountsHandler(w http.ResponseWriter, r *http.Request) {
	if !isSuperadmin(r.Context(), sessionGetUsername(r)) {
		w.WriteHeader(http.StatusForbidden)
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Forbiden"})
		return
	}
	if r.Method == "POST" {
		err := r.ParseMultipartForm(4096)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("Failed to parse form"))
			return
		}
		if !stringOneOf(r.FormValue("param"), "bypass_ispban", "allow_host_request", "terminated", "no_request_reason") {
			w.WriteHeader(403)
			w.Write([]byte("Param is bad (" + r.FormValue("param") + ")"))
			return
		}
		if stringOneOf(r.FormValue("param"), "bypass_ispban", "allow_host_request", "terminated") {
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
		tag, derr := dbpool.Exec(context.Background(), "UPDATE accounts SET "+r.FormValue("param")+" = $1 WHERE username = $2", r.FormValue("val"), r.FormValue("name"))
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
		err = modSendWebhook(fmt.Sprintf("Administrator `%s` changed `%s` to `%s` for user `%s`.", sessionGetUsername(r), r.FormValue("param"), r.FormValue("val"), r.FormValue("name")))
		if err != nil {
			log.Println(err)
		}
		if r.FormValue("param") == "norequest_reason" {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msggreen": true, "msg": "Success"})
			w.Header().Set("Refresh", "1; /moderation/accounts")
		}
	} else {
		rows, derr := dbpool.Query(context.Background(), `select to_json(accounts) from accounts order by id asc;`)
		if derr != nil {
			if derr == pgx.ErrNoRows {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msg": "No games played"})
			} else {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database query error: " + derr.Error()})
			}
			return
		}
		defer rows.Close()
		var accounts []map[string]any
		for rows.Next() {
			var j string
			err := rows.Scan(&j)
			if err != nil {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database scan error: " + err.Error()})
				return
			}
			m := map[string]any{}
			if err := json.Unmarshal([]byte(j), &m); err != nil {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Json parse error: " + err.Error()})
				return
			}
			accounts = append(accounts, m)
		}
		basicLayoutLookupRespond("modAccounts", w, r, map[string]any{
			"accounts": accounts,
		})
	}
}

func modSendWebhook(content string) error {
	return sendWebhook(cfg.GetDSString("", "webhooks", "actions"), content)
}

func modMergeHandler(w http.ResponseWriter, r *http.Request) {
	if !isSuperadmin(r.Context(), sessionGetUsername(r)) {
		w.WriteHeader(http.StatusForbidden)
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Forbiden"})
		return
	}
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("Failed to parse form: " + err.Error()))
			return
		}
		intoID, err := strconv.Atoi(r.FormValue("into"))
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("Failed to parse into field: " + err.Error()))
			return
		}
		var fromIDs []int
		err = json.Unmarshal([]byte(r.FormValue("from")), &fromIDs)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("Failed to parse from ids: " + err.Error()))
			return
		}
		report := fmt.Sprintf("Merging %v into %d\n", fromIDs, intoID)
		intoName := ""
		derr := dbpool.QueryRow(r.Context(), `SELECT name FROM players WHERE id = $1`, intoID).Scan(&intoName)
		if derr != nil {
			report += fmt.Sprintf("Error getting player %d id: %v\n", intoID, derr.Error())
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"nocenter": true, "msg": report})
			return
		}
		totalgames := 0
		for _, i := range fromIDs {
			report += fmt.Sprintf("Merge from %d to %d\n\n", i, intoID)
			if i == intoID {
				report += fmt.Sprintf("Ignoring from id %d since it is equal to to id\n", i)
				continue
			}
			fromName := ""
			derr := dbpool.QueryRow(r.Context(), `SELECT name FROM players WHERE id = $1`, i).Scan(&fromName)
			if derr != nil {
				report += fmt.Sprintf("Error getting player %d name: %v\n", i, derr.Error())
				continue
			}
			report += fmt.Sprintf("Merge from player [%s] (%d) to player [%s] (%d)\n", fromName, i, intoName, intoID)
			tag, derr := dbpool.Exec(r.Context(), `INSERT INTO plrenames (id, oldname, newname) VALUES ($1, $2, $3);`, intoID, fromName, intoName)
			if derr != nil {
				report += fmt.Sprintf("Error updating renames: %v\n", derr.Error())
				continue
			}
			report += fmt.Sprintf("Adding a playerrename: %s\n", tag)
			tag, derr = dbpool.Exec(r.Context(), `UPDATE games SET players = array_replace(players, $1, $2) WHERE $1 = ANY(players);`, i, intoID)
			if derr != nil {
				report += fmt.Sprintf("Error updating games!: %v\n", derr.Error())
				continue
			}
			totalgames += int(tag.RowsAffected())
			report += fmt.Sprintf("Moving games: %s\n", tag)
		}
		report += fmt.Sprintf("Done! Total games affected: %d\n", totalgames)
		modSendWebhook(fmt.Sprintf("Administrator `%s` merged `%v` into `%d`", sessionGetUsername(r), fromIDs, intoID))
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"nocenter": true, "plaintext": true, "msg": report})
	} else {
		basicLayoutLookupRespond("modMerge", w, r, map[string]any{})
	}
}

func modNewsHandler(w http.ResponseWriter, r *http.Request) {
	if !isSuperadmin(r.Context(), sessionGetUsername(r)) {
		w.WriteHeader(http.StatusForbidden)
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Forbiden"})
		return
	}
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("Failed to parse form: " + err.Error()))
			return
		}
		tag, err := dbpool.Exec(r.Context(), `insert into news (title, content, color, when_posted) values ($1, $2, $3, $4)`, r.FormValue("title"), r.FormValue("content"), r.FormValue("color"), r.FormValue("date"))
		result := ""
		if err != nil {
			result = err.Error()
		} else {
			result = tag.String()
		}
		msg := template.HTML(result + `<br><a href="/moderation/news">back</a>`)
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"nocenter": true, "plaintext": true,
			"msg": msg})
	} else {
		basicLayoutLookupRespond("modNews", w, r, map[string]any{})
	}
}

func modBansHandler(w http.ResponseWriter, r *http.Request) {
	if !isSuperadmin(r.Context(), sessionGetUsername(r)) {
		w.WriteHeader(http.StatusForbidden)
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Forbiden"})
		return
	}
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("Failed to parse form: " + err.Error()))
			return
		}
		dur, err := strconv.Atoi(r.FormValue("duration"))
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("Wrong duration: " + err.Error()))
			return
		}
		tag, err := dbpool.Exec(r.Context(), `insert into bans (hash, duration, reason, playerid) values ($1, $2, $3, (select id from players where hash = $1))`, r.FormValue("hash"), dur, r.FormValue("reason"))
		result := ""
		if err != nil {
			result = err.Error()
		} else {
			result = tag.String()
		}
		msg := template.HTML(result + `<br><a href="/moderation/bans">back</a>`)
		modSendWebhook(fmt.Sprintf("Administrator `%s` banned `%v` for `%v` duration `%v`", sessionGetUsername(r), r.FormValue("hash"), r.FormValue("reason"), (time.Duration(dur) * time.Second).String()))
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"nocenter": true, "plaintext": true, "msg": msg})
	} else {
		basicLayoutLookupRespond("modBans", w, r, map[string]any{})
	}
}

func APIgetBans(_ http.ResponseWriter, r *http.Request) (int, any) {
	if !isSuperadmin(r.Context(), sessionGetUsername(r)) {
		return 403, nil
	}
	var ret []byte
	derr := dbpool.QueryRow(r.Context(), `SELECT array_to_json(array_agg(to_json(bans))) FROM bans;`).Scan(&ret)
	if derr != nil {
		return 500, derr
	}
	return 200, ret
}

func APIgetLogs(_ http.ResponseWriter, r *http.Request) (int, any) {
	if !isSuperadmin(r.Context(), sessionGetUsername(r)) {
		return 403, nil
	}
	reqLimit := parseQueryInt(r, "limit", 50)
	if reqLimit > 500 {
		reqLimit = 500
	}
	if reqLimit <= 0 {
		reqLimit = 1
	}
	reqOffset := parseQueryInt(r, "offset", 0)
	if reqOffset < 0 {
		reqOffset = 0
	}
	reqSortOrder := parseQueryStringFiltered(r, "order", "desc", "asc")
	reqSortField := parseQueryStringFiltered(r, "sort", "id", "whensent")

	wherecase := ""
	whereargs := []any{}

	reqFilterJ := parseQueryString(r, "filter", "")
	reqFilterFields := map[string]string{}
	reqDoFilters := false
	if reqFilterJ != "" {
		err := json.Unmarshal([]byte(reqFilterJ), &reqFilterFields)
		if err == nil && len(reqFilterFields) > 0 {
			reqDoFilters = true
		}
	}

	if reqDoFilters {
		val, ok := reqFilterFields["name"]
		if ok {
			whereargs = append(whereargs, val)
			if wherecase == "" {
				wherecase = "WHERE name = $1"
			} else {
				wherecase += " AND name = $1"
			}
		}
		val, ok = reqFilterFields["hash"]
		if ok {
			whereargs = append(whereargs, val)
			if wherecase == "" {
				wherecase = "WHERE starts_with(hash, $1)"
			} else {
				wherecase += fmt.Sprintf(" AND starts_with(hash, $%d)", len(whereargs))
			}
		}
	}

	reqSearch := parseQueryString(r, "search", "")

	similarity := 0.3

	if reqSearch != "" {
		whereargs = append(whereargs, reqSearch)
		if wherecase == "" {
			wherecase = fmt.Sprintf("WHERE similarity(msg, $1::text) > %f or msg ~ $1::text", similarity)
		} else {
			wherecase += fmt.Sprintf(" AND similarity(msg, $%d::text) > %f or msg ~ $1::text", len(whereargs), similarity)
		}
	}

	ordercase := fmt.Sprintf("ORDER BY %s %s", reqSortField, reqSortOrder)
	limiter := fmt.Sprintf("LIMIT %d", reqLimit)
	offset := fmt.Sprintf("OFFSET %d", reqOffset)

	totalsc := make(chan int)
	var totals int
	totalspresent := false

	totalsNoFilterc := make(chan int)
	var totalsNoFilter int
	totalsNoFilterpresent := false

	type DbLogEntry struct {
		ID       int    `json:"id"`
		Whensent string `json:"whensent"`
		Hash     string `json:"hash"`
		Name     string `json:"name"`
		Msg      string `json:"msg"`
	}

	lrowc := make(chan []DbLogEntry)
	var ls []DbLogEntry
	lpresent := false

	echan := make(chan error)
	go func() {
		var c int
		derr := dbpool.QueryRow(r.Context(), `select count(composelog) from composelog;`).Scan(&c)
		if derr != nil {
			echan <- derr
			return
		}
		totalsNoFilterc <- c
	}()
	go func() {
		var c int
		req := `select count(composelog) from composelog ` + wherecase + `;`
		derr := dbpool.QueryRow(r.Context(), req, whereargs...).Scan(&c)
		// log.Println(req)
		if derr != nil {
			echan <- derr
			return
		}
		totalsc <- c
	}()
	go func() {
		req := `SELECT id, to_char(whensent, 'YYYY-MM-DD_HH24:MI:SS'), hash, coalesce(name, ''), coalesce(msg, '') FROM composelog ` + wherecase + ` ` + ordercase + ` ` + offset + ` ` + limiter + ` ;`
		// log.Println(req)
		rows, derr := dbpool.Query(r.Context(), req, whereargs...)
		if derr != nil {
			echan <- derr
			return
		}
		defer rows.Close()
		lStage := []DbLogEntry{}
		for rows.Next() {
			l := DbLogEntry{}
			err := rows.Scan(&l.ID, &l.Whensent, &l.Hash, &l.Name, &l.Msg)
			if err != nil {
				echan <- err
				return
			}
			lStage = append(lStage, l)
		}
		lrowc <- lStage
	}()
	for !(lpresent && totalspresent && totalsNoFilterpresent) {
		select {
		case derr := <-echan:
			if derr == pgx.ErrNoRows {
				return 200, []byte(`{"total": 0, "totalNotFiltered": 0, "rows": []}`)
			}
			return 500, derr
		case ls = <-lrowc:
			lpresent = true
		case totals = <-totalsc:
			totalspresent = true
		case totalsNoFilter = <-totalsNoFilterc:
			totalsNoFilterpresent = true
		}
	}
	return 200, map[string]any{
		"total":            totals,
		"totalNotFiltered": totalsNoFilter,
		"rows":             ls,
	}
}

func modResendEmailConfirm(accountID int) error {
	var email, emailcode string
	err := dbpool.QueryRow(context.Background(), `SELECT email, email_confirm_code FROM accounts WHERE id = $1`, accountID).Scan(&email, &emailcode)
	if err != nil {
		if err == pgx.ErrNoRows {
			return errors.New("no account")
		}
		return err
	}
	return sendgridConfirmcode(email, emailcode)
}
