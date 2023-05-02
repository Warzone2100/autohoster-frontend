package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jackc/pgx/v4"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	type article struct {
		Title   string    `json:"title"`
		Content string    `json:"content"`
		Time    time.Time `json:"posttime"`
		Color   string    `json:"color"`
	}
	timeInterval := "48 hours"
	news := []article{}
	gamesPlayed := 0
	gamesPlayedMasterbal := 0
	gamesPlayedTiny := 0
	uniqPlayers := 0
	gameTime := 0
	unitsProduced := 0
	structsBuilt := 0
	eloTransferred := 0
	ratingTransferred := 0
	gamesGraph := map[string]int{}
	gamesGraphAvg := map[string]int{}
	ratingGamesGraph := map[string]int{}
	ratingGamesGraphAvg := map[string]int{}
	err := RequestMultiple(func() error {
		rows, err := dbpool.Query(r.Context(), `select title, content, posttime, color from news order by posttime desc limit 25;`)
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
		return dbpool.QueryRow(r.Context(), `select count(*) from games where timestarted + $1::interval > now() and mod != 'masterbal'`, timeInterval).Scan(&gamesPlayed)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select count(*) from games where timestarted + $1::interval > now() and mod = 'masterbal'`, timeInterval).Scan(&gamesPlayedMasterbal)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select count(*) from games where timestarted + $1::interval > now() and mapname = 'Tiny_VautEdition'`, timeInterval).Scan(&gamesPlayedTiny)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select count(distinct p) from games, unnest(players) as p where timestarted + $1::interval > now()`, timeInterval).Scan(&uniqPlayers)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select sum(gametime) from games where timestarted + $1::interval > now()`, timeInterval).Scan(&gameTime)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select sum(k) from games, unnest(unitbuilt) as k where timestarted + $1::interval > now() and k > 0`, timeInterval).Scan(&unitsProduced)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select sum(k) from games, unnest(structbuilt) as k where timestarted + $1::interval > now() and k > 0`, timeInterval).Scan(&structsBuilt)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select sum(coalesce(abs(elodiff[1]), 0)) from games where timestarted + $1::interval > now();`, timeInterval).Scan(&eloTransferred)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select sum(coalesce(abs(ratingdiff[1]), 0)) from games where timestarted + $1::interval > now();`, timeInterval).Scan(&ratingTransferred)
	}, func() error {
		var date string
		var count, average int
		_, err := dbpool.QueryFunc(r.Context(), `SELECT
		to_char(date_trunc('day', g.timestarted), 'YYYY-MM-DD') as d,
		count(g.timestarted) as c,
		round(avg(count(g.timestarted)) over(order by date_trunc('day', g.timestarted) rows between 6 preceding and current row))
	FROM games as g
	WHERE g.timestarted > now() - '1 year 7 days'::interval
	GROUP BY date_trunc('day', g.timestarted)
	ORDER BY date_trunc('day', g.timestarted)`,
			[]interface{}{}, []interface{}{&date, &count, &average}, func(_ pgx.QueryFuncRow) error {
				gamesGraph[date] = count
				gamesGraphAvg[date] = average
				return nil
			})
		return err
	}, func() error {
		var date string
		var count, average int
		_, err := dbpool.QueryFunc(r.Context(), `SELECT
		to_char(date_trunc('day', g.timestarted), 'YYYY-MM-DD') as d, 
		count(g.timestarted) as c, 
		round(avg(count(g.timestarted)) over(order by date_trunc('day', g.timestarted) rows between 6 preceding and current row))
	FROM games as g
	WHERE g.timestarted > now() - '1 year 7 days'::interval and
		not g.ratingdiff @> ARRAY[0]
	GROUP BY date_trunc('day', g.timestarted)
	ORDER BY date_trunc('day', g.timestarted)`,
			[]interface{}{}, []interface{}{&date, &count, &average}, func(_ pgx.QueryFuncRow) error {
				ratingGamesGraph[date] = count
				ratingGamesGraphAvg[date] = average
				return nil
			})
		return err
	})

	if err != nil {
		log.Println(err)
	}
	basicLayoutLookupRespond("index", w, r, map[string]interface{}{
		"News":                news,
		"LastGames":           gamesPlayed,
		"LastGamesMasterbal":  gamesPlayedMasterbal,
		"LastGamesTiny":       gamesPlayedTiny,
		"LastGamesTinyPrc":    fmt.Sprintf("(%.1f%%)", float64(gamesPlayedTiny)/float64(gamesPlayed)*float64(100)),
		"LastPlayers":         uniqPlayers,
		"LastGTime":           gameTime,
		"LastProduced":        humanize.Comma(int64(unitsProduced)),
		"LastBuilt":           humanize.Comma(int64(structsBuilt)),
		"LastElo":             eloTransferred,
		"LastRating":          ratingTransferred,
		"GamesGraph":          gamesGraph,
		"GamesGraphAvg":       gamesGraphAvg,
		"RatingGamesGraph":    ratingGamesGraph,
		"RatingGamesGraphAvg": ratingGamesGraphAvg,
	})
}
