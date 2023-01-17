package main

import (
	"encoding/json"
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
	gamesGraph := []byte("[]")
	err := RequestMultiple(func(ech chan<- error) {
		rows, err := dbpool.Query(r.Context(), `select title, content, posttime, color from news order by posttime desc limit 25;`)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				news = []article{
					{
						Title:   "No news yet",
						Content: "I am lazy",
						Time:    time.Now(),
						Color:   "success",
					},
				}
			} else {
				news = []article{
					{
						Title:   "Error loading news",
						Content: err.Error(),
						Time:    time.Now(),
						Color:   "danger",
					},
				}
			}
		} else {
			for rows.Next() {
				var a article
				err = rows.Scan(&a.Title, &a.Content, &a.Time, &a.Color)
				if err != nil {
					news = []article{
						{
							Title:   "Error loading news",
							Content: err.Error(),
							Time:    time.Now(),
							Color:   "danger",
						},
					}
					break
				}
				news = append(news, a)
			}
		}
	}, func(ech chan<- error) {
		err := dbpool.QueryRow(r.Context(), `select count(*) from games where timestarted + $1::interval > now() and mod != 'masterbal'`, timeInterval).Scan(&gamesPlayed)
		if err != nil {
			ech <- err
		}
	}, func(ech chan<- error) {
		err := dbpool.QueryRow(r.Context(), `select count(*) from games where timestarted + $1::interval > now() and mod = 'masterbal'`, timeInterval).Scan(&gamesPlayedMasterbal)
		if err != nil {
			ech <- err
		}
	}, func(ech chan<- error) {
		err := dbpool.QueryRow(r.Context(), `select count(*) from games where timestarted + $1::interval > now() and mapname = 'Tiny_VautEdition'`, timeInterval).Scan(&gamesPlayedTiny)
		if err != nil {
			ech <- err
		}
	}, func(ech chan<- error) {
		err := dbpool.QueryRow(r.Context(), `select count(distinct p) from games, unnest(players) as p where timestarted + $1::interval > now()`, timeInterval).Scan(&uniqPlayers)
		if err != nil {
			ech <- err
		}
	}, func(ech chan<- error) {
		err := dbpool.QueryRow(r.Context(), `select sum(gametime) from games where timestarted + $1::interval > now()`, timeInterval).Scan(&gameTime)
		if err != nil {
			ech <- err
		}
	}, func(ech chan<- error) {
		err := dbpool.QueryRow(r.Context(), `select sum(k) from games, unnest(unitbuilt) as k where timestarted + $1::interval > now() and k > 0`, timeInterval).Scan(&unitsProduced)
		if err != nil {
			ech <- err
		}
	}, func(ech chan<- error) {
		err := dbpool.QueryRow(r.Context(), `select sum(k) from games, unnest(structbuilt) as k where timestarted + $1::interval > now() and k > 0`, timeInterval).Scan(&structsBuilt)
		if err != nil {
			ech <- err
		}
	}, func(ech chan<- error) {
		err := dbpool.QueryRow(r.Context(), `select sum(coalesce(abs(elodiff[1]), 0)) from games where timestarted + $1::interval > now();`, timeInterval).Scan(&eloTransferred)
		if err != nil {
			ech <- err
		}
	}, func(ech chan<- error) {
		err := dbpool.QueryRow(r.Context(), `select sum(coalesce(abs(ratingdiff[1]), 0)) from games where timestarted + $1::interval > now();`, timeInterval).Scan(&ratingTransferred)
		if err != nil {
			ech <- err
		}
	}, func(ech chan<- error) {
		d, err := getDatesGraphData(r.Context(), "day")
		if err != nil {
			ech <- err
		}
		gamesGraph, err = json.Marshal(d)
		if err != nil {
			ech <- err
		}
	})
	if err != nil {
		log.Println(err)
	}
	basicLayoutLookupRespond("index", w, r, map[string]interface{}{
		"News":               news,
		"LastGames":          gamesPlayed,
		"LastGamesMasterbal": gamesPlayedMasterbal,
		"LastGamesTiny":      gamesPlayedTiny,
		"LastGamesTinyPrc":   fmt.Sprintf("(%.1f%%)", float64(gamesPlayedTiny)/float64(gamesPlayed)*float64(100)),
		"LastPlayers":        uniqPlayers,
		"LastGTime":          gameTime,
		"LastProduced":       humanize.Comma(int64(unitsProduced)),
		"LastBuilt":          humanize.Comma(int64(structsBuilt)),
		"LastElo":            eloTransferred,
		"LastRating":         ratingTransferred,
		"GamesGraph":         string(gamesGraph),
	})
}
