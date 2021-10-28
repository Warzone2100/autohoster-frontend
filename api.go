package main

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

func APItryReachMultihoster(w http.ResponseWriter, r *http.Request) {
	s, m := RequestStatus()
	io.WriteString(w, m)
	if s {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

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
	w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
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
	w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, j)
	w.WriteHeader(http.StatusOK)
}

func APIgetDayAverageByHour(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("Wrong method on graph api [%s]", r.Method)
		return
	}
	rows, derr := dbpool.Query(context.Background(), `select count(gg) as c, extract('hour' from timestarted) as d from games as gg group by d order by d`)
	if derr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(derr.Error())
		return
	}
	defer rows.Close()
	re := make(map[int]int)
	for rows.Next() {
		var d, c int
		err := rows.Scan(&c, &d)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Print(err.Error())
			return
		}
		re[d] = c
	}
	j, err := json.Marshal(re)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, string(j))
	w.WriteHeader(http.StatusOK)
}

func APIgetMapNameCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("Wrong method on graph api [%s]", r.Method)
		return
	}
	rows, derr := dbpool.Query(context.Background(), `select mapname, count(*) as c from games group by mapname order by c desc`)
	if derr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(derr.Error())
		return
	}
	defer rows.Close()
	re := make(map[string]int)
	for rows.Next() {
		var c int
		var n string
		err := rows.Scan(&n, &c)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Print(err.Error())
			return
		}
		re[n] = c
	}
	j, err := json.Marshal(re)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, string(j))
	w.WriteHeader(http.StatusOK)
}

func APIgetReplayFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("Wrong method on graph api [%s]", r.Method)
		return
	}
	params := mux.Vars(r)
	gid := params["gid"]
	dir := "0"
	derr := dbpool.QueryRow(context.Background(), `SELECT coalesce(gamedir) FROM games WHERE id = $1;`, gid).Scan(&dir)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(derr.Error())
		return
	}
	if dir == "-1" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	log.Print(dir)
	replaydir := os.Getenv("MULTIHOSTER_GAMEDIRBASE") + dir + "replay/multiplay/"
	files, err := ioutil.ReadDir(replaydir)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}
	for _, f := range files {
		// log.Println(f.Name())
		if strings.HasSuffix(f.Name(), ".wzrp") {
			h, err := os.Open(replaydir + f.Name())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Print(derr.Error())
				return
			}
			var header [4]byte
			n, err := io.ReadFull(h, header[:])
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Print(derr.Error())
				return
			}
			h.Close()
			if n == 4 && string(header[:]) == "WZrp" {
				w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
				w.Header().Set("Content-Disposition", "attachment; filename=\"autohoster-game-"+gid+".wzrp\"")
				http.ServeFile(w, r, replaydir+f.Name())
				return
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
	return
}
