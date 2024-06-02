package main

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v4"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	type article struct {
		Title   string        `json:"title"`
		Content template.HTML `json:"content"`
		Time    time.Time     `json:"when_posted"`
		Color   string        `json:"color"`
	}
	timeInterval := "48 hours"
	news := []article{}
	gamesPlayed := 0
	gamesPlayedTiny := 0
	// uniqPlayers := 0
	gameTime := 0
	// unitsProduced := 0
	// structsBuilt := 0
	// eloTransferred := 0
	// ratingTransferred := 0
	gamesGraph := map[string]int{}
	gamesGraphAvg := map[string]int{}
	ratingGamesGraph := map[string]int{}
	ratingGamesGraphAvg := map[string]int{}
	err := RequestMultiple(func() error {
		rows, err := dbpool.Query(r.Context(), `select title, content, when_posted, color from news order by when_posted desc limit 25;`)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				news = []article{{
					Title:   "No news yet",
					Content: "I am lazy",
					Time:    time.Now(),
					Color:   "success",
				}}
			} else {
				return err
			}
		} else {
			for rows.Next() {
				var a article
				err = rows.Scan(&a.Title, &a.Content, &a.Time, &a.Color)
				if err != nil {
					return err
				}
				news = append(news, a)
			}
		}
		return nil
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select count(*) from games where time_started + $1::interval > now() and mods != 'masterbal'`, timeInterval).Scan(&gamesPlayed)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select count(*) from games where time_started + $1::interval > now() and map_name = 'Tiny_VautEdition'`, timeInterval).Scan(&gamesPlayedTiny)
		// }, func() error {
		// 	return dbpool.QueryRow(r.Context(), `select count(distinct p) from games, unnest(players) as p where time_started + $1::interval > now()`, timeInterval).Scan(&uniqPlayers)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select coalesce(sum(game_time), 0) from games where time_started + $1::interval > now()`, timeInterval).Scan(&gameTime)
		// }, func() error { // TODO: migrate
		// 	return dbpool.QueryRow(r.Context(), `select coalesce(sum(k), 0) from games, unnest(unitbuilt) as k where time_started + $1::interval > now() and k > 0`, timeInterval).Scan(&unitsProduced)
		// }, func() error { // TODO: migrate
		// 	return dbpool.QueryRow(r.Context(), `select coalesce(sum(k), 0) from games, unnest(structbuilt) as k where time_started + $1::interval > now() and k > 0`, timeInterval).Scan(&structsBuilt)
		// }, func() error { // TODO: migrate
		// 	return dbpool.QueryRow(r.Context(), `select coalesce(sum(coalesce(abs(elodiff[1]), 0)), 0) from games where time_started + $1::interval > now();`, timeInterval).Scan(&eloTransferred)
		// }, func() error { // TODO: migrate
		// 	return dbpool.QueryRow(r.Context(), `select coalesce(sum(coalesce(abs(ratingdiff[1]), 0)), 0) from games where time_started + $1::interval > now();`, timeInterval).Scan(&ratingTransferred)
	}, func() error {
		var date string
		var count, average int
		_, err := dbpool.QueryFunc(r.Context(), `SELECT
		to_char(date_trunc('day', g.time_started), 'YYYY-MM-DD') as d,
		count(g.time_started) as c,
		round(avg(count(g.time_started)) over(order by date_trunc('day', g.time_started) rows between 6 preceding and current row))
	FROM games as g
	WHERE g.time_started > now() - '1 year 7 days'::interval
	GROUP BY date_trunc('day', g.time_started)
	ORDER BY date_trunc('day', g.time_started)`,
			[]any{}, []any{&date, &count, &average}, func(_ pgx.QueryFuncRow) error {
				gamesGraph[date] = count
				gamesGraphAvg[date] = average
				return nil
			})
		return err
	}, func() error {
		var date string
		var count, average int
		_, err := dbpool.QueryFunc(r.Context(), `SELECT
		to_char(date_trunc('day', g.time_started), 'YYYY-MM-DD') as d, 
		count(g.time_started) as c, 
		round(avg(count(g.time_started)) over(order by date_trunc('day', g.time_started) rows between 6 preceding and current row))
	FROM games as g
	WHERE g.time_started > now() - '1 year 7 days'::interval
	GROUP BY date_trunc('day', g.time_started)
	ORDER BY date_trunc('day', g.time_started)`,
			[]any{}, []any{&date, &count, &average}, func(_ pgx.QueryFuncRow) error {
				ratingGamesGraph[date] = count
				ratingGamesGraphAvg[date] = average
				return nil
			})
		return err
	})

	if err != nil {
		log.Println(err)
	}
	basicLayoutLookupRespond("index", w, r, map[string]any{
		"News":             news,
		"LastGames":        gamesPlayed,
		"LastGamesTiny":    gamesPlayedTiny,
		"LastGamesTinyPrc": fmt.Sprintf("(%.1f%%)", float64(gamesPlayedTiny)/float64(gamesPlayed)*float64(100)),
		// "LastPlayers":      uniqPlayers, // TODO: migrate
		"LastGTime": gameTime,
		// "LastProduced":        humanize.Comma(int64(unitsProduced)), // TODO: migrate
		// "LastBuilt":           humanize.Comma(int64(structsBuilt)),  // TODO: migrate
		// "LastElo":             eloTransferred, // TODO: migrate
		// "LastRating":          ratingTransferred, // TODO: migrate
		"GamesGraph":          gamesGraph,
		"GamesGraphAvg":       gamesGraphAvg,
		"RatingGamesGraph":    ratingGamesGraph,    // TODO: migrate
		"RatingGamesGraphAvg": ratingGamesGraphAvg, // TODO: migrate
	})
}
