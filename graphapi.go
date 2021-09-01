package main

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

func APIgetGraphData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("Wrong method on graph api [%s]", r.Method)
		return
	}
	params := mux.Vars(r)
	gid := params["gid"]
	var j string
	derr := dbpool.QueryRow(context.Background(), `SELECT json_agg(frames)::text FROM frames WHERE game = $1`, gid).Scan(&j)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(derr.Error())
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "https://tacticalpepe.me https://dev.tacticalpepe.me")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, j)
	w.WriteHeader(http.StatusOK)
}

func APIgetDatesGraphData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("Wrong method on graph api [%s]", r.Method)
		return
	}
	params := mux.Vars(r)
	interval := params["interval"]
	var j string
	derr := dbpool.QueryRow(context.Background(), `select
		json_agg(json_build_object(b::text,(select count(*) from games where date_trunc($1, timestarted) = b)))
	from generate_series('2021-07-07'::timestamp, now(), $2::interval) as b;`, interval, "1 "+interval).Scan(&j)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(derr.Error())
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "https://tacticalpepe.me https://dev.tacticalpepe.me")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, j)
	w.WriteHeader(http.StatusOK)
}
