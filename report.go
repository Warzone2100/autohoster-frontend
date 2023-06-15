package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func reportHandler(w http.ResponseWriter, r *http.Request) {
	if !checkUserAuthorized(r) {
		respondWithUnauthorized(w, r)
		return
	}

	wzprofile2 := -1
	var lastreport time.Time
	err := dbpool.QueryRow(r.Context(), `SELECT coalesce(wzprofile2, -1), lastreport FROM users WHERE username = $1`, sessionGetUsername(r)).Scan(&wzprofile2, &lastreport)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, contact administrator"})
		log.Println(err)
		return
	}
	if wzprofile2 < 0 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "You must link in-game profile first to be able to report others"})
		return
	}
	if time.Since(lastreport).Hours() < 12 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "You can submit only one report in 12 hours"})
		return
	}

	err = r.ParseForm()
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Invalid form"})
		return
	}

	iViolation := r.FormValue("violation")
	iViolationTime := r.FormValue("violationTime")
	iOffender := r.FormValue("offender")
	iComment := r.FormValue("comment")

	if r.FormValue("agree1") != "on" || r.FormValue("agree2") != "on" || r.FormValue("agree3") != "on" {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "You must understand reporting rules"})
		return
	}

	if iViolation == "" || iOffender == "" || iComment == "" || iViolationTime == "" {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Empty fields not allowed"})
		return
	}
	if len(iViolation) > 80 || len(iViolationTime) > 24 || len(iOffender) > 300 || len(iComment) > 1500 {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "One or more fields are exeeding it's length"})
		return
	}

	_, err = dbpool.Exec(r.Context(), `INSERT INTO reports (reporter, violation, violationtime, offender, comment) VALUES ($1, $2, $3, $4, $5)`,
		sessionGetUsername(r), iViolation, iViolationTime, iOffender, iComment)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, contact administrator"})
		log.Println(err)
		return
	}

	_, err = dbpool.Exec(r.Context(), `UPDATE users SET lastreport = now() WHERE username = $1`, sessionGetUsername(r))
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Error occured, contact administrator"})
		log.Println(err)
		return
	}

	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Report successfully submitted."})
	sendReportWebhook(fmt.Sprintf("User `%s` reported violations `%s` of a player `%s` at `%s` \nComment:\n```\n%s\n```",
		escapeBacktick(sessionGetUsername(r)),
		escapeBacktick(r.FormValue("violation")),
		escapeBacktick(r.FormValue("offender")),
		escapeBacktick(r.FormValue("violationTime")),
		escapeBacktick(r.FormValue("comment"))))
}

func sendReportWebhook(content string) error {
	return sendWebhook(os.Getenv("DISCORD_ADMIN_REPORTS"), content)
}
