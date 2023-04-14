package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

func APIcall(c func(http.ResponseWriter, *http.Request) (int, interface{})) func(http.ResponseWriter, *http.Request) {
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
				response, err = json.Marshal(map[string]interface{}{"error": econtent.Error()})
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

func APItryReachMultihoster(w http.ResponseWriter, _ *http.Request) {
	s, m := RequestStatus()
	io.WriteString(w, m)
	if s {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

func APIgetGraphData(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	params := mux.Vars(r)
	gid := params["gid"]
	var j string
	derr := dbpool.QueryRow(r.Context(), `SELECT json_agg(frames)::text FROM frames WHERE game = $1`, gid).Scan(&j)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return 204, nil
		}
		return 500, derr
	}
	return 200, []byte(j)
}

func getDatesGraphData(ctx context.Context, interval string) ([]map[string]int, error) {
	rows, derr := dbpool.Query(ctx, `SELECT date_trunc($1, g.timestarted)::text || '+00', count(g.timestarted)
	FROM games as g
	WHERE g.timestarted > now() - '1 year 7 days'::interval
	GROUP BY date_trunc($1, g.timestarted)
	ORDER BY date_trunc($1, g.timestarted)`, interval)
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

func APIgetDatesGraphData(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	ret, err := getDatesGraphData(r.Context(), mux.Vars(r)["interval"])
	if err != nil {
		return 500, err
	}
	return 200, ret
}

func APIgetDayAverageByHour(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	rows, derr := dbpool.Query(r.Context(), `select count(gg) as c, extract('hour' from timestarted) as d from games as gg group by d order by d`)
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

func APIgetUniquePlayersPerDay(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
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

func APIgetMapNameCount(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	rows, derr := dbpool.Query(r.Context(), `select mapname, count(*) as c from games group by mapname order by c desc limit 30`)
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

func APIgetReplayFile(w http.ResponseWriter, r *http.Request) (int, interface{}) {
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
	}
	dir := "0"
	derr := dbpool.QueryRow(r.Context(), `SELECT coalesce(gamedir) FROM games WHERE id = $1;`, gid).Scan(&dir)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return 204, nil
		}
		return 500, derr
	}
	if dir == "-1" {
		return 204, nil
	}
	replaypath, err := findReplayByConfigFolder(dir)
	if err != nil {
		return 500, err
	}
	if replaypath == "" {
		return 204, nil
	}
	replaycontent, err = os.ReadFile(replaypath)
	if err != nil {
		return 500, err
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\"autohoster-game-"+gids+".wzrp\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(replaycontent)))
	w.Write(replaycontent)
	return -1, nil
}

func APIgetClassChartGame(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	params := mux.Vars(r)
	gid := params["gid"]
	reslog := "0"
	derr := dbpool.QueryRow(r.Context(), `SELECT coalesce(researchlog, '{}') FROM games WHERE id = $1;`, gid).Scan(&reslog)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return 204, nil
		}
		return 500, derr
	}
	if reslog == "-1" {
		return 204, nil
	}
	c, err := LoadClassification()
	if err != nil {
		return 500, err
	}
	var resl []resEntry
	err = json.Unmarshal([]byte(reslog), &resl)
	if err != nil {
		return 500, err
	}
	return 200, CountClassification(c, resl)
}

func APIgetHashInfo(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	params := mux.Vars(r)
	phash := params["hash"]
	var resp []byte
	derr := dbpool.QueryRow(r.Context(),
		`SELECT json_build_object(
			'hash', $1::text,
			'id', players.id,
			'name', players.name,
			'spam', COALESCE((SELECT COUNT(*) FROM games WHERE players.id = ANY(games.players) AND gametime < 30000 AND timestarted+'1 day' > now() AND calculated = true), 0),
			'ispbypass', COALESCE(users.bypass_ispban, false),
			'banned', COALESCE(CASE WHEN bans.duration = 0 THEN true ELSE bans.whenbanned + (bans.duration || ' second')::interval > now() END, false),
			'banreason', bans.reason,
			'bandate', to_char(whenbanned, 'DD Mon YYYY HH12:MI:SS'),
			'banid', 'M-' || bans.id,
			'banexpiery', bans.duration,
			'banexpierystr', CASE WHEN bans.duration = 0 THEN 'never' ELSE to_char(whenbanned + (duration || ' second')::interval, 'DD Mon YYYY HH12:MI:SS') END
		)
		FROM players
		LEFT OUTER JOIN bans ON players.id = bans.playerid
		LEFT OUTER JOIN users ON players.id = users.wzprofile2
		WHERE players.hash = $1::text OR bans.hash = $1::text`, phash).Scan(&resp)
	if derr != nil {
		if errors.Is(derr, pgx.ErrNoRows) {
			return 200, map[string]any{
				"hash":      phash,
				"id":        0,
				"name":      "Noname",
				"spam":      0,
				"ispbypass": false,
				"banned":    false,
			}
		} else {
			return 500, derr
		}
	}
	return 200, resp
}

func APIgetPlayerAllowedJoining(w http.ResponseWriter, r *http.Request) (int, interface{}) {
	params := mux.Vars(r)
	phash := params["hash"]
	badplayed := 0
	derr := dbpool.QueryRow(r.Context(), `SELECT COUNT(id) FROM games WHERE (SELECT id FROM players WHERE hash = $1) = ANY(players) AND gametime < 30000 AND timestarted+'1 day' > now() AND calculated = true;`, phash).Scan(&badplayed)
	if derr != nil {
		return 500, derr
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, fmt.Sprint(badplayed))
	return -1, nil
}

func APIgetPlayerLinked(w http.ResponseWriter, r *http.Request) (int, interface{}) {
	params := mux.Vars(r)
	phash := params["hash"]
	linked := 0
	derr := dbpool.QueryRow(r.Context(), `select count(*) from users where wzprofile2 = (select id from players where hash = $1);`, phash).Scan(&linked)
	if derr != nil {
		return 500, derr
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, fmt.Sprint(linked))
	return -1, nil
}

func APIgetLinkedPlayers(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	hashes := []string{}
	rows, err := dbpool.Query(r.Context(), `select hash from players join users on players.id = users.wzprofile2;`)
	if err != nil {
		return 500, err
	}
	defer rows.Close()
	for rows.Next() {
		h := ""
		err = rows.Scan(&h)
		if err != nil {
			return 500, err
		}
		hashes = append(hashes, h)
	}
	return 200, hashes
}

func APIgetISPbypassHashes(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	hashes := []string{}
	rows, err := dbpool.Query(r.Context(), `select hash from players join users on players.id = users.wzprofile2 where users.bypass_ispban = true;`)
	if err != nil {
		return 500, err
	}
	defer rows.Close()
	for rows.Next() {
		h := ""
		err = rows.Scan(&h)
		if err != nil {
			return 500, err
		}
		hashes = append(hashes, h)
	}
	return 200, hashes
}

func APIgetISPbypassHash(w http.ResponseWriter, r *http.Request) (int, interface{}) {
	params := mux.Vars(r)
	phash := params["hash"]
	linked := 0
	derr := dbpool.QueryRow(r.Context(), `select count(*) from users where wzprofile2 = (select id from players where hash = $1) and bypass_ispban = true;`, phash).Scan(&linked)
	if derr != nil {
		return 500, derr
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, fmt.Sprint(linked))
	return -1, nil
}

func APIgetAllowedModerators(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	rows, derr := dbpool.Query(r.Context(), `select hash from players join users on players.id = users.wzprofile2 where users.allow_preset_request = true;`)
	if derr != nil {
		return 500, derr
	}
	defer rows.Close()
	re := []string{}
	for rows.Next() {
		var h string
		err := rows.Scan(&h)
		if err != nil {
			return 500, err
		}
		re = append(re, h)
	}
	return 200, re
}

type resEntry struct {
	Name     string  `json:"name"`
	Position float64 `json:"position"`
	Time     float64 `json:"time"`
}

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
func CountClassification(c []map[string]string, resl []resEntry) (ret map[int]map[string]int) {
	cl := map[string]string{}
	ret = map[int]map[string]int{}
	for _, b := range c {
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

func APIgetClassChartPlayer(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	params := mux.Vars(r)
	pid, err := strconv.Atoi(params["pid"])
	if err != nil {
		return 400, nil
	}
	filteri, err := strconv.Atoi(params["category"])
	if err != nil {
		return 400, nil
	}
	filter := ""
	filter2 := ""
	if filteri == 1 {
		// filter = " AND array_length(players, 1) = 2 "
	} else if filteri == 2 {
		filter = " AND array_length(players, 1) >= 2 AND alliancetype != 2 "
	} else if filteri == 3 {
		filter2 = " LIMIT 100 "
		// filter = " AND array_length(players, 1) = 2 "
	} else if filteri == 4 {
		filter2 = " LIMIT 100 "
		filter = " AND array_length(players, 1) >= 2 AND alliancetype != 2 "
	}
	rows, derr := dbpool.Query(r.Context(),
		`SELECT coalesce(id, -1), coalesce(researchlog, ''), coalesce(players) 
		FROM games 
		WHERE 
			$1 = any(players) `+filter+`
			AND finished = true 
			AND calculated = true 
			AND hidden = false 
			AND deleted = false 
			AND id > 2000
		ORDER BY id desc
		`+filter2, pid)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return 204, nil
		}
		return 500, derr
	}
	defer rows.Close()
	rowcount := 0
	researches := []string{}
	players := []int{}
	gids := []int{}
	for rows.Next() {
		rowcount++
		var h string
		var p []int
		var gid int
		err := rows.Scan(&gid, &h, &p)
		if err != nil {
			return 500, err
		}
		playerfound := false
		playersreallen := 0
		for i, j := range p {
			if j != -1 {
				playersreallen++
			}
			if j == pid {
				players = append(players, i)
				playerfound = true
				break
			}
		}
		if (filteri == 1 || filteri == 3) && playersreallen > 2 {
			continue
		}
		if !playerfound {
			log.Printf("Can not find player %d in game %d THIS MUST NOT HAPPEN", pid, gid)
			return 500, nil
		}
		researches = append(researches, h)
		gids = append(gids, gid)
	}
	if rowcount == 0 {
		return 204, nil
	}
	classif, err := LoadClassification()
	if err != nil {
		return 500, err
	}
	ret := map[string]int{}
	for i, j := range researches {
		var resl []resEntry
		err = json.Unmarshal([]byte(j), &resl)
		if err != nil {
			log.Print(err.Error())
			log.Printf("Gid: %d", gids[i])
			log.Print(spew.Sdump(j))
			continue
		}
		cl := CountClassification(classif, resl)
		for v, c := range cl[players[i]] {
			if val, ok := ret[v]; ok {
				ret[v] = val + c
			} else {
				ret[v] = c
			}
		}
	}
	return 200, ret
}

func APIgetElodiffChartPlayer(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	params := mux.Vars(r)
	pid, err := strconv.Atoi(params["pid"])
	if err != nil {
		return 400, nil
	}
	rows, derr := dbpool.Query(r.Context(),
		`SELECT
			id,
			coalesce(elodiff, '{0,0,0,0,0,0,0,0,0,0,0}'),
			coalesce(ratingdiff, '{0,0,0,0,0,0,0,0,0,0,0}'),
			to_char(timestarted, 'YYYY-MM-DD HH24:MI'),
			players
		FROM games
		where
			$1 = any(players)
			AND finished = true
			AND calculated = true
			AND hidden = false
			AND deleted = false
		order by timestarted asc`, pid)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return 204, nil
		}
		return 500, derr
	}
	defer rows.Close()
	type eloHist struct {
		Elo    int
		Rating int
	}
	h := map[string]eloHist{}
	prevts := ""
	for rows.Next() {
		var gid int
		var ediff []int
		var rdiff []int
		var timestarted string
		var players []int
		err := rows.Scan(&gid, &ediff, &rdiff, &timestarted, &players)
		if err != nil {
			return 500, err
		}
		k := -1
		for i, p := range players {
			if p == pid {
				k = i
				break
			}
		}
		if k < 0 || k >= len(ediff) || k >= len(rdiff) {
			log.Printf("Game %d is broken (k %d) players %v diffs %v %v", gid, k, players, ediff, rdiff)
			continue
		}
		eDiff := ediff[k]
		rDiff := rdiff[k]
		if prevts == "" {
			h[timestarted] = eloHist{
				Elo:    1400 + eDiff,
				Rating: 1400 + rDiff,
			}
		} else {
			ph := h[prevts]
			h[timestarted] = eloHist{
				Elo:    ph.Elo + eDiff,
				Rating: ph.Rating + rDiff,
			}
		}
		prevts = timestarted
	}
	return 200, h
}

