package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strconv"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
	db "github.com/warzone2100/autohoster-db"
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

func APItryReachMultihoster(w http.ResponseWriter, _ *http.Request) {
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
	gid := params["gid"]
	var j string
	derr := dbpool.QueryRow(r.Context(), `SELECT coalesce(graphs, 'null') FROM games WHERE id = $1;`, gid).Scan(&j)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return 204, nil
		}
		return 500, derr
	}
	if j == "null" {
		return 204, nil
	}
	return 200, []byte(j)
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

func APIgetUniquePlayersPerDay(_ http.ResponseWriter, r *http.Request) (int, any) {
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
	var resl []resEntry
	err := json.Unmarshal([]byte(reslog), &resl)
	if err != nil {
		return 500, err
	}
	return 200, CountClassification(resl)
}

func APIgetHashInfo(_ http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	phash := params["hash"]
	var resp []byte
	derr := dbpool.QueryRow(r.Context(),
		`SELECT json_build_object(
			'hash', $1::text,
			'id', players.id,
			'name', players.name,
			'spam', COALESCE((SELECT COUNT(*) FROM games WHERE players.id = ANY(games.players) AND gametime < 30000 AND timestarted+'1 day' > now() AND calculated = true), 0),
			'ispbypass', COALESCE(accounts.bypass_ispban, false),
			'userid', COALESCE(accounts.id, -1),
			'banned', COALESCE(CASE WHEN bans.duration = 0 THEN true ELSE bans.whenbanned + (bans.duration || ' second')::interval > now() END, false),
			'banreason', bans.reason,
			'bandate', to_char(whenbanned, 'DD Mon YYYY HH12:MI:SS'),
			'banid', 'M-' || bans.id,
			'banexpiery', bans.duration,
			'banexpierystr', CASE WHEN bans.duration = 0 THEN 'never' ELSE to_char(whenbanned + (duration || ' second')::interval, 'DD Mon YYYY HH12:MI:SS') END
		)
		FROM players
		LEFT OUTER JOIN bans ON players.id = bans.playerid
		LEFT OUTER JOIN accounts ON players.id = accounts.wzprofile2
		WHERE players.hash = $1::text OR bans.hash = $1::text
		ORDER BY bans.id DESC`, phash).Scan(&resp)
	if derr != nil {
		if errors.Is(derr, pgx.ErrNoRows) {
			return 200, map[string]any{
				"hash":      phash,
				"id":        0,
				"name":      "Noname",
				"spam":      0,
				"ispbypass": false,
				"userid":    -1,
				"banned":    false,
			}
		} else {
			return 500, derr
		}
	}
	return 200, resp
}

func APIgetPlayerAllowedJoining(w http.ResponseWriter, r *http.Request) (int, any) {
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

func APIgetPlayerLinked(w http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	phash := params["hash"]
	linked := 0
	derr := dbpool.QueryRow(r.Context(), `select count(*) from accounts where wzprofile2 = (select id from players where hash = $1);`, phash).Scan(&linked)
	if derr != nil {
		return 500, derr
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, fmt.Sprint(linked))
	return -1, nil
}

func APIgetLinkedPlayers(_ http.ResponseWriter, r *http.Request) (int, any) {
	hashes := []string{}
	rows, err := dbpool.Query(r.Context(), `select hash from players join accounts on players.id = accounts.wzprofile2;`)
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

func APIgetISPbypassHashes(_ http.ResponseWriter, r *http.Request) (int, any) {
	hashes := []string{}
	rows, err := dbpool.Query(r.Context(), `select hash from players join accounts on players.id = accounts.wzprofile2 where accounts.bypass_ispban = true;`)
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

func APIgetISPbypassHash(w http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	phash := params["hash"]
	linked := 0
	derr := dbpool.QueryRow(r.Context(), `select count(*) from accounts where wzprofile2 = (select id from players where hash = $1) and bypass_ispban = true;`, phash).Scan(&linked)
	if derr != nil {
		return 500, derr
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, fmt.Sprint(linked))
	return -1, nil
}

func APIgetAllowedModerators(_ http.ResponseWriter, r *http.Request) (int, any) {
	rows, derr := dbpool.Query(r.Context(), `select hash from players join accounts on players.id = accounts.wzprofile2 where accounts.allow_preset_request = true;`)
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

func APIgetIdentities(_ http.ResponseWriter, r *http.Request) (int, any) {
	ret, err := db.GetIdentities(r.Context(), dbpool)
	if err != nil {
		return 500, err
	}
	return 200, ret
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
	ret, err := db.GetAccounts(r.Context(), dbpool)
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

func APIgetGames(_ http.ResponseWriter, r *http.Request) (int, any) {
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
		"TimeStarted": "time_started",
		"TimeEnded":   "time_ended",
		"ID":          "id",
		"MapName":     "map_name",
		"GameTime":    "game_time",
	}
	reqSortField := parseQueryStringMapped(r, "sort", "time_started", fieldmappings)

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
	whereargs := []any{}
	if reqDoFilters {
		val, ok := reqFilterFields["MapName"]
		if ok {
			whereargs = append(whereargs, val)
			if wherecase == "" {
				wherecase = "WHERE g.map_name = $1"
			} else {
				wherecase += " AND g.map_name = $1"
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
	joincase := ""

	totalsc := make(chan int)
	var totals int
	totalspresent := false

	totalsNoFilterc := make(chan int)
	var totalsNoFilter int
	totalsNoFilterpresent := false

	growsc := make(chan []*Game)
	var gms []*Game
	gpresent := false

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
		req := `select
		g.id, g.version, g.time_started, g.time_ended, g.game_time,
		g.setting_scavs, g.setting_alliance, g.setting_power, g.setting_base,
		map_name, g.map_hash, g.mods, g.deleted, g.hidden, g.calculated, g.debug_triggered,
		json_agg(json_build_object(
			'Position', players.position,
			'Name', identities.name,
			'Team', players.team,
			'Usertype', players.usertype,
			'Color', players.color,
			'Account', coalesce(accounts.id, 0),
			'Elo', coalesce(rating.elo, 0),
			'Played', coalesce(rating.played, 0),
			'Won', coalesce(rating.won, 0),
			'Lost', coalesce(rating.lost, 0)
		)) as players
	from games as g
	join players on game = g.id
	join identities on identity = identities.id
	left join accounts on identities.account = accounts.id
	full outer join rating on identities.account = rating.account
	where rating.category = g.display_category
	group by g.id
	` + ordercase + `
	` + limiter + `
	` + offset
		var gmsStage []*Game
		err := pgxscan.Select(r.Context(), dbpool, &gmsStage, req)
		if err != nil {
			echan <- err
			return
		}
		for _, v := range gmsStage {
			if v == nil {
				continue
			}
			slices.SortFunc(v.Players, func(a Player, b Player) int {
				return a.Position - b.Position
			})
		}
		growsc <- gmsStage
	}()
	for !(gpresent && totalspresent && totalsNoFilterpresent) {
		select {
		case derr := <-echan:
			if derr == pgx.ErrNoRows {
				return 200, []byte(`{"total": 0, "totalNotFiltered": 0, "rows": []}`)
			}
			return 500, derr
		case gms = <-growsc:
			gpresent = true
		case totals = <-totalsc:
			totalspresent = true
		case totalsNoFilter = <-totalsNoFilterc:
			totalsNoFilterpresent = true
		}
	}
	return 200, map[string]any{
		"total":            totals,
		"totalNotFiltered": totalsNoFilter,
		"rows":             gms,
	}
}
