package main

import (
	"context"
	"encoding/json"
	"fmt"
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
		w.WriteHeader(http.StatusMethodNotAllowed)
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
		w.WriteHeader(http.StatusMethodNotAllowed)
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
		w.WriteHeader(http.StatusMethodNotAllowed)
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
		w.WriteHeader(http.StatusMethodNotAllowed)
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
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !checkUserAuthorized(r) {
		w.WriteHeader(http.StatusUnauthorized)
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
				log.Print(err.Error())
				return
			}
			var header [4]byte
			n, err := io.ReadFull(h, header[:])
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Print(err.Error())
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
}

func APIgetClassChartGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	params := mux.Vars(r)
	gid := params["gid"]
	reslog := "0"
	derr := dbpool.QueryRow(context.Background(), `SELECT coalesce(researchlog) FROM games WHERE id = $1;`, gid).Scan(&reslog)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(derr.Error())
		return
	}
	if reslog == "-1" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	ret := map[int]map[string]int{}
	cf, err := os.ReadFile(os.Getenv("CLASSIFICATIONJSON"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}
	var c []map[string]string
	err = json.Unmarshal(cf, &c)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}
	type resEntry struct {
		Name     string  `json:"name"`
		Position float64 `json:"position"`
		Time     float64 `json:"time"`
	}
	var resl []resEntry
	err = json.Unmarshal([]byte(reslog), &resl)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}
	cl := map[string]string{}
	for _, b := range c {
		cl[b["name"]] = b["Subclass"]
	}
	for _, b := range resl {
		j, f := cl[b.Name]
		if f {
			_, ff := ret[int(b.Position)]
			if !ff {
				ret[int(b.Position)] = map[string]int{}
			}
			_, ff = ret[int(b.Position)][j]
			if ff {
				ret[int(b.Position)][j]++
			} else {
				ret[int(b.Position)][j] = 1
			}
		}
	}
	ans, err := json.Marshal(ret)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, string(ans))
	io.WriteString(w, string("\n"))
	w.WriteHeader(http.StatusOK)
}

func APIgetPlayerAllowedJoining(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	params := mux.Vars(r)
	phash := params["hash"]
	badplayed := 0
	derr := dbpool.QueryRow(context.Background(), `SELECT COUNT(id) FROM games WHERE (SELECT id FROM players WHERE hash = $1) = ANY(players) AND gametime < 30000 AND timestarted+'1 day' > now();`, phash).Scan(&badplayed)
	if derr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(derr.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, fmt.Sprint(badplayed))
	log.Println(badplayed)
}
