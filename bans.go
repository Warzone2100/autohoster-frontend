package main

import (
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v4"
)

func bansHandler(w http.ResponseWriter, r *http.Request) {
	type viewBan struct {
		ID        int
		Player    PlayerLeaderboard
		Reason    string
		IssuedAt  string
		ExpiresAt string
		IsBanned  bool
	}
	ret := []viewBan{}

	var banid, duration int
	var whenbanned time.Time
	var reason string

	var pid, pelo2, pautoplayed, pautolost, pautowon, puid int
	var pname, phash string
	_, err := dbpool.QueryFunc(r.Context(),
		`select bans.id, time_issued, time_expires, reason
		from bans
		order by bans.id desc;`, []any{},
		[]any{&banid, &whenbanned, &duration, &reason},
		func(_ pgx.QueryFuncRow) error {
			v := viewBan{
				ID: banid,
				Player: PlayerLeaderboard{
					ID:         pid,
					Name:       pname,
					Hash:       phash,
					Elo2:       pelo2,
					Autoplayed: pautoplayed,
					Autolost:   pautolost,
					Autowon:    pautowon,
					Userid:     puid,
				},
				Reason:   reason,
				IssuedAt: whenbanned.Format(time.DateTime),
			}
			if duration == 0 {
				v.ExpiresAt = "Never"
			} else {
				expiresAt := whenbanned.Add(time.Second * time.Duration(duration))
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
