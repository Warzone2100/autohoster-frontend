package main

import (
	"context"
	"net/http"
	"time"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4/pgxpool"
)

type RatingCategory struct {
	ID         int
	TimeStarts **time.Time
	TimeEnds   **time.Time
	Name       string
}

func GetRatingCategories(ctx context.Context, db *pgxpool.Pool) ([]*RatingCategory, error) {
	r := []*RatingCategory{}
	return r, pgxscan.Select(ctx, db, &r, `SELECT * FROM rating_categories`)
}

type LeaderboardEntry struct {
	Name       string `db:"display_name"`
	Account    int
	Category   int
	Elo        int
	Played     int
	Won        int
	Lost       int
	TimePlayed int
}

func GetLeaderboardTop(ctx context.Context, db *pgxpool.Pool, category int, limit int) ([]*LeaderboardEntry, error) {
	r := []*LeaderboardEntry{}
	return r, pgxscan.Select(ctx, db, &r, `SELECT * FROM leaderboard WHERE category = $1 LIMIT $2`, category, limit)
}

func LeaderboardsHandler(w http.ResponseWriter, r *http.Request) {
	cats, err := GetRatingCategories(r.Context(), dbpool)
	if err != nil {
		basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": err.Error()})
		return
	}
	lb := map[*RatingCategory][]*LeaderboardEntry{}
	for _, c := range cats {
		l, err := GetLeaderboardTop(r.Context(), dbpool, c.ID, 3)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]any{"msgred": true, "msg": err.Error()})
			return
		}
		lb[c] = l
	}
	basicLayoutLookupRespond("leaderboards", w, r, map[string]any{"leaderboards": lb})
}
