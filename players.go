package main

import (
	"context"
	"encoding/json"
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
}

func PlayersListHandler(w http.ResponseWriter, r *http.Request) {
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
	SELECT id, name, hash, elo, elo2, autoplayed, autolost, autowon, coalesce((SELECT id FROM users WHERE players.id = users.wzprofile2), -1)
	FROM players
	WHERE autoplayed > 2
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
		rows.Scan(&pp.ID, &pp.Name, &pp.Hash, &pp.Elo, &pp.Elo2, &pp.Autoplayed, &pp.Autolost, &pp.Autowon, &pp.Userid)
		P = append(P, pp)
	}
	basicLayoutLookupRespond("players", w, r, map[string]interface{}{"Players": P})
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
	derr := dbpool.QueryRow(context.Background(), `
	SELECT name, hash, elo, elo2, autoplayed, autolost, autowon, coalesce((SELECT id FROM users WHERE players.id = users.wzprofile2), -1)
	FROM players WHERE id = $1`, pid).Scan(&pp.Name, &pp.Hash, &pp.Elo, &pp.Elo2, &pp.Autoplayed, &pp.Autolost, &pp.Autowon, &pp.Userid)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "Player not found"})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	pp.ID = pid
	rows, derr := dbpool.Query(context.Background(), `
	SELECT
		games.id as gid, finished, to_char(timestarted, 'YYYY-MM-DD HH24:MI'), coalesce(to_char(timestarted, 'YYYY-MM-DD HH24:MI'), '==='), gametime,
		players, teams, colour, usertype,
		mapname, maphash,
		baselevel, powerlevel, scavs, alliancetype,
		array_agg(to_json(p)::jsonb || json_build_object('userid', coalesce((SELECT id AS userid FROM users WHERE p.id = users.wzprofile2), -1))::jsonb)::text[] as pnames, kills,
		coalesce(elodiff, '{0,0,0,0,0,0,0,0,0,0,0}'), coalesce(ratingdiff, '{0,0,0,0,0,0,0,0,0,0,0}')
	FROM games
	JOIN players as p ON p.id = any(games.players)
	WHERE deleted = false AND hidden = false AND $1 = any(games.players)
	GROUP BY gid
	ORDER BY timestarted DESC
	LIMIT 100;`, pid)
	if derr != nil {
		if derr == pgx.ErrNoRows {

		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	defer rows.Close()
	var gms []DbGamePreview
	for rows.Next() {
		var g DbGamePreview
		var plid []int
		var plteam []int
		var plcolour []int
		var plusertype []string
		var plsj []string
		var dskills []int
		var dselodiff []int
		var dsratingdiff []int
		err := rows.Scan(&g.ID, &g.Finished, &g.TimeStarted, &g.TimeEnded, &g.GameTime,
			&plid, &plteam, &plcolour, &plusertype,
			&g.MapName, &g.MapHash, &g.BaseLevel, &g.PowerLevel, &g.Scavengers, &g.Alliances, &plsj,
			&dskills, &dselodiff, &dsratingdiff)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database scan error: " + err.Error()})
			return
		}
		var np [11]DbGamePlayerPreview
		for pi, pv := range plsj {
			err := json.Unmarshal([]byte(pv), &np[pi])
			if err != nil {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json unpack error: " + err.Error()})
				return
			}
		}
		for slot, nid := range plid {
			gpi := -1
			for pi, pv := range np {
				if pv.ID == nid {
					gpi = pi
					break
				}
			}
			if gpi == -1 {
				// log.Print("Failed to find player " + strconv.Itoa(slot) + " for game " + strconv.Itoa(g.Id))
				continue
			}
			g.Players[slot] = np[gpi]
			g.Players[slot].Team = plteam[slot]
			g.Players[slot].Colour = plcolour[slot]
			g.Players[slot].Position = slot
			if g.Finished {
				g.Players[slot].Usertype = plusertype[slot]
				g.Players[slot].Kills = dskills[slot]
				if len(dselodiff) > slot {
					g.Players[slot].EloDiff = dselodiff[slot]
					g.Players[slot].RatingDiff = dsratingdiff[slot]
				}
			} else {
				g.Players[slot].Usertype = "fighter"
				g.Players[slot].Kills = 0
			}
		}
		// spew.Dump(g)
		includegame := true
		for _, gameplayer := range g.Players {
			if gameplayer.Usertype == "spectator" && gameplayer.ID == pid {
				includegame = false
				break
			}
		}
		if includegame {
			gms = append(gms, g)
		}
	}
	renameRows, derr := dbpool.Query(context.Background(), `select oldname, newname, "time"::text from plrenames where id = $1 order by "time" desc;`, pid)
	if derr != nil {
		if derr != pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	defer renameRows.Close()
	type renameEntry struct {
		Oldname string
		Newname string
		Time    string
	}
	renames := []renameEntry{}
	for renameRows.Next() {
		var o, n, t string
		renameRows.Scan(&o, &n, &t)
		renames = append(renames, renameEntry{Oldname: o, Newname: n, Time: t})
	}
	basicLayoutLookupRespond("player", w, r, map[string]interface{}{"Player": pp, "Games": gms, "Renames": renames})
}
