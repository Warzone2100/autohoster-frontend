package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v4"
)

var (
	researchNamed = map[string]string{}
)

func prepareStatNames() {
	b, err := os.ReadFile("./research.json")
	if err != nil {
		log.Fatal(err)
	}
	var r map[string]any
	err = json.Unmarshal(b, &r)
	if err != nil {
		log.Fatal(err)
	}
	for k, v := range r {
		v1, ok := v.(map[string]any)
		if !ok {
			log.Printf("Research [%s] object is not a msi", k)
			continue
		}
		v2, ok := v1["name"]
		if !ok {
			log.Printf("Research [%s] has no name", k)
			continue
		}
		v3, ok := v2.(string)
		if !ok {
			log.Printf("Research [%s] name not a string", k)
			continue
		}
		researchNamed[k] = v3
	}
}

func getResearchName(n string) string {
	r, ok := researchNamed[n]
	if !ok {
		return n
	}
	return r
}

func resstatHandler(w http.ResponseWriter, r *http.Request) {
	if !checkUserAuthorized(r) {
		basicLayoutLookupRespond(templateNotAuthorized, w, r, map[string]any{})
		return
	}
	var versions []string
	derr := dbpool.QueryRow(context.Background(), `SELECT array_agg(DISTINCT COALESCE(version, 'any')) FROM games;`).Scan(&versions)
	if derr != nil {
		log.Print(derr.Error())
		return
	}
	sqbase := 1
	reqbase, ok := r.URL.Query()["base"]
	if ok && (reqbase[0] == "0" || reqbase[0] == "1" || reqbase[0] == "2") {
		areqbase, err := strconv.Atoi(reqbase[0])
		if err != nil {
			basicLayoutLookupRespond("error403", w, r, map[string]any{})
			return
		}
		sqbase = areqbase
	}
	sqver := "4.2.1"
	reqver, ok := r.URL.Query()["gamever"]
	if ok {
		sqver = reqver[0]
	}
	ming := 1032
	rming, ok := r.URL.Query()["gamelimit"]
	if ok {
		sming, err := strconv.Atoi(rming[0])
		if err == nil {
			ming = sming
		}
	}
	llim := 3
	rllim, ok := r.URL.Query()["leadlim"]
	if ok {
		sllim, err := strconv.Atoi(rllim[0])
		if err == nil {
			llim = sllim
		}
	}
	var rows pgx.Rows
	if sqver == "any" {
		rows, derr = dbpool.Query(context.Background(), `
		SELECT
		games.id, players, researchlog
		FROM games
		WHERE researchlog is not null AND baselevel = $1 AND calculated = true AND hidden = false AND deleted = false AND alliancetype = 3 AND id > $2`, sqbase, ming)
	} else {
		rows, derr = dbpool.Query(context.Background(), `
		SELECT
		games.id, players, researchlog
		FROM games
		WHERE researchlog is not null AND baselevel = $1 AND calculated = true AND hidden = false AND deleted = false AND alliancetype = 3 AND version = $2 AND id > $3`, sqbase, sqver, ming)
	}
	if derr != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database query error: " + derr.Error()})
		return
	}
	defer rows.Close()
	type GamePlayer struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Hash       string `json:"hash"`
		Autoplayed int    `json:"autoplayed"`
		Autolost   int    `json:"autolost"`
		Autowon    int    `json:"autowon"`
		Elo        int    `json:"elo"`
		Elo2       int    `json:"elo2"`
		Userid     int    `json:"userid"`
	}
	type bestresearch struct {
		Playerid int
		Player   GamePlayer
		Time     int
		Gameid   int
	}
	type research struct {
		Name string  `json:"name"`
		Pos  float64 `json:"position"`
		Time float64 `json:"time"`
	}
	bestr := map[string][]bestresearch{}
	for rows.Next() {
		var gid int
		var players []int
		var reslogj string
		err := rows.Scan(&gid, &players, &reslogj)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database scan error: " + err.Error()})
			return
		}
		var reslog []research
		if err := json.Unmarshal([]byte(reslogj), &reslog); err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Json parse error: " + err.Error()})
			return
		}
		for _, e := range reslog {
			if int(e.Time) == 2 || gid <= 1032 || players[int(e.Pos)] == 321 {
				continue
			}
			bt, ok := bestr[getResearchName(e.Name)]
			if !ok {
				bt = []bestresearch{}
			}
			bt = append(bt, bestresearch{
				Playerid: players[int(e.Pos)],
				Time:     int(e.Time),
				Gameid:   gid,
			})
			bestr[getResearchName(e.Name)] = bt
		}
	}
	best := map[string][]bestresearch{}
	plid := map[int][]string{}
	for k, b := range bestr {
		sort.Slice(b, func(i, j int) bool { return b[i].Time < b[j].Time })
		bestfiltered := []bestresearch{}
		was := []int{}
		for _, v := range b {
			found := false
			for _, vv := range was {
				if vv == v.Playerid {
					found = true
					break
				}
			}
			if !found {
				bestfiltered = append(bestfiltered, v)
				was = append(was, v.Playerid)
				plida, ok := plid[v.Playerid]
				if !ok {
					plid[v.Playerid] = []string{k}
				} else {
					plid[v.Playerid] = append(plida, k)
				}
				if len(bestfiltered) > llim {
					break
				}
			}
		}
		if len(bestfiltered) > llim {
			bestfiltered = bestfiltered[:llim]
		}
		best[k] = bestfiltered
	}
	for i, j := range plid {
		var pl GamePlayer
		derr := dbpool.QueryRow(context.Background(), `
					SELECT
						to_json(players)::jsonb || json_build_object('userid', coalesce((SELECT id AS userid FROM accounts WHERE players.id = accounts.wzprofile2), -1))::jsonb
					FROM players
					WHERE players.id = $1`, i).Scan(&pl)
		if derr != nil {
			if derr == pgx.ErrNoRows {
				log.Print("Player ", i, " not found")
				continue
			}
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database scan error: " + derr.Error()})
			log.Print(derr.Error())
			return
		}
		for _, v := range j {
			if e, ok := best[v]; ok {
				for ii := range e {
					if e[ii].Playerid == i {
						e[ii].Player = pl
					}
				}
				best[v] = e
			} else {
				log.Print("Wut ", v)
			}
		}
	}
	basicLayoutLookupRespond("resstat", w, r, map[string]any{"Versions": versions, "Best": best, "Selver": sqver, "Selbase": sqbase})
}
