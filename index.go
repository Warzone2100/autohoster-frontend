package main

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v4"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
	type announcement struct {
		Title   string        `json:"title"`
		Content template.HTML `json:"content"`
		Time    time.Time     `json:"when_posted"`
		Color   string        `json:"color"`
	}
	timeInterval := "48 hours"
	announcements := []announcement{}
	gamesPlayed := 0
	gamesPlayedByPlayercount := map[int]int{}
	uniqPlayers := 0
	gameTime := 0
	// unitsProduced := 0
	// structsBuilt := 0
	gamesGraph := map[string]int{}
	gamesGraphAvg := map[string]int{}
	gamesGraphRated := map[string]int{}
	gamesGraphRatedAvg := map[string]int{}
	gamesGraphNonBot := map[string]int{}
	gamesGraphNonBotAvg := map[string]int{}
	err := RequestMultiple(func() error {
		rows, err := dbpool.Query(r.Context(), `select title, content, when_posted, color from announcements order by when_posted desc limit 25;`)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				announcements = []announcement{{
					Title:   "No announcements yet",
					Content: "I am lazy",
					Time:    time.Now(),
					Color:   "success",
				}}
			} else {
				return err
			}
		} else {
			for rows.Next() {
				var a announcement
				err = rows.Scan(&a.Title, &a.Content, &a.Time, &a.Color)
				if err != nil {
					return err
				}
				announcements = append(announcements, a)
			}
		}
		return nil
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select count(*) from games where time_started + $1::interval > now() and mods != 'masterbal'`, timeInterval).Scan(&gamesPlayed)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select
	count(distinct coalesce(a.id::text, i.hash))
from players as p
join games as g on g.id = p.game
join identities as i on i.id = p.identity
left join accounts as a on a.id = i.account
where g.time_started > now()-$1::interval`, timeInterval).Scan(&uniqPlayers)
	}, func() error {
		return dbpool.QueryRow(r.Context(), `select coalesce(sum(game_time), 0) from games where time_started + $1::interval > now()`, timeInterval).Scan(&gameTime)
		// }, func() error { // TODO: migrate
		// 	return dbpool.QueryRow(r.Context(), `select coalesce(sum(k), 0) from games, unnest(unitbuilt) as k where time_started + $1::interval > now() and k > 0`, timeInterval).Scan(&unitsProduced)
		// }, func() error { // TODO: migrate
		// 	return dbpool.QueryRow(r.Context(), `select coalesce(sum(k), 0) from games, unnest(structbuilt) as k where time_started + $1::interval > now() and k > 0`, timeInterval).Scan(&structsBuilt)

	}, func() error {
		var pc, gc int
		_, err := dbpool.QueryFunc(r.Context(), `with gms as (select
			g.id, count(p)
		from games as g
		join players as p on p.game = g.id
		where g.time_started > now()-$1::interval
		group by g.id
		order by g.id)
		
		select
			count, count(id)
		from gms
		group by count`,
			[]any{timeInterval}, []any{&pc, &gc}, func(_ pgx.QueryFuncRow) error {
				gamesPlayedByPlayercount[pc] = gc
				return nil
			})
		return err
	}, func() error {
		var date string
		var count, average int
		var nbcount, nbaverage int
		_, err := dbpool.QueryFunc(r.Context(), `SELECT
	to_char(date_trunc('day', g.time_started), 'YYYY-MM-DD') as d,
	count(g.time_started) as c,
	round(avg(count(g.time_started)) over(order by date_trunc('day', g.time_started) rows between 6 preceding and current row)),
	count(g.time_started) - sum(case when gc.category = 5 then 1 else 0 end) as nb,
	round(avg(count(g.time_started) - sum(case when gc.category = 5 then 1 else 0 end)) over(order by date_trunc('day', g.time_started) rows between 6 preceding and current row))
FROM games as g
LEFT JOIN games_rating_categories as gc on gc.game = g.id
WHERE g.time_started > now() - '1 year 7 days'::interval
GROUP BY date_trunc('day', g.time_started)
ORDER BY date_trunc('day', g.time_started)`,
			[]any{}, []any{&date, &count, &average, &nbcount, &nbaverage}, func(_ pgx.QueryFuncRow) error {
				gamesGraph[date] = count
				gamesGraphAvg[date] = average
				gamesGraphRated[date] = 0
				gamesGraphRatedAvg[date] = 0
				gamesGraphNonBot[date] = nbcount
				gamesGraphNonBotAvg[date] = nbaverage
				return nil
			})
		return err
	})

	if err != nil {
		log.Println(err)
	}
	basicLayoutLookupRespond("index", w, r, map[string]any{
		"News":               announcements,
		"LastGames":          gamesPlayed,
		"LastGamesByPlayers": gamesPlayedByPlayercount,
		"LastPlayers":        uniqPlayers,
		"LastGTime":          gameTime,
		// "LastProduced":        humanize.Comma(int64(unitsProduced)), // TODO: migrate
		// "LastBuilt":           humanize.Comma(int64(structsBuilt)),  // TODO: migrate
		"GamesGraph":          gamesGraph,
		"GamesGraphAvg":       gamesGraphAvg,
		"GamesGraphRated":     gamesGraphRated,
		"GamesGraphRatedAvg":  gamesGraphRatedAvg,
		"GamesGraphNonBot":    gamesGraphNonBot,
		"GamesGraphNonBotAvg": gamesGraphNonBotAvg,
	})
}