func APIgetUsers(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	if !isSuperadmin(r.Context(), sessionGetUsername(r)) {
		return 403, nil
	}
	var ret []byte
	derr := dbpool.QueryRow(r.Context(), `
		SELECT array_to_json(array_agg(row_to_json(t)))
		FROM (
			SELECT id, username, email, last_seen, email_confirmed, wzprofile2, account_created,
				allow_host_request, allow_preset_request, last_host_request, norequest_reason,
				allow_profile_merge, terminated, bypass_ispban FROM users
		) as t`).Scan(&ret)
	if derr != nil {
		return 500, derr
	}
	return 200, ret
}

func APIgetLeaderboard(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	// dbOrder := parseQueryStringFiltered(r, "order", "desc", "asc")
	// dbLimit := parseQueryInt(r, "limit", 5)
	// dbOffset := parseQueryInt(r, "offset", 0)
	// dbOrderBy := parseQueryStringMapped(r, "sort", "elo", map[string]string{
	// 	"Elo2":       "elo2",
	// 	"Autoplayed": "autoplayed",
	// 	"Autowon":    "autowon",
	// 	"Autolost":   "autolost",
	// 	"Name":       "name",
	// 	"ID":         "id",
	// })
	filterOnlyRegistered := ""
	if parseQueryStringFiltered(r, "OnlyRegistered", "true", "false") == "true" {
		filterOnlyRegistered = "AND users.id != -1"
	}
	rows, derr := dbpool.Query(r.Context(), `
	SELECT players.id, name, hash, elo, elo2, autoplayed, autolost, autowon, coalesce(users.id, -1) as userid, timeplayed
	FROM players
	FULL OUTER JOIN users on players.id = users.wzprofile2
	WHERE autoplayed > 0 `+filterOnlyRegistered)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return 204, nil
		}
		return 500, derr
	}
	defer rows.Close()
	var P []PlayerLeaderboard
	for rows.Next() {
		var pp PlayerLeaderboard
		rows.Scan(&pp.ID, &pp.Name, &pp.Hash, &pp.Elo, &pp.Elo2, &pp.Autoplayed, &pp.Autolost, &pp.Autowon, &pp.Userid, &pp.Timeplayed)
		P = append(P, pp)
	}
	return 200, P
}

