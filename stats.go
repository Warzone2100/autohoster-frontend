package main

import (
	"log"
	"net/http"

	"github.com/jackc/pgx/v4"
)

func statsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	GamesByHour := map[int]int{}
	RatingGamesByHour := map[int]int{}
	GamesByWeekday := map[int]int{}
	GamesByWeekdayLast := map[int]int{}
	PlayerCount := map[string]int{}
	PlayerCountAvg := map[string]int{}
	RatingPlayerCount := map[string]int{}
	RatingPlayerCountAvg := map[string]int{}
	MapCounts := map[string]int{}
	LastPlayers := []struct {
		ID    int
		Name  string
		Count int
		Diff  int
	}{}
	GamesByPlayercount := map[int]int{2: 0, 4: 0, 6: 0, 8: 0, 10: 0}
	GamesByPlayercountLast := map[int]int{2: 0, 4: 0, 6: 0, 8: 0, 10: 0}
	RatingGamesByPlayercount := map[int]int{2: 0, 4: 0, 6: 0, 8: 0, 10: 0}
	RatingGamesByPlayercountLast := map[int]int{2: 0, 4: 0, 6: 0, 8: 0, 10: 0}
	err := RequestMultiple(func() error {
		var d, c int
		_, err := dbpool.QueryFunc(ctx, `SELECT COUNT(gg) AS c, EXTRACT('hour' FROM timestarted) AS d FROM games AS gg GROUP BY d ORDER BY d`,
			[]interface{}{}, []interface{}{&c, &d},
			func(_ pgx.QueryFuncRow) error {
				GamesByHour[d] = c
				return nil
			})
		return err
	}, func() error {
		var d, c int
		_, err := dbpool.QueryFunc(ctx, `SELECT COUNT(gg) AS c, EXTRACT('hour' FROM timestarted) AS d FROM games AS gg WHERE not gg.ratingdiff @> ARRAY[0] GROUP BY d ORDER BY d`,
			[]interface{}{}, []interface{}{&c, &d},
			func(_ pgx.QueryFuncRow) error {
				RatingGamesByHour[d] = c
				return nil
			})
		return err
	}, func() error {
		var d, c int
		_, err := dbpool.QueryFunc(ctx, `SELECT COUNT(gg) AS c, EXTRACT('isodow' FROM timestarted) AS d FROM games AS gg GROUP BY d ORDER BY d`,
			[]interface{}{}, []interface{}{&c, &d},
			func(_ pgx.QueryFuncRow) error {
				GamesByWeekday[d] = c
				return nil
			})
		return err
	}, func() error {
		var d, c int
		_, err := dbpool.QueryFunc(ctx, `SELECT COUNT(gg) AS c, EXTRACT('isodow' FROM timestarted) AS d FROM games AS gg WHERE timestarted + '2 weeks'::interval > now() GROUP BY d ORDER BY d`,
			[]interface{}{}, []interface{}{&c, &d},
			func(_ pgx.QueryFuncRow) error {
				GamesByWeekdayLast[d] = c
				return nil
			})
		return err
	}, func() error {
		var mapname string
		var count int
		_, err := dbpool.QueryFunc(ctx, `SELECT mapname, COUNT(*) AS c FROM games WHERE timestarted + '2 weeks'::interval > now() GROUP BY mapname ORDER BY c DESC LIMIT 30`,
			[]interface{}{}, []interface{}{&mapname, &count},
			func(_ pgx.QueryFuncRow) error {
				MapCounts[mapname] = count
				return nil
			})
		return err
	}, func() error {
		var date string
		var count int
		var avg int
		_, err := dbpool.QueryFunc(ctx, `SELECT
		to_char(d, 'YYYY-MM-DD'),
		count(r.p),
		round(avg(count(r.p)) OVER(ORDER BY d ROWS BETWEEN 7 PRECEDING AND CURRENT ROW))
	FROM (SELECT
		DISTINCT unnest(gg.players) as p,
		date_trunc('day', gg.timestarted) AS d
		FROM games AS gg) as r
	GROUP BY d
	ORDER BY d DESC`,
			[]interface{}{}, []interface{}{&date, &count, &avg},
			func(_ pgx.QueryFuncRow) error {
				PlayerCount[date] = count
				PlayerCountAvg[date] = avg
				return nil
			})
		return err
	}, func() error {
		var date string
		var count int
		var avg int
		_, err := dbpool.QueryFunc(ctx, `SELECT
		to_char(gg.d, 'YYYY-MM-DD') as ret_date,
		count(gg.p) as ret_count,
		round(avg(count(gg.p)) OVER(ORDER BY gg.d ROWS BETWEEN 7 PRECEDING AND CURRENT ROW)) as ret_avg
	FROM (SELECT
		DISTINCT unnest(gg.players) AS p,
		date_trunc('day', gg.timestarted) AS d
		FROM games AS gg) AS gg
	JOIN users AS u ON gg.p = u.wzprofile2
	GROUP BY gg.d
	ORDER BY gg.d DESC`,
			[]interface{}{}, []interface{}{&date, &count, &avg},
			func(_ pgx.QueryFuncRow) error {
				RatingPlayerCount[date] = count
				RatingPlayerCountAvg[date] = avg
				return nil
			})
		return err
	}, func() error {
		var name string
		var count, id, diff int
		_, err := dbpool.QueryFunc(ctx, `SELECT
		p.id, p.name, count(g) AS c, sum(g.ratingdiff[array_position(g.players, p.id)])
	FROM players AS p
	JOIN users AS u ON u.wzprofile2 = p.id
	JOIN (SELECT players, ratingdiff FROM games WHERE timestarted + '7 days' > now()) AS g ON p.id = any(g.players)
	WHERE p.autoplayed > 10
	GROUP BY p.id
	ORDER BY c DESC, p.autoplayed DESC`,
			[]interface{}{}, []interface{}{&id, &name, &count, &diff},
			func(_ pgx.QueryFuncRow) error {
				LastPlayers = append(LastPlayers, struct {
					ID    int
					Name  string
					Count int
					Diff  int
				}{
					ID:    id,
					Name:  name,
					Count: count,
					Diff:  diff,
				})
				return nil
			})
		return err
	}, func() error {
		var pc, c int
		_, err := dbpool.QueryFunc(ctx, `select array_position(players, -1)-1 as playercount, count(id)*(array_position(players, -1)-1) as c
from games
where calculated = true
group by playercount
order by playercount`,
			[]interface{}{}, []interface{}{&pc, &c},
			func(_ pgx.QueryFuncRow) error {
				GamesByPlayercount[pc] = c
				return nil
			})
		return err
	}, func() error {
		var pc, c int
		_, err := dbpool.QueryFunc(ctx, `select array_position(players, -1)-1 as playercount, count(id)*(array_position(players, -1)-1) as c
from games
where calculated = true and timestarted + '2 months' > now()
group by playercount
order by playercount`,
			[]interface{}{}, []interface{}{&pc, &c},
			func(_ pgx.QueryFuncRow) error {
				GamesByPlayercountLast[pc] = c
				return nil
			})
		return err
	}, func() error {
		var pc, c int
		_, err := dbpool.QueryFunc(ctx, `select array_position(players, -1)-1 as playercount, count(id)*(array_position(players, -1)-1) as c
from games
where calculated = true and ratingdiff[1] != 0
group by playercount
order by playercount`,
			[]interface{}{}, []interface{}{&pc, &c},
			func(_ pgx.QueryFuncRow) error {
				RatingGamesByPlayercount[pc] = c
				return nil
			})
		return err
	}, func() error {
		var pc, c int
		_, err := dbpool.QueryFunc(ctx, `select array_position(players, -1)-1 as playercount, count(id)*(array_position(players, -1)-1) as c
from games
where calculated = true and ratingdiff[1] != 0 and timestarted + '2 months' > now()
group by playercount
order by playercount`,
			[]interface{}{}, []interface{}{&pc, &c},
			func(_ pgx.QueryFuncRow) error {
				RatingGamesByPlayercountLast[pc] = c
				return nil
			})
		return err
	})

	if err != nil {
		log.Println(err)
	}
	basicLayoutLookupRespond("stats", w, r, map[string]interface{}{
		"GamesByHour":                  GamesByHour,
		"RatingGamesByHour":            RatingGamesByHour,
		"GamesByWeekday":               GamesByWeekday,
		"GamesByWeekdayLast":           GamesByWeekdayLast,
		"PlayersByDay":                 PlayerCount,
		"PlayersByDayAvg":              PlayerCountAvg,
		"RatingPlayersByDay":           RatingPlayerCount,
		"RatingPlayersByDayAvg":        RatingPlayerCountAvg,
		"MapCounts":                    MapCounts,
		"LastPlayers":                  LastPlayers,
		"GamesByPlayercount":           GamesByPlayercount,
		"GamesByPlayercountLast":       GamesByPlayercountLast,
		"RatingGamesByPlayercount":     RatingGamesByPlayercount,
		"RatingGamesByPlayercountLast": RatingGamesByPlayercountLast,
	})
}
