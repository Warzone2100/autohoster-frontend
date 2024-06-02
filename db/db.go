package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func AddEventLog(ctx context.Context, db *pgxpool.Pool, format string, args ...any) error {
	tag, err := db.Exec(ctx, `INSERT INTO eventlog (msg) values ($1)`, fmt.Sprintf(format, args...))
	if err != nil {
		return err
	}
	if !tag.Insert() {
		return errors.New("returned tag is not insert")
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("returned affected rows on insert is not 1 (%d)", tag.RowsAffected())
	}
	return nil
}

type HashInfo struct {
	IdentityID         int
	Name               string
	AllowedJoining     bool
	AllowedChatting    bool
	AllowedPlaying     bool
	BanReasonString    string
	BanJoiningIssued   time.Time
	BanJoiningExpires  time.Time
	BanChattingIssued  time.Time
	BanChattingExpires time.Time
	BanPlayingIssued   time.Time
	BanPlayingExpires  time.Time
}

// func GetHashInfo(ctx context.Context, db *pgxpool.Pool, format string, args ...any) (*HashInfo, error) {
// 	hi := &HashInfo{}
// 	db.Query(ctx, `select bans.id
// 	from bans
// 	where when_expires >= now() and
// 		((select id from identities where hash = $1 or ) = identity or
// 		(select account from identities where hash = $1 or ) = account)`)
// }
