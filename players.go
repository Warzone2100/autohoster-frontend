package main

import (
	"context"
	"encoding/hex"
	"errors"
	"log"
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
	LastGame   int     `json:",omitempty"`
}

func PlayersHandler(w http.ResponseWriter, r *http.Request) {
	identPubKeyHex := mux.Vars(r)["id"]
	identPubKey, err := hex.DecodeString(identPubKeyHex)
	if err != nil {
		log.Println(err)
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msg": "Badly formatted player id"})
		return
	}
	var identID int
	var identName string
	err = dbpool.QueryRow(r.Context(), `select id, name from identities where pkey = $1 or hash = encode(sha256($1), 'hex')`, identPubKey).Scan(&identID, &identName)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, pgx.ErrNoRows) {
			log.Println(err)
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msg": "request error"})
			return
		}
	}
	basicLayoutLookupRespond("player", w, r, map[string]any{
		"Player": map[string]any{
			"Name":           identName,
			"IdentityPubKey": identPubKeyHex,
		},
	})

	// var pp PlayerLeaderboard
	// pp.ID = pid
	// type renameEntry struct {
	// 	Oldname string
	// 	Newname string
	// 	Time    string
	// }
	// renames := []renameEntry{}
	// ChartGamesByPlayercount := newSC("Games by player count", "Game count", "Player count")
	// ChartGamesByBaselevel := newSC("Games by base level", "Game count", "Base level")
	// ChartGamesByAlliances := newSC("Games by alliance type (2x2+)", "Game count", "Alliance type")
	// ChartGamesByScav := newSC("Games by scavengers", "Game count", "Scavengers")
	// RatingHistory := map[string]eloHist{}
	// ResearchClassificationTotal := map[string]int{}
	// ResearchClassificationRecent := map[string]int{}
	// AltCount := 0
	// err = dbpool.QueryRow(r.Context(), `
	// 	SELECT name, hash, elo, elo2, autoplayed, autolost, autowon, coalesce((SELECT id FROM accounts WHERE players.id = accounts.wzprofile2), -1)
	// 	FROM players WHERE id = $1`, pid).Scan(&pp.Name, &pp.Hash, &pp.Elo, &pp.Elo2, &pp.Autoplayed, &pp.Autolost, &pp.Autowon, &pp.Userid)
	// if err != nil {
	// 	if err == pgx.ErrNoRows {
	// 		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msg": "Player not found"})
	// 	} else if r.Context().Err() == context.Canceled {
	// 		return
	// 	} else {
	// 		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database query error: " + err.Error()})
	// 	}
	// 	return
	// }
	// err = RequestMultiple(func() error {
	// 	var o, n, t string
	// 	_, err := dbpool.QueryFunc(r.Context(), `select oldname, newname, "time"::text from plrenames where id = $1 order by "time" desc;`,
	// 		[]any{pid}, []any{&o, &n, &t}, func(_ pgx.QueryFuncRow) error {
	// 			renames = append(renames, renameEntry{Oldname: o, Newname: n, Time: t})
	// 			return nil
	// 		})
	// 	return err
	// }, func() error {
	// 	return dbpool.QueryRow(r.Context(), `select coalesce(avg(p.elo2), 0)
	// 		from games as g
	// 		cross join unnest(g.players) as up
	// 		join players as p on up = p.id
	// 		where
	// 			array[$1::int] <@ g.players and
	// 			calculated = true and
	// 			finished = true and
	// 			g.usertype[array_position(g.players, $1)] = 'winner' and
	// 			g.usertype[array_position(g.players, up)] = 'loser' and
	// 			ratingdiff[1] != 0`, pid).Scan(&pp.Rwon)
	// }, func() error {
	// 	return dbpool.QueryRow(r.Context(), `select coalesce(avg(p.elo2), 0)
	// 		from games as g
	// 		cross join unnest(g.players) as up
	// 		join players as p on up = p.id
	// 		where
	// 			array[$1::int] <@ g.players and
	// 			calculated = true and
	// 			finished = true and
	// 			g.usertype[array_position(g.players, $1)] = 'loser' and
	// 			g.usertype[array_position(g.players, up)] = 'winner' and
	// 			ratingdiff[1] != 0`, pid).Scan(&pp.Rlost)
	// }, func() error {
	// 	var k, c int
	// 	var ut string
	// 	_, err := dbpool.QueryFunc(r.Context(),
	// 		`select array_position(players, -1)-1 as pc, coalesce(usertype[array_position(players, $1)], '') as ut, count(id) as c
	// 		from games
	// 		where
	// 			array[$1::int] <@ players and
	// 			calculated = true and
	// 			finished = true
	// 		group by pc, ut
	// 		order by pc, ut`,
	// 		[]any{pid}, []any{&k, &ut, &c},
	// 		func(_ pgx.QueryFuncRow) error {
	// 			switch ut {
	// 			case "loser":
	// 				ChartGamesByPlayercount.appendToColumn(fmt.Sprintf("%dp", k), "Lost", chartSCcolorLost, c)
	// 			case "winner":
	// 				ChartGamesByPlayercount.appendToColumn(fmt.Sprintf("%dp", k), "Won", chartSCcolorWon, c)
	// 			}
	// 			return nil
	// 		})
	// 	return err
	// }, func() error {
	// 	var k, c int
	// 	var ut string
	// 	_, err := dbpool.QueryFunc(r.Context(),
	// 		`select baselevel, usertype[array_position(players, $1)] as ut, count(id)
	// 		from games
	// 		where
	// 			array[$1::int] <@ players and
	// 			calculated = true and
	// 			finished = true
	// 		group by baselevel, ut
	// 		order by baselevel, ut`,
	// 		[]any{pid}, []any{&k, &ut, &c},
	// 		func(_ pgx.QueryFuncRow) error {
	// 			switch ut {
	// 			case "loser":
	// 				ChartGamesByBaselevel.appendToColumn(fmt.Sprintf(`<img class="icons icons-base%d">`, k), "Lost", chartSCcolorLost, c)
	// 			case "winner":
	// 				ChartGamesByBaselevel.appendToColumn(fmt.Sprintf(`<img class="icons icons-base%d">`, k), "Won", chartSCcolorWon, c)
	// 			}
	// 			return nil
	// 		})
	// 	return err
	// }, func() error {
	// 	var k, c int
	// 	_, err := dbpool.QueryFunc(r.Context(),
	// 		`select alliancetype, count(id)
	// 		from games
	// 		where
	// 			array[$1::int] <@ players and
	// 			calculated = true and
	// 			finished = true and
	// 			array_position(players, -1)-1 > 2
	// 		group by alliancetype`,
	// 		[]any{pid}, []any{&k, &c},
	// 		func(_ pgx.QueryFuncRow) error {
	// 			if k == 1 {
	// 				return nil
	// 			}
	// 			ChartGamesByAlliances.appendToColumn(fmt.Sprintf(`<img class="icons icons-alliance%d">`, templatesAllianceToClassI(k)), "", chartSCcolorNeutral, c)
	// 			return nil
	// 		})
	// 	return err
	// }, func() error {
	// 	var k, c int
	// 	_, err := dbpool.QueryFunc(r.Context(),
	// 		`select scavs::int, count(id)
	// 		from games
	// 		where
	// 			array[$1::int] <@ players and
	// 			calculated = true and
	// 			finished = true
	// 		group by scavs`,
	// 		[]any{pid}, []any{&k, &c},
	// 		func(_ pgx.QueryFuncRow) error {
	// 			ChartGamesByScav.appendToColumn(fmt.Sprintf(`<img class="icons icons-scav%d">`, k), "", chartSCcolorNeutral, c)
	// 			return nil
	// 		})
	// 	return err
	// }, func() error {
	// 	var err error
	// 	ResearchClassificationTotal, ResearchClassificationRecent, err = getPlayerClassifications(pid)
	// 	return err
	// }, func() error {
	// 	var err error
	// 	RatingHistory, err = getRatingHistory(pid)
	// 	return err
	// }, func() error {
	// 	return dbpool.QueryRow(r.Context(), `select count(*) from players where hash = any((select distinct hash from chatlog where ip = any((select distinct ip from chatlog where hash = any((select hash from players where hash = any((select distinct hash from chatlog where ip = any((select distinct ip from chatlog where hash = $1)))) order by id desc)) order by ip desc))));`, pp.Hash).Scan(&AltCount)
	// })
	// if err != nil {
	// 	if err == pgx.ErrNoRows {
	// 		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msg": "Player not found"})
	// 	} else if r.Context().Err() == context.Canceled {
	// 		return
	// 	} else {
	// 		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": "Database query error: " + err.Error()})
	// 	}
	// 	return
	// }
	// basicLayoutLookupRespond("player", w, r, map[string]any{
	// 	"Player":                       pp,
	// 	"Renames":                      renames,
	// 	"ChartGamesByPlayercount":      ChartGamesByPlayercount.calcTotals(),
	// 	"ChartGamesByBaselevel":        ChartGamesByBaselevel.calcTotals(),
	// 	"ChartGamesByAlliances":        ChartGamesByAlliances.calcTotals(),
	// 	"ChartGamesByScav":             ChartGamesByScav.calcTotals(),
	// 	"RatingHistory":                RatingHistory,
	// 	"ResearchClassificationTotal":  ResearchClassificationTotal,
	// 	"ResearchClassificationRecent": ResearchClassificationRecent,
	// 	"AltCount":                     AltCount,
	// })
}

