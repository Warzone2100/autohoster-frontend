package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"strings"
)

const (
	templateNotAuthorized  = "noauth"
	templateErrorForbidden = "error403"
	templatePlainMessage   = "plainmsg"
)

func basicLayoutHandler(page string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		basicLayoutLookupRespond(page, w, r, nil)
	}
}

func basicLayoutLookupRespond(page string, w http.ResponseWriter, r *http.Request, p interface{}) {
	in := layouts.Lookup(page)
	if in != nil {
		var params map[string]interface{}
		if p == nil {
			params = map[string]interface{}{}
		} else {
			m, mk := p.(map[string]interface{})
			if !mk {
				log.Println("Basic respond got parameters interface of wrong type")
			} else {
				params = m
			}
		}
		params["NavWhere"] = page
		if strings.HasPrefix(r.Host, "dev.") {
			params["IsDevWebsite"] = true
		}
		params["IsEloRecalculating"] = isEloRecalculating.Load()
		sessionAppendUser(r, &params)
		w.Header().Set("Server", "TacticalPepe webserver "+CommitHash)
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
		err := in.Execute(w, params)
		if err != nil {
			log.Println(err)
		}
	} else {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}

func basicLayoutLookupExecuteAnonymus(in *template.Template, p interface{}) string {
	m, mk := p.(map[string]interface{})
	if !mk {
		log.Println("Basic respond got parameters interface of wrong type")
	}
	var tpl bytes.Buffer
	err := in.Execute(&tpl, m)
	if err != nil {
		log.Println(err)
	}
	return tpl.String()
}
