package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
	"github.com/maxsupermanhd/go-wz/packet"
	"github.com/maxsupermanhd/go-wz/replay"
	"github.com/maxsupermanhd/go-wz/wznet"
)

func APIcall(c func(http.ResponseWriter, *http.Request) (int, any)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		code, content := c(w, r)
		if code <= 0 {
			return
		}
		var response []byte
		var err error
		if content != nil {
			if bcontent, ok := content.([]byte); ok {
				if json.Valid(bcontent) {
					response = bcontent
				}
			} else if econtent, ok := content.(error); ok {
				log.Printf("Error inside handler [%v]: %v", r.URL.Path, econtent.Error())
				response, err = json.Marshal(map[string]any{"error": econtent.Error()})
				if err != nil {
					code = 500
					response = []byte(`{"error": "Failed to marshal json response"}`)
					log.Println("Failed to marshal json content: ", err.Error())
				}
			} else {
				response, err = json.Marshal(content)
				if err != nil {
					code = 500
					response = []byte(`{"error": "Failed to marshal json response"}`)
					log.Println("Failed to marshal json content: ", err.Error())
				}
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", "https://wz2100-autohost.net https://dev.wz2100-autohost.net")
		if len(response) > 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Content-Length", strconv.Itoa(len(response)+1))
			w.WriteHeader(code)
			w.Write(response)
			w.Write([]byte("\n"))
		} else {
			w.WriteHeader(code)
		}
	}
}

func APItryReachBackend(w http.ResponseWriter, _ *http.Request) {
	s, m := RequestStatus()
	io.WriteString(w, m)
	if s {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

func APIgetGraphData(_ http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	gids := params["gid"]
	gid, err := strconv.Atoi(gids)
	if err != nil {
		return 500, err
	}
	var j string
	err = dbpool.QueryRow(r.Context(), `SELECT coalesce(graphs, 'null') FROM games WHERE id = $1;`, gid).Scan(&j)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 204, nil
		}
		return 500, err
	}
	if j == "null" {
		return 204, nil
	}
	frames := []map[string]any{}
	err = json.Unmarshal([]byte(j), &frames)
	if err != nil {
		return 500, err
	}
	sort.Slice(frames, func(i, j int) bool {
		gti, ok := frames[i]["gameTime"].(float64)
		if !ok {
			return true
		}
		gtj, ok := frames[j]["gameTime"].(float64)
		if !ok {
			return true
		}
		return gti < gtj
	})
	avg := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	avgw := float64(60)

	var rpl *replay.Replay
	replaycontent, err := getReplayFromStorage(gid)
	if err != nil {
		if err != errReplayNotFound {
			return 500, err
		}
	} else {
		rpl, err = replay.ReadReplay(bytes.NewBuffer(replaycontent))
		if err != nil {
			return 500, err
		}
		if rpl == nil {
			return 500, errors.New("replay is nil")
		}
	}
	rplPktIndex := 0

	prevOrderFp := make([]int32, 32)

	calcDroidCs := func(droids []uint32) byte {
		slices.Sort(droids)
		buf := bytes.NewBufferString("")
		binary.Write(buf, binary.NativeEndian, droids)
		return md5.Sum(buf.Bytes())[0]
	}

	for i, v := range frames {
		if rpl != nil {
			rplPktCount := make([]int, rpl.Settings.GameOptions.Game.MaxPlayers)
			gt, ok := v["gameTime"].(float64)
			if ok {
			rplcountloop:
				for ; rplPktIndex < len(rpl.Messages); rplPktIndex++ {
					pos := rpl.Settings.GameOptions.NetplayPlayers[rpl.Messages[rplPktIndex].Player].Position
					switch p := rpl.Messages[rplPktIndex].NetPacket.(type) {
					case packet.PkGameGameTime:
						if p.GameTime >= uint32(gt) {
							break rplcountloop
						}
					case packet.PkGameDroidInfo:
						if p.SecOrder == wznet.DSO_RETURN_TO_LOC {
							continue
						}
						if p.Order == wznet.DORDER_NONE {
							continue
						}
						currOrderFp := (p.CoordX ^ p.CoordY) + int32(calcDroidCs(p.Droids))
						if prevOrderFp[pos] != currOrderFp {
							rplPktCount[pos]++
							prevOrderFp[pos] = currOrderFp
						}
					case packet.PkGameResearchStatus:
						rplPktCount[rpl.Settings.GameOptions.NetplayPlayers[rpl.Messages[rplPktIndex].Player].Position]++
					}
				}
			}
			v["replayPackets"] = rplPktCount
			rplPktSum := make([]int, rpl.Settings.GameOptions.Game.MaxPlayers)
			for i2 := i - 60; i2 != i; i2++ {
				if i2 < 0 {
					continue
				}
				oldPktCount := frames[i2]["replayPackets"].([]int)
				for i3, v3 := range oldPktCount {
					rplPktSum[i3] += v3
				}
			}
			v["replayPacketsP60t"] = rplPktSum
		} else {
			v["replayPackets"] = []int{}
			v["replayPacketsP60t"] = []int{}
		}

		val := []int{}
		v["labActivityP60t"] = val
		if i == 0 {
			continue
		}
		prfs, ok := v["recentResearchPerformance"].([]any)
		if !ok {
			continue
		}
		pots, ok := v["recentResearchPotential"].([]any)
		if !ok {
			continue
		}
		prevPrfs, ok := frames[i-1]["recentResearchPerformance"].([]any)
		if !ok {
			continue
		}
		prevPots, ok := frames[i-1]["recentResearchPotential"].([]any)
		if !ok {
			continue
		}
		for p := 0; p < min(len(prfs), len(pots)); p++ {
			prf, ok := prfs[p].(float64)
			if !ok {
				continue
			}
			pot, ok := pots[p].(float64)
			if !ok {
				continue
			}
			prevPrf, ok := prevPrfs[p].(float64)
			if !ok {
				continue
			}
			prevPot, ok := prevPots[p].(float64)
			if !ok {
				continue
			}
			navg := int(0)
			if pot > 1 && prf > 1 && prevPrf > 1 && prevPot > 1 {
				avg[p] -= avg[p] / avgw
				nval := (prf - prevPrf) / (pot - prevPot)
				if pot == prevPot {
					nval = 0
				}
				avg[p] += (100 * nval) / avgw
				navg = int(avg[p])
			}
			val = append(val, navg)
		}
		v["labActivityP60t"] = val
	}

	return 200, frames
}

