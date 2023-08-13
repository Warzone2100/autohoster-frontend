package main

import (
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
		`select bans.id, whenbanned, duration, reason,
		players.id, name, players.hash, elo2, autoplayed, autolost, autowon, coalesce(users.id, -1) as userid
		from bans
		join players on playerid = players.id
		full outer join users on playerid = users.wzprofile2
		where playerid is not null
		order by bans.id desc;`, []interface{}{},
		[]interface{}{&banid, &whenbanned, &duration, &reason, &pid, &pname, &phash, &pelo2, &pautoplayed, &pautolost, &pautowon, &puid},
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
		return
	}
	basicLayoutLookupRespond("bans", w, r, map[string]any{
		"Bans": ret,
	})
}
