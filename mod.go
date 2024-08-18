package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v4"
)

func modSendWebhook(content string) error {
	return sendWebhook(cfg.GetDSString("", "webhooks", "actions"), content)
}

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

func basicSuperadminHandler(page string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isSuperadmin(r.Context(), sessionGetUsername(r)) {
			respondWithForbidden(w, r)
			return
		}
		basicLayoutLookupRespond(page, w, r, nil)
	}
}

func SuperadminCheck(next func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isSuperadmin(r.Context(), sessionGetUsername(r)) {
			respondWithForbidden(w, r)
			return
		}
		next(w, r)
	}
}

func APISuperadminCheck(next func(w http.ResponseWriter, r *http.Request) (int, any)) func(w http.ResponseWriter, r *http.Request) (int, any) {
	return func(w http.ResponseWriter, r *http.Request) (int, any) {
		if !isSuperadmin(r.Context(), sessionGetUsername(r)) {
			return http.StatusForbidden, nil
		}
		return next(w, r)
	}
}

func modAccountsPOST(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(4096)
	if err != nil {
		respondWithCodeAndPlaintext(w, 400, "Failed to parse form")
		return
	}
	if !stringOneOf(r.FormValue("param"), "bypass_ispban", "allow_host_request", "terminated", "no_request_reason") {
		respondWithCodeAndPlaintext(w, 400, "Param is bad ("+r.FormValue("param")+")")
		return
	}
	if stringOneOf(r.FormValue("param"), "bypass_ispban", "allow_host_request", "terminated") {
		if !stringOneOf(r.FormValue("val"), "true", "false") {
			respondWithCodeAndPlaintext(w, 400, "Val is bad")
			return
		}
	}
	if r.FormValue("name") == "" {
		respondWithCodeAndPlaintext(w, 400, "Name is missing")
		return
	}
	tag, derr := dbpool.Exec(context.Background(), "UPDATE accounts SET "+r.FormValue("param")+" = $1 WHERE username = $2", r.FormValue("val"), r.FormValue("name"))
	if derr != nil {
		logRespondWithCodeAndPlaintext(w, 500, "Database query error: "+derr.Error())
		return
	}
	if !tag.Update() || tag.RowsAffected() != 1 {
		logRespondWithCodeAndPlaintext(w, 500, "Sus result "+tag.String())
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
}

func modNewsPOST(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		respondWithCodeAndPlaintext(w, 400, "Failed to parse form: "+err.Error())
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
	basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"nocenter": true, "plaintext": true, "msg": msg})
}

func modBansPOST(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		respondWithCodeAndPlaintext(w, 400, "Failed to parse form: "+err.Error())
		return
	}
	dur := parseFormInt(r, "duration")
	var inExpires *time.Time
	if dur != nil && *dur != 0 {
		d := time.Now().Add(time.Duration(*dur) * time.Second)
		inExpires = &d
	}
	inAccount := parseFormInt(r, "account")
	inIdentity := parseFormInt(r, "identity")
	if inAccount == nil && inIdentity == nil {
		respondWithCodeAndPlaintext(w, 400, "Both identity and account are nil")
		return
	}

	inForbidsJoining := parseFormBool(r, "forbids-joining")
	inForbidsChatting := parseFormBool(r, "forbids-chatting")
	inForbidsPlaying := parseFormBool(r, "forbids-playing")

	tag, err := dbpool.Exec(r.Context(),
		`insert into bans
(account, identity, time_expires, reason, forbids_joining, forbids_chatting, forbids_playing) values
($1, $2, $3, $4, $5, $6, $7)`, inAccount, inIdentity, inExpires, r.FormValue("reason"),
		inForbidsJoining, inForbidsChatting, inForbidsPlaying)
	result := ""
	if err != nil {
		result = err.Error()
	} else {
		result = tag.String()
	}
	msg := template.HTML(result + `<br><a href="/moderation/bans">back</a>`)
	modSendWebhook(fmt.Sprintf("Administrator `%s` banned"+
		"\naccount `%+#v` identity `%+#v`"+
		"\nfor `%+#v` (ends at `%+#v`)"+
		"\nduration `%+#v`"+
		"\njoining `%+#v` `%+#v`"+
		"\nchatting `%+#v` `%+#v`"+
		"\nplaying `%+#v` `%+#v`",
		sessionGetUsername(r),
		r.FormValue("account"), r.FormValue("identity"),
		r.FormValue("reason"), dur, inExpires,
		r.FormValue("forbids-joining"), inForbidsJoining,
		r.FormValue("forbids-chatting"), inForbidsChatting,
		r.FormValue("forbids-playing"), inForbidsPlaying))
	basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"nocenter": true, "plaintext": true, "msg": msg})
}