type eloHist struct {
	Rating int
}

func getRatingHistory(pid int) (map[string]eloHist, error) {
	rows, derr := dbpool.Query(context.Background(),
		`SELECT
			id,
			coalesce(ratingdiff, '{0,0,0,0,0,0,0,0,0,0,0}'),
			to_char(timestarted, 'YYYY-MM-DD HH24:MI'),
			players
		FROM games
		where
			array[$1::int] <@ players
			AND finished = true
			AND calculated = true
			AND hidden = false
			AND deleted = false
		order by timestarted asc`, pid)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, derr
	}
	defer rows.Close()
	h := map[string]eloHist{}
	prevts := ""
	for rows.Next() {
		var gid int
		var rdiff []int
		var timestarted string
		var players []int
		err := rows.Scan(&gid, &rdiff, &timestarted, &players)
		if err != nil {
			return nil, err
		}
		k := -1
		for i, p := range players {
			if p == pid {
				k = i
				break
			}
		}
		if k < 0 || k >= len(rdiff) {
			log.Printf("Game %d is broken (k %d) players %v diffs %v", gid, k, players, rdiff)
			continue
		}
		rDiff := rdiff[k]
		if prevts == "" {
			h[timestarted] = eloHist{
				Rating: 1400 + rDiff,
			}
		} else {
			ph := h[prevts]
			h[timestarted] = eloHist{
				Rating: ph.Rating + rDiff,
			}
		}
		prevts = timestarted
	}
	return h, nil
}

func APIgetElodiffChartPlayer(_ http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	pid, err := strconv.Atoi(params["pid"])
	if err != nil {
		return 400, nil
	}
	h, err := getRatingHistory(pid)
	if err != nil {
		return 500, err
	}
	return 200, h
}
