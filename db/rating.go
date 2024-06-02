package db

import (
	"context"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Rating struct {
	UserID           int `db:"user"`
	RatingCategoryID int `db:"category"`
	Elo              int
}

func GetUserRating(ctx context.Context, db *pgxpool.Pool, userID int, ratingCategory int) (*Rating, error) {
	r := &Rating{
		UserID:           userID,
		RatingCategoryID: ratingCategory,
		Elo:              1400,
	}
	return r, pgxscan.Select(ctx, db, r, `SELECT elo FROM rating WHERE account=$1 AND category=$2`, userID, ratingCategory)
}