func getDatesGraphData(ctx context.Context, interval string) ([]map[string]int, error) {
	rows, derr := dbpool.Query(ctx, `SELECT date_trunc($1, g.time_started)::text || '+00', count(g.time_started)
	FROM games as g
	WHERE g.time_started > now() - '1 year 7 days'::interval
	GROUP BY date_trunc($1, g.time_started)
	ORDER BY date_trunc($1, g.time_started)`, interval)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return []map[string]int{}, nil
		}
		return []map[string]int{}, derr
	}
	defer rows.Close()
	ret := []map[string]int{}
	for rows.Next() {
		var d string
		var c int
		err := rows.Scan(&d, &c)
		if err != nil {
			return []map[string]int{}, err
		}
		ret = append(ret, map[string]int{d: c})
	}
	return ret, nil
}

func APIgetDatesGraphData(_ http.ResponseWriter, r *http.Request) (int, any) {
	ret, err := getDatesGraphData(r.Context(), mux.Vars(r)["interval"])
	if err != nil {
		return 500, err
	}
	return 200, ret
}

func APIgetDayAverageByHour(_ http.ResponseWriter, r *http.Request) (int, any) {
	rows, derr := dbpool.Query(r.Context(), `select count(gg) as c, extract('hour' from time_started) as d from games as gg group by d order by d`)
	if derr != nil {
		return 500, derr
	}
	defer rows.Close()
	re := make(map[int]int)
	for rows.Next() {
		var d, c int
		err := rows.Scan(&c, &d)
		if err != nil {
			return 500, err
		}
		re[d] = c
	}
	return 200, re
}