func APIgetBans(_ http.ResponseWriter, r *http.Request) (int, any) {
	var ret []byte
	derr := dbpool.QueryRow(r.Context(), `SELECT array_to_json(array_agg(to_json(bans))) FROM bans;`).Scan(&ret)
	if derr != nil {
		return 500, derr
	}
	return 200, ret
}

func APIgetLogs(_ http.ResponseWriter, r *http.Request) (int, any) {
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
				wherecase = "WHERE starts_with(pkey, $1)"
			} else {
				wherecase += fmt.Sprintf(" AND starts_with(pkey, $%d)", len(whereargs))
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
		Pkey     string `json:"pkey"`
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
		req := `SELECT id, to_char(whensent, 'YYYY-MM-DD_HH24:MI:SS'), pkey, coalesce(name, ''), coalesce(msg, '') FROM composelog ` + wherecase + ` ` + ordercase + ` ` + offset + ` ` + limiter + ` ;`
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
			err := rows.Scan(&l.ID, &l.Whensent, &l.Pkey, &l.Name, &l.Msg)
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

func APIgetLogs2(_ http.ResponseWriter, r *http.Request) (int, any) {
	return genericViewRequest[struct {
		ID       int       `json:"id"`
		Whensent time.Time `json:"whensent"`
		Pkey     string    `json:"pkey"`
		Name     string    `json:"name"`
		MsgType  string    `json:"msgtype"`
		Msg      string    `json:"msg"`
	}](r, genericRequestParams{
		tableName:               "composelog",
		limitClamp:              1500,
		sortDefaultOrder:        "desc",
		sortDefaultColumn:       "id",
		sortColumns:             []string{"id", "whensent"},
		filterColumnsFull:       []string{"id", "msg"},
		filterColumnsStartsWith: []string{"name", "pkey"},
		searchColumn:            "name || msg",
		searchSimilarity:        0.3,
		columnMappings: map[string]string{
			"ID":       "id",
			"Whensent": "whensent",
			"Pkey":     "pkey",
			"Name":     "name",
			"MsgType":  "msgtype",
			"Msg":      "msg",
		},
		columnsSpecifier: "id, whensent, encode(pkey, 'base64'), name, msg",
	})
}

func APIgetIdentities(_ http.ResponseWriter, r *http.Request) (int, any) {
	return genericViewRequest[struct {
		ID      int
		Name    string
		Pkey    string
		Hash    string
		Account *int
	}](r, genericRequestParams{
		tableName:               "identities_view",
		limitClamp:              500,
		sortDefaultOrder:        "desc",
		sortDefaultColumn:       "id",
		sortColumns:             []string{"id", "name", "account"},
		filterColumnsFull:       []string{"id", "account"},
		filterColumnsStartsWith: []string{"name", "pkey", "hash"},
		searchColumn:            "name",
		searchSimilarity:        0.3,
		columnMappings: map[string]string{
			"ID":      "id",
			"Name":    "name",
			"Pkey":    "pkey",
			"Hash":    "hash",
			"Account": "account",
		},
	})
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

func modIdentitiesHandler(w http.ResponseWriter, r *http.Request) {

}
