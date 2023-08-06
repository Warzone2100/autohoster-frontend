package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/davecgh/go-spew/spew"
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
	var j []map[string]interface{}
	derr := dbpool.QueryRow(context.Background(), `SELECT coalesce(researchlog, '[]')::jsonb FROM games WHERE id = $1`, gid).Scan(&j)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(derr.Error())
		return
	}
	for i := range j {
		for k, v := range j[i] {
			if k == "name" {
				j[i][k] = getResearchName(v.(string))
			}
		}
	}
	b, err := json.Marshal(j)
	if err != nil {
		log.Println(err)
	}
	w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
	w.WriteHeader(http.StatusOK)
}

type resEntry struct {
	Name     string  `json:"name"`
	Position float64 `json:"position"`
	Time     float64 `json:"time"`
}

var (
	researchClassification []map[string]string
)

func LoadClassification() (ret []map[string]string, err error) {
	var content []byte
	content, err = os.ReadFile(os.Getenv("CLASSIFICATIONJSON"))
	if err != nil {
		return
	}
	err = json.Unmarshal(content, &ret)
	return
}

// CountClassification in: classification, research out: position[research[time]]
func CountClassification(resl []resEntry) (ret map[int]map[string]int) {
	cl := map[string]string{}
	ret = map[int]map[string]int{}
	for _, b := range researchClassification {
		cl[b["name"]] = b["Subclass"]
	}
	for _, b := range resl {
		if b.Time < 10 {
			continue
		}
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
	return
}

func getPlayerClassifications(pid int) (total, recent map[string]int, err error) {
	total = map[string]int{}
	recent = map[string]int{}
	rows, err := dbpool.Query(context.Background(),
		`SELECT coalesce(id, -1), coalesce(researchlog, ''), array_position(players, $1)-1
		FROM games 
		WHERE 
			$1 = any(players)
			AND array_position(players, -1)-1 = 2
			AND finished = true 
			AND calculated = true 
			AND hidden = false 
			AND deleted = false 
			AND id > 1032
		ORDER BY id desc`, pid)
	if err != nil {
		if err == pgx.ErrNoRows {
			return
		}
		return
	}
	type gameResearch struct {
		gid       int
		playerpos int
		research  string
		cl        map[int]map[string]int
	}
	games := []gameResearch{}
	for rows.Next() {
		g := gameResearch{}
		err = rows.Scan(&g.gid, &g.research, &g.playerpos)
		if err != nil {
			return
		}
		games = append(games, g)
	}
	for i, g := range games {
		var resl []resEntry
		err = json.Unmarshal([]byte(g.research), &resl)
		if err != nil {
			log.Print(err.Error())
			log.Print(spew.Sdump(g))
			continue
		}
		games[i].cl = CountClassification(resl)
		for v, c := range games[i].cl[g.playerpos] {
			if val, ok := total[v]; ok {
				total[v] = val + c
			} else {
				total[v] = c
			}
		}
	}
	lastlen := len(games)
	if lastlen > 200 {
		lastlen = 200
	}
	for _, g := range games[:lastlen] {
		for v, c := range g.cl[g.playerpos] {
			if val, ok := recent[v]; ok {
				recent[v] = val + c
			} else {
				recent[v] = c
			}
		}
	}
	err = nil
	return
}

func APIresearchClassification(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	params := mux.Vars(r)
	pids := params["pid"]
	pid, err := strconv.Atoi(pids)
	if err != nil {
		return 400, nil
	}
	a, b, err := getPlayerClassifications(pid)
	_ = a
	_ = b
	_ = err
	return 200, a
}
