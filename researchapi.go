package main

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

func APIgetResearchlogData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("Wrong method on research log api [%s]", r.Method)
		return
	}
	params := mux.Vars(r)
	gid := params["gid"]
	var j string
	derr := dbpool.QueryRow(context.Background(), `SELECT coalesce(researchlog, '{}') FROM games WHERE id = $1`, gid).Scan(&j)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(derr.Error())
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, j)
	w.WriteHeader(http.StatusOK)
}
