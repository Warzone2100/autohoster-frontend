package main

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
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
	Userid     int
	Timeplayed int     `json:",omitempty"`
	Rwon       float64 `json:",omitempty"`
	Rlost      float64 `json:",omitempty"`
}

func PlayersHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	pids := params["id"]
	pid, err := strconv.Atoi(pids)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Badly formatted player id"})
		return
	}
	var pp PlayerLeaderboard
	pp.ID = pid
	type renameEntry struct {
		Oldname string
		Newname string
		Time    string
	}
	renames := []renameEntry{}
	err = RequestMultiple(func() error {
		return dbpool.QueryRow(r.Context(), `
		SELECT name, hash, elo, elo2, autoplayed, autolost, autowon, coalesce((SELECT id FROM users WHERE players.id = users.wzprofile2), -1)
		FROM players WHERE id = $1`, pid).Scan(&pp.Name, &pp.Hash, &pp.Elo, &pp.Elo2, &pp.Autoplayed, &pp.Autolost, &pp.Autowon, &pp.Userid)
	}, func() error {
		var o, n, t string
		_, err := dbpool.QueryFunc(r.Context(), `select oldname, newname, "time"::text from plrenames where id = $1 order by "time" desc;`,
			[]interface{}{pid}, []interface{}{&o, &n, &t}, func(qfr pgx.QueryFuncRow) error {
				renames = append(renames, renameEntry{Oldname: o, Newname: n, Time: t})
				return nil
			})
		return err
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select avg(p.elo2)
		from games as g
		join players as p on g.players[array_position(g.usertype, 'loser')] = p.id
		where
			$1 = any(g.players) and
			g.usertype[array_position(g.players, $1)] = 'winner' and
			ratingdiff[1] != 0`, pid).Scan(&pp.Rwon)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select avg(p.elo2)
		from games as g
		join players as p on g.players[array_position(g.usertype, 'winner')] = p.id
		where
			$1 = any(g.players) and
			g.usertype[array_position(g.players, $1)] = 'loser' and
			ratingdiff[1] != 0`, pid).Scan(&pp.Rlost)
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Player not found"})
		} else if r.Context().Err() == context.Canceled {
			return
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + err.Error()})
		}
		return
	}
	basicLayoutLookupRespond("player", w, r, map[string]interface{}{"Player": pp, "Renames": renames})
}
