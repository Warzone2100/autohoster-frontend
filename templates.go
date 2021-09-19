package main

import (
	"log"
	"net/http"
)

const (
	templateNotAuthorized  = "noauth"
	templateErrorForbidden = "error403"
	templatePlainMessage   = "plainmsg"
)

func basicLayoutLookupRespond(page string, w http.ResponseWriter, r *http.Request, p interface{}) {
	in := layouts.Lookup(page)
	if in != nil {
		m, mk := p.(map[string]interface{})
		if mk == false {
			log.Println("Basic respond got parameters interface of wrong type")
		}
		m["NavWhere"] = page
		sessionAppendUser(r, &m)
		w.Header().Set("Server", "TacticalPepe webserver "+CommitHash)
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
		err := in.Execute(w, m)
		if err != nil {
			log.Println(err)
		}
	} else {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}
