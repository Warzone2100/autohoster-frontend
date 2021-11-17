package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v4"
)

func resstatHandler(w http.ResponseWriter, r *http.Request) {
	if !checkUserAuthorized(r) || sessionGetUsername(r) != "Flex seal" {
		basicLayoutLookupRespond(templateNotAuthorized, w, r, map[string]interface{}{})
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
			basicLayoutLookupRespond("error403", w, r, map[string]interface{}{})
			return
		}
		sqbase = areqbase
	}
	sqver := "4.2.1"
	reqver, ok := r.URL.Query()["gamever"]
	if ok {
		sqver = reqver[0]
	}
	rows, derr := dbpool.Query(context.Background(), `
		SELECT
			games.id, players, researchlog
		FROM games
		WHERE researchlog is not null AND baselevel = $1 AND calculated = true AND hidden = false AND deleted = false AND version = $2`, sqbase, sqver)
	if derr != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
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
	best := map[string]bestresearch{}
	for rows.Next() {
		var gid int
		var players []int
		var reslogj string
		err := rows.Scan(&gid, &players, &reslogj)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database scan error: " + err.Error()})
			return
		}
		var reslog []research
		if err := json.Unmarshal([]byte(reslogj), &reslog); err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json parse error: " + err.Error()})
			return
		}
		for _, e := range reslog {
			if int(e.Time) == 2 || gid <= 1032 {
				continue
			}
			bt, ok := best[e.Name]
			if !ok || bt.Time > int(e.Time) {
				n := bestresearch{
					Playerid: players[int(e.Pos)],
					Time:     int(e.Time),
					Gameid:   gid,
				}
				best[e.Name] = n
			}
		}
	}
	plid := map[int][]string{}
	for i, j := range best {
		plida, ok := plid[j.Playerid]
		if !ok {
			plid[j.Playerid] = []string{i}
		} else {
			plid[j.Playerid] = append(plida, i)
		}
	}
	for i, j := range plid {
		var pl GamePlayer
		derr := dbpool.QueryRow(context.Background(), `
					SELECT
						to_json(players)::jsonb || json_build_object('userid', coalesce((SELECT id AS userid FROM users WHERE players.id = users.wzprofile2), -1))::jsonb
					FROM players
					WHERE players.id = $1`, i).Scan(&pl)
		if derr != nil {
			if derr == pgx.ErrNoRows {
				log.Print("Player ", i, " not found")
				continue
			}
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database scan error: " + derr.Error()})
			log.Print(derr.Error())
			return
		}
		for _, v := range j {
			if e, ok := best[v]; ok {
				e.Player = pl
				best[v] = e
			} else {
				log.Print("Wut ", v)
			}
		}
	}
	basicLayoutLookupRespond("resstat", w, r, map[string]interface{}{"Versions": versions, "Best": best, "Selver": sqver, "Selbase": sqbase})
}
