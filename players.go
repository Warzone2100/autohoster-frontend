package main

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v4"
)

type PlayerLeaderboard struct {
	ID         int
	Name       string
	Hash       string
	Elo        int
	Elo2       int
	Autoplayed int
	Autolost   int
	Autowon    int
}

func PlayersHandler(w http.ResponseWriter, r *http.Request) {
	sorttypearr, ok := r.URL.Query()["sort"]
	var sortby string
	if !ok || len(sorttypearr[0]) < 1 {
		sortby = "elo"
	} else {
		if sorttypearr[0] != "autoplayed" &&
			sorttypearr[0] != "autowon" &&
			sorttypearr[0] != "autolost" &&
			sorttypearr[0] != "name" &&
			sorttypearr[0] != "id" &&
			sorttypearr[0] != "hash" &&
			sorttypearr[0] != "elo" &&
			sorttypearr[0] != "elo2" {
			sortby = "elo"
		} else {
			sortby = sorttypearr[0]
		}
	}
	sortdirarr, ok := r.URL.Query()["reverse"]
	var sortdir string
	if !ok || len(sortdirarr[0]) < 1 {
		sortdir = "desc"
	} else {
		if sortdirarr[0] != "asc" &&
			sortdirarr[0] != "desc" {
			sortdir = "desc"
		} else {
			sortdir = sortdirarr[0]
		}
	}
	rows, derr := dbpool.Query(context.Background(), `
	SELECT id, name, hash, elo, elo2, autoplayed, autolost, autowon
	FROM players
	ORDER BY `+sortby+` `+sortdir)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "No games played"})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	defer rows.Close()
	var P []PlayerLeaderboard
	for rows.Next() {
		var pp PlayerLeaderboard
		rows.Scan(&pp.ID, &pp.Name, &pp.Hash, &pp.Elo, &pp.Elo2, &pp.Autoplayed, &pp.Autolost, &pp.Autowon)
		P = append(P, pp)
	}
	basicLayoutLookupRespond("players", w, r, map[string]interface{}{"Players": P})
}
