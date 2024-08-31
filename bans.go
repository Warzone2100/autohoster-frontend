package main

import (
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v4"
)

func bansHandler(w http.ResponseWriter, r *http.Request) {
	type viewBan struct {
		ID           int
		Identity     *int
		IdentityName *string
		IdentityKey  *string
		Account      *int
		AccountName  *string
		Reason       string
		IssuedAt     string
		ExpiresAt    string
		IsBanned     bool
	}
	ret := []viewBan{}

	var (
		banid       int
		whenbanned  time.Time
		whenexpires *time.Time
		reason      string
		ident       *int
		identName   *string
		identKey    *string
		acc         *int
		accName     *string
	)

	_, err := dbpool.QueryFunc(r.Context(),
		`select
	bans.id, accounts.id, accounts.display_name, identities.id, identities.name, coalesce(encode(identities.pkey, 'hex'), identities.hash), time_issued, time_expires, reason
from bans
left join identities on bans.identity = identities.id
left join accounts on bans.account = accounts.id
order by bans.id desc;`, []any{},
		[]any{&banid, &acc, &accName, &ident, &identName, &identKey, &whenbanned, &whenexpires, &reason},
		func(_ pgx.QueryFuncRow) error {
			v := viewBan{
				ID:           banid,
				Identity:     ident,
				IdentityName: identName,
				IdentityKey:  identKey,
				Account:      acc,
				AccountName:  accName,
				Reason:       reason,
				IssuedAt:     whenbanned.Format(time.DateTime),
			}
			if whenexpires == nil {
				v.ExpiresAt = "Never"
			} else {
				expiresAt := *whenexpires
				v.ExpiresAt = expiresAt.Format(time.DateTime)
				v.IsBanned = time.Now().Before(expiresAt)
			}
			ret = append(ret, v)
			return nil
		})
	if err != nil {
		log.Println(err)
		return
	}
	basicLayoutLookupRespond("bans", w, r, map[string]any{
		"Bans": ret,
	})
}
