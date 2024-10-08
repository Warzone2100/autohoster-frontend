package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type Ra struct {
	Dummy                 bool   `json:"dummy"`
	Autohoster            bool   `json:"autohoster"`
	Star                  [3]int `json:"star"`
	Medal                 int    `json:"medal"`
	Level                 int    `json:"level"`
	Elo                   string `json:"elo"`
	Details               string `json:"details"`
	Name                  string `json:"name"`
	Tag                   string `json:"tag"`
	NameTextColorOverride [3]int `json:"nameTextColorOverride"`
	TagTextColorOverride  [3]int `json:"tagTextColorOverride"`
	EloTextColorOverride  [3]int `json:"eloTextColorOverride"`
}

func ratingHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	hash, ok := params["hash"]
	if !ok {
		hash = r.Header.Get("WZ-Player-Hash")
	}
	w.Header().Set("Content-Type", "application/json")
	if hash == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{\"error\": \"Empty hash.\"}"))
		return
	}
	m := ratingLookup(hash, r.Header.Get("WZ-Version"))
	j, err := json.Marshal(m)
	if err != nil {
		log.Println(err.Error())
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, string(j))
}

func ratingLookup(hash string, gameVersion string) Ra {
	m := Ra{
		Dummy:                 false,
		Autohoster:            false,
		Star:                  [3]int{},
		Medal:                 0,
		Level:                 0,
		Elo:                   "",
		Details:               "",
		Name:                  "",
		Tag:                   "",
		NameTextColorOverride: [3]int{0xff, 0xff, 0xff},
		TagTextColorOverride:  [3]int{0xff, 0xff, 0xff},
		EloTextColorOverride:  [3]int{0xff, 0xff, 0xff},
	}
	ohash, ok := cfg.GetString("ratingOverrides", hash)
	if ok {
		hash = ohash
	}
	if hash == "a0c124533ddcaf5a19cc7d593c33d750680dc428b0021672e0b86a9b0dcfd711" {
		m.Autohoster = true
		var c int
		derr := dbpool.QueryRow(context.Background(), "select count(games) from games where hidden = false and deleted = false;").Scan(&c)
		if derr != nil {
			log.Print(derr)
		}
		m.Details = "wz2100-autohost.net\n\nTotal games served: " + strconv.Itoa(c) + "\n"
		m.Elo = "Visit wz2100-autohost.net"
		return m
	}
	if hash == "21494390542d3bb20bb39c0986c2c6d9a338be2db3f68b47610744be6b2045f2" {
		m.Autohoster = false
		m.Details = "Used to be CleptoMantis but now he is fake Autohoster"
		m.Elo = "Fake autohoster"
		m.NameTextColorOverride = [3]int{0x00, 0x00, 0x00}
		m.EloTextColorOverride = [3]int{0xff, 0x00, 0x00}
		return m
	}
	var delo, dautoplayed, dautowon, dautolost, duuserid int
	var duterm, dallowed, dadmin bool
	var dname string
	derr := dbpool.QueryRow(context.Background(), `select
	identities.name, accounts.id, accounts.terminated, accounts.allow_host_request, accounts.superadmin, rating.elo, rating.played, rating.won, rating.lost
from identities
left join accounts on identities.account = accounts.id
left join rating on accounts.id = rating.account
left join rating_categories on rating.category = rating_categories.id
where hash = $1 and category = 2`, hash).
		Scan(&dname, &duuserid, &duterm, &dallowed, &dadmin, &delo, &dautoplayed, &dautowon, &dautolost)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			if m.Elo == "" {
				wonCount := 0
				err := dbpool.QueryRow(context.Background(), `select count(p)
from players as p
join identities as i on p.identity = i.id
where p.usertype = 'winner' and i.hash = $1`, hash).Scan(&wonCount)
				m.Details = "Casual noname"
				m.NameTextColorOverride = [3]int{0x66, 0x66, 0x66}
				m.EloTextColorOverride = [3]int{0xff, 0x44, 0x44}
				if err != nil {
					if err != pgx.ErrNoRows {
						log.Println(err)
					}
					m.Elo = "Unknown player"
				} else {
					m.Elo = fmt.Sprintf("Unknown player (% 4d wins)", wonCount)
				}
			}
		} else {
			log.Print(derr)
		}
		return m
	}

	if gameVersion != "" {
		m.Name = dname
	}

	if duuserid > 0 {
		m.Details += fmt.Sprintf("Rating: % 4d\n", delo)
		m.Details = fmt.Sprintf("Played: % 4d\n", dautoplayed)
		m.Details += fmt.Sprintf("Won: % 4d Lost: % 4d\n", dautowon, dautolost)
		if dallowed {
			m.Details += "Allowed to moderate and request rooms\n"
		}
	} else {
		m.Details += "Not registered user.\n"
	}
	// if len(drenames) > 0 {
	// 	m.Details += "Other names:"
	// 	for _, v := range drenames {
	// 		m.Details += "\n" + v
	// 	}
	// }
	// m.Details += fmt.Sprintf("Elo: %d (#%d)\n", de, dep)

	// if isAprilFools() {
	// 	dbpool.QueryRow(context.Background(), `select elo2, autoplayed, autolost, autowon from players join accounts on accounts.wzprofile2 = players.id where autoplayed > 5 and accounts.id != 0 order by random() limit 1;`).Scan(&delo2, &dautoplayed, &dautolost, &dautowon)
	// 	m.Level = rand.Intn(8)
	// 	if duuserid == 14 || duuserid == 17 {
	// 		m.Level = 8
	// 	}
	// }

	if m.Elo == "" {
		var pc string
		if dautowon+dautolost > 0 {
			pc = fmt.Sprintf("%03.1f%%", float64(100)*(float64(dautowon)/float64(dautowon+dautolost)))
		} else {
			pc = "-"
		}
		if duuserid != -1 && duuserid != 0 {
			m.Elo = fmt.Sprintf("R[% 4d] % 4d %s", delo, dautoplayed, pc)
		} else {
			m.Elo = "Unauthorized player"
			m.NameTextColorOverride = [3]int{0x66, 0x66, 0x66}
			m.EloTextColorOverride = [3]int{0xff, 0x44, 0x44}
		}
	}

	if dallowed {
		m.Level = 7
		if gameVersion != "" {
			m.TagTextColorOverride = [3]int{0x11, 0xaa, 0x11}
			m.Tag = "Moderator"
		} else {
			m.NameTextColorOverride = [3]int{0x11, 0xaa, 0x11}
			m.Name = "Moderator"
		}
	}
	if dadmin {
		m.Level = 8
		if gameVersion != "" {
			m.Tag = "Admin"
			m.TagTextColorOverride = [3]int{0x33, 0xff, 0x33}
		} else {
			m.NameTextColorOverride = [3]int{0x33, 0xff, 0x33}
			m.Name = "Admin"
		}
	}
	if duterm {
		m.Level = 0
		m.NameTextColorOverride = [3]int{0xff, 0x22, 0x22}
		m.EloTextColorOverride = [3]int{0xff, 0x22, 0x22}
		m.TagTextColorOverride = [3]int{0xff, 0x22, 0x22}
		if gameVersion != "" {
			m.Tag = ""
		} else {
			m.Name = ""
		}
		m.Elo = "Account terminated"
	} else {
		if dautoplayed < 5 || duuserid <= 0 {
			m.Dummy = true
		} else {
			m.Dummy = false
			if dautolost == 0 {
				dautolost = 1
			}
			if dautowon >= 24 && float64(dautowon)/float64(dautolost) > 6.0 {
				m.Medal = 1
			} else if dautowon >= 12 && float64(dautowon)/float64(dautolost) > 4.0 {
				m.Medal = 2
			} else if dautowon >= 6 && float64(dautowon)/float64(dautolost) > 3.0 {
				m.Medal = 3
			}
			if delo > 1800 {
				m.Star[0] = 1
			} else if delo > 1550 {
				m.Star[0] = 2
			} else if delo > 1400 {
				m.Star[0] = 3
			}
			if dautoplayed > 60 {
				m.Star[1] = 1
			} else if dautoplayed > 30 {
				m.Star[1] = 2
			} else if dautoplayed > 10 {
				m.Star[1] = 3
			}
			if dautowon > 60 {
				m.Star[2] = 1
			} else if dautowon > 30 {
				m.Star[2] = 2
			} else if dautowon > 10 {
				m.Star[2] = 3
			}
		}
	}

	return m
}
