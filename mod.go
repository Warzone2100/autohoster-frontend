package main

import (
	"context"
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

func APIgetLogs2(_ http.ResponseWriter, r *http.Request) (int, any) {
	return genericViewRequest[struct {
		ID       int       `json:"id"`
		Whensent time.Time `json:"whensent"`
		Pkey     []byte    `json:"pkey"`
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
			"pkey":     "pkey",
			"name":     "name",
			"MsgType":  "msgtype",
			"Msg":      "msg",
		},
	})
}

func APIgetIdentities(_ http.ResponseWriter, r *http.Request) (int, any) {
	return genericViewRequest[struct {
		ID      int
		Name    string
		Pkey    []byte
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
