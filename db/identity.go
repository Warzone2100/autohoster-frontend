package db

import (
	"context"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Identity struct {
	ID      int
	Name    string
	Pkey    []byte
	Hash    string
	Account int
}

func GetUserIdentities(ctx context.Context, db *pgxpool.Pool, userID int) ([]*Identity, error) {
	r := []*Identity{}
	return r, pgxscan.Select(ctx, db, &r, `SELECT id, name, pkey, hash, account FROM identities WHERE account=$1`, userID)
}

func GetIdentities(ctx context.Context, db *pgxpool.Pool) ([]*Identity, error) {
	r := []*Identity{}
	return r, pgxscan.Select(ctx, db, &r, `SELECT id, name, pkey, hash, account FROM identities`)
}