func APIgetUniquePlayersPerDay(_ http.ResponseWriter, r *http.Request) (int, any) {
	return http.StatusNotImplemented, nil
	rows, derr := dbpool.Query(r.Context(),
		`SELECT d::text, count(r.p)
		FROM (SELECT distinct unnest(gg.players) as p, date_trunc('day', gg.timestarted) AS d FROM games AS gg) as r
		WHERE d > now() - '1 year 7 days'::interval
		GROUP BY d
		ORDER BY d DESC`)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return 204, nil
		}
		return 500, derr
	}
	defer rows.Close()
	re := make(map[string]int)
	for rows.Next() {
		var d string
		var c int
		err := rows.Scan(&d, &c)
		if err != nil {
			return 500, err
		}
		re[d] = c
	}
	return 200, re
}

func APIgetMapNameCount(_ http.ResponseWriter, r *http.Request) (int, any) {
	rows, derr := dbpool.Query(r.Context(), `select map_name, count(*) as c from games group by map_name order by c desc limit 30`)
	if derr != nil {
		return 500, derr
	}
	defer rows.Close()
	re := make(map[string]int)
	for rows.Next() {
		var c int
		var n string
		err := rows.Scan(&n, &c)
		if err != nil {
			return 500, derr
		}
		re[n] = c
	}
	return 200, re
}

func APIgetReplayFile(w http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	gids := params["gid"]
	gid, err := strconv.Atoi(gids)
	if err != nil {
		return 400, nil
	}
	replaycontent, err := getReplayFromStorage(gid)
	if err == nil {
		log.Println("Serving replay from replay storage, gid ", gids)
		w.Header().Set("Content-Disposition", "attachment; filename=\"autohoster-game-"+gids+".wzrp\"")
		w.Header().Set("Content-Length", strconv.Itoa(len(replaycontent)))
		w.Write(replaycontent)
		return -1, nil
	} else if err != errReplayNotFound {
		log.Printf("ERROR getting replay from storage: %v game id is %d", err, gid)
		return 500, err
	}
	return 204, nil
}

func APIgetClassChartGame(_ http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	gid := params["gid"]
	reslog := "0"
	derr := dbpool.QueryRow(r.Context(), `SELECT coalesce(research_log, '{}') FROM games WHERE id = $1;`, gid).Scan(&reslog)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return 204, nil
		}
		return 500, derr
	}
	if reslog == "-1" {
		return 204, nil
	}
	var resl []resEntry
	err := json.Unmarshal([]byte(reslog), &resl)
	if err != nil {
		return 500, err
	}
	return 200, CountClassification(resl)
}

func APIgetRatingCategories(_ http.ResponseWriter, r *http.Request) (int, any) {
	var ret []byte
	err := dbpool.QueryRow(r.Context(), `select json_agg(rating_categories) from rating_categories`).Scan(&ret)
	if err != nil {
		return 500, err
	}
	return 200, ret
}

func APIgetAccounts(_ http.ResponseWriter, r *http.Request) (int, any) {
	type requestRow struct {
		ID               int
		Username         string
		Email            string
		AccountCreated   time.Time
		LastSeen         *time.Time
		Terminated       bool
		EmailConfirmed   *time.Time
		AllowHostRequest bool
		DisplayName      *string
		LastReport       time.Time
		LastRequest      time.Time
	}
	ret := []requestRow{}
	scanRow := requestRow{}
	_, err := dbpool.QueryFunc(r.Context(), `select id, username, email, account_created, last_seen, terminated, email_confirmed, allow_host_request, display_name, last_report, last_request from accounts order by id desc`, []any{},
		[]any{&scanRow.ID, &scanRow.Username, &scanRow.Email, &scanRow.AccountCreated, &scanRow.LastSeen, &scanRow.Terminated, &scanRow.EmailConfirmed, &scanRow.AllowHostRequest, &scanRow.DisplayName, &scanRow.LastReport, &scanRow.LastRequest}, func(qfr pgx.QueryFuncRow) error {
			ret = append(ret, scanRow)
			return nil
		})
	if err != nil {
		return 500, err
	}
	return 200, ret
}

func APIresendEmailConfirm(_ http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		return 400, nil
	}
	modSendWebhook(fmt.Sprintf("Administrator `%s` resent activation email for account `%v`", sessionGetUsername(r), id))
	return 200, modResendEmailConfirm(id)
}
