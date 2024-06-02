package db

import (
	"context"
	"time"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Account struct {
	ID               int
	Username         string
	Email            string
	AccountCreated   time.Time
	LastSeen         time.Time
	Terminated       bool
	EmailConfirmed   *time.Time
	AllowHostRequest bool
	BypasssISPBan    bool `db:"bypass_ispban"`
	DisplayName      *string
	LastReport       time.Time
	LastRequest      time.Time
}

func GetAccounts(ctx context.Context, db *pgxpool.Pool) ([]*Account, error) {
	r := []*Account{}
	return r, pgxscan.Select(ctx, db, &r, `SELECT
		id, username, email, account_created,
		last_seen, terminated, email_confirmed,
		allow_host_request, bypass_ispban, display_name,
		last_report, last_request FROM accounts`)
}