func APIgetGames(_ http.ResponseWriter, r *http.Request) (int, interface{}) {
	reqLimit := parseQueryInt(r, "limit", 50)
	if reqLimit > 200 {
		reqLimit = 200
	}
	if reqLimit <= 0 {
		reqLimit = 1
	}
	reqOffset := parseQueryInt(r, "offset", 0)
	if reqOffset < 0 {
		reqOffset = 0
	}
	reqSortOrder := parseQueryStringFiltered(r, "order", "desc", "asc")
	fieldmappings := map[string]string{
		"TimeStarted": "timestarted",
		"TimeEnded":   "timeended",
		"ID":          "id",
		"MapName":     "mapname",
		"GameTime":    "gametime",
	}
	reqSortField := parseQueryStringMapped(r, "sort", "timestarted", fieldmappings)

	reqFilterJ := parseQueryString(r, "filter", "")
	reqFilterFields := map[string]string{}
	reqDoFilters := false
	if reqFilterJ != "" {
		err := json.Unmarshal([]byte(reqFilterJ), &reqFilterFields)
		if err == nil && len(reqFilterFields) > 0 {
			reqDoFilters = true
		}
	}

	wherecase := "WHERE deleted = false AND hidden = false"
	if sessionGetUsername(r) == "Flex seal" {
		wherecase = ""
	}
	pid := parseQueryInt(r, "player", -1)
	if pid > 0 {
		if wherecase == "" {
			wherecase = fmt.Sprintf("WHERE %d = p.id", pid)
		} else {
			wherecase += fmt.Sprintf(" AND %d = p.id", pid)
		}
	}
	whereargs := []interface{}{}
	if reqDoFilters {
		val, ok := reqFilterFields["MapName"]
		if ok {
			whereargs = append(whereargs, val)
			if wherecase == "" {
				wherecase = "WHERE g.mapname = $1"
			} else {
				wherecase += " AND g.mapname = $1"
			}
		}
	}

	reqSearch := parseQueryString(r, "search", "")

	similarity := 0.3

	if reqSearch != "" {
		whereargs = append(whereargs, reqSearch)
		if wherecase == "" {
			wherecase = fmt.Sprintf("WHERE similarity(p.name, $1::text) > %f", similarity)
		} else {
			wherecase += fmt.Sprintf(" AND similarity(p.name, $%d::text) > %f", len(whereargs), similarity)
		}
	}

	ordercase := fmt.Sprintf("ORDER BY %s %s", reqSortField, reqSortOrder)
	limiter := fmt.Sprintf("LIMIT %d", reqLimit)
	offset := fmt.Sprintf("OFFSET %d", reqOffset)
	joincase := "JOIN players AS p ON p.id = any(g.players)"

	totalsc := make(chan int)
	var totals int
	totalspresent := false

	totalsNoFilterc := make(chan int)
	var totalsNoFilter int
	totalsNoFilterpresent := false

	growsc := make(chan []DbGamePreview)
	var gms []DbGamePreview
	gpresent := false

	pmapc := make(chan map[int]DbGamePlayerPreview)
	var pmap map[int]DbGamePlayerPreview
	ppresent := false

	echan := make(chan error)
	go func() {
		var c int
		derr := dbpool.QueryRow(r.Context(), `select count(games) from games where hidden = false and deleted = false;`).Scan(&c)
		if derr != nil {
			echan <- derr
			return
		}
		totalsNoFilterc <- c
	}()
	go func() {
		var c int
		req := `select count(distinct g.id) from games as g ` + joincase + ` ` + wherecase + `;`
		derr := dbpool.QueryRow(r.Context(), req, whereargs...).Scan(&c)
		// log.Println(req)
		if derr != nil {
			echan <- derr
			return
		}
		totalsc <- c
	}()
	go func() {
		req := `SELECT
			g.id, g.finished, to_char(g.timestarted, 'YYYY-MM-DD HH24:MI'), coalesce(to_char(g.timeended, 'YYYY-MM-DD HH24:MI'), '==='), g.gametime,
			g.players, g.teams, g.colour, g.usertype,
			g.mapname, g.maphash,
			g.baselevel, g.powerlevel, g.scavs, g.alliancetype,
			coalesce(g.elodiff, '{0,0,0,0,0,0,0,0,0,0,0}'), coalesce(g.ratingdiff, '{0,0,0,0,0,0,0,0,0,0,0}'),
			g.hidden, g.calculated, g.debugtriggered, coalesce(g.version, '???'), g.mod
		FROM games as g ` + joincase + ` ` + wherecase + ` GROUP BY g.id ` + ordercase + ` ` + offset + ` ` + limiter + ` ;`
		// log.Println(req)
		rows, derr := dbpool.Query(r.Context(), req, whereargs...)
		if derr != nil {
			echan <- derr
			return
		}
		defer rows.Close()
		gmsStage := []DbGamePreview{}
		for rows.Next() {
			g := DbGamePreview{}
			var splayers []int
			var steams []int
			var scolour []int
			var susertype []string
			var selodiff []int
			var sratingdiff []int
			err := rows.Scan(&g.ID, &g.Finished, &g.TimeStarted, &g.TimeEnded, &g.GameTime,
				&splayers, &steams, &scolour, &susertype,
				&g.MapName, &g.MapHash,
				&g.BaseLevel, &g.PowerLevel, &g.Scavengers, &g.Alliances,
				&selodiff, &sratingdiff, &g.Hidden, &g.Calculated, &g.DebugTriggered, &g.GameVersion, &g.Mod)
			if err != nil {
				echan <- err
				return
			}
			for i, p := range splayers {
				if p == -1 {
					continue
				}
				// log.Printf("Filling player %v", i)
				g.Players[i].Position = i
				g.Players[i].ID = p
				g.Players[i].Team = steams[i]
				g.Players[i].Colour = scolour[i]
				if len(susertype) > i {
					g.Players[i].Usertype = susertype[i]
				}
				if len(selodiff) > i {
					g.Players[i].EloDiff = selodiff[i]
				}
				if len(sratingdiff) > i {
					g.Players[i].RatingDiff = sratingdiff[i]
				}
			}
			gmsStage = append(gmsStage, g)
		}
		growsc <- gmsStage
	}()
	go func() {
		req := `SELECT
			p.id, p.name, p.hash, p.elo, p.elo2, p.autoplayed, p.autowon, p.autolost, coalesce(u.id, -1)
		FROM players as p
		LEFT JOIN users as u ON u.wzprofile2 = p.id
		WHERE p.id = any((select distinct unnest(a.players)
				FROM (SELECT players FROM games as g ` + joincase + ` ` + wherecase + `  GROUP BY g.id ` + ordercase + ` ` + offset + ` ` + limiter + `) as a));`
		// log.Println(req)
		rows, derr := dbpool.Query(r.Context(), req, whereargs...)
		if derr != nil {
			echan <- derr
			return
		}
		defer rows.Close()
		pmapStage := map[int]DbGamePlayerPreview{}
		for rows.Next() {
			p := DbGamePlayerPreview{}
			err := rows.Scan(&p.ID, &p.Name, &p.Hash, &p.Elo, &p.Elo2, &p.Autoplayed, &p.Autowon, &p.Autolost, &p.Userid)
			if err != nil {
				echan <- err
				return
			}
			pmapStage[p.ID] = p
		}
		pmapc <- pmapStage
	}()
	for !(gpresent && ppresent && totalspresent && totalsNoFilterpresent) {
		select {
		case derr := <-echan:
			if derr == pgx.ErrNoRows {
				return 200, []byte(`{"total": 0, "totalNotFiltered": 0, "rows": []}`)
			}
			return 500, derr
		case gms = <-growsc:
			gpresent = true
		case pmap = <-pmapc:
			ppresent = true
		case totals = <-totalsc:
			totalspresent = true
		case totalsNoFilter = <-totalsNoFilterc:
			totalsNoFilterpresent = true
		}
	}
	for i := range gms {
		for j := range gms[i].Players {
			if gms[i].Players[j].ID <= 0 {
				continue
			}
			p, ok := pmap[gms[i].Players[j].ID]
			if !ok {
				log.Printf("Game %v has unknown player %v (%v)", gms[i].ID, gms[i].Players[j].ID, gms[i].Players)
				continue
			}
			gms[i].Players[j].Name = p.Name
			gms[i].Players[j].Hash = p.Hash
			gms[i].Players[j].Elo = p.Elo
			gms[i].Players[j].Elo2 = p.Elo2
			gms[i].Players[j].Autoplayed = p.Autoplayed
			gms[i].Players[j].Autolost = p.Autolost
			gms[i].Players[j].Autowon = p.Autowon
			gms[i].Players[j].Userid = p.Userid
		}
	}
	return 200, map[string]interface{}{
		"total":            totals,
		"totalNotFiltered": totalsNoFilter,
		"rows":             gms,
	}
}
