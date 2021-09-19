package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
)

func checkPresetEditPermission(username string) bool {
	var a bool
	derr := dbpool.QueryRow(context.Background(), `SELECT allow_preset_edit FROM users WHERE username = $1`, username).Scan(&a)
	return derr == nil && a
}

func getPresetsFromDatabase() ([]map[string]interface{}, error) {
	var presets []map[string]interface{}
	rows, derr := dbpool.Query(context.Background(), `SELECT to_json(presets) FROM presets ORDER BY id ASC`)
	if derr != nil {
		log.Print(derr.Error())
		return presets, derr
	}
	defer rows.Close()
	for rows.Next() {
		var j string
		if err := rows.Scan(&j); err != nil {
			log.Print(err.Error())
			return presets, err
		}
		m := map[string]interface{}{}
		if err := json.Unmarshal([]byte(j), &m); err != nil {
			log.Print(err.Error())
			return presets, err
		}
		presets = append(presets, m)
	}
	return presets, derr
}

func presetEditorHandler(w http.ResponseWriter, r *http.Request) {
	if !checkUserAuthorized(r) {
		respondWithUnauthorized(w, r)
		return
	}
	if !checkPresetEditPermission(sessionGetUsername(r)) {
		respondWithForbidden(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if !checkFormParse(w, r) {
			return
		}
		respondWithNotImplemented(w, r)
	} else {
		presets, err := getPresetsFromDatabase()
		if !checkRespondGenericErrorAny(w, r, err) {
			return
		}
		basicLayoutLookupRespond("presetedit", w, r, map[string]interface{}{
			"Presets": presets,
		})
	}
}
