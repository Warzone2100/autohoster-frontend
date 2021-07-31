package main

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"math"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/jackc/pgx/v4"
)

type Elo struct {
	ID         int `json:ID`
	Elo        int `json:"elo"`
	Elo2       int `json:"elo2"`
	Autowon    int `json:"autowon"`
	Autolost   int `json:"autolost"`
	Autoplayed int `json:"autoplayed"`
}

type EloGamePlayer struct {
	ID       int
	Team     int
	Usertype string
	ElloDiff int
}

type EloGame struct {
	ID       int
	GameTime int
	Base     int
	Players  []EloGamePlayer
}

func CalcEloForAll(G []*EloGame, P map[int]*Elo) {
	for _, p := range P {
		p.Elo = 1400
		p.Elo2 = 1400
	}
	for gamei, game := range G {
		Team1ID := []int{}
		Team2ID := []int{}
		for _, p := range game.Players {
			if p.Team == 0 {
				Team1ID = append(Team1ID, p.ID)
			} else if p.Team == 1 {
				Team2ID = append(Team2ID, p.ID)
			}
		}
		if len(Team1ID) != len(Team2ID) {
			continue
		}
		Team1EloSum := 0
		Team2EloSum := 0
		for _, p := range Team1ID {
			Team1EloSum += P[p].Elo
		}
		for _, p := range Team2ID {
			Team2EloSum += P[p].Elo
		}
		Team1Won := 0
		Team2Won := 0
		if game.Players[0].Usertype == "winner" {
			SecondTeamFoundLost := false
			for i, p := range game.Players {
				if i == 0 {
					continue
				}
				if p.Team != game.Players[0].Team && p.Usertype == "loser" {
					SecondTeamFoundLost = true
					break
				}
			}
			Team1Won = 1
			if !SecondTeamFoundLost {
				log.Printf("Game %d is sus", game.ID)
			}
		} else if game.Players[0].Usertype == "loser" {
			SecondTeamFoundWon := false
			for i, p := range game.Players {
				if i == 0 {
					continue
				}
				if p.Team != game.Players[0].Team && p.Usertype == "winner" {
					SecondTeamFoundWon = true
					break
				}
			}
			Team2Won = 1
			if !SecondTeamFoundWon {
				log.Printf("Game %d is sus", game.ID)
			}
		}
		Team1EloAvg := Team1EloSum / len(Team1ID)
		Team2EloAvg := Team2EloSum / len(Team2ID)
		log.Printf("Processing game %d", game.ID)
		log.Printf("Team won: %v %v", Team2Won, Team1Won)
		log.Printf("Team avg: %v %v", Team1EloAvg, Team2EloAvg)
		K := float64(20)
		Elo1 := 1 / (1 + math.Pow(float64(10), float64(Team1EloAvg-Team2EloAvg)/float64(400)))
		Elo2 := 1 / (1 + math.Pow(float64(10), float64(Team2EloAvg-Team1EloAvg)/float64(400)))
		log.Printf("Elo: %v %v", Elo1, Elo2)
		New1 := Team1EloAvg + int(math.Round(K*(float64(Team2Won)-Elo1)))
		New2 := Team2EloAvg + int(math.Round(K*(float64(Team1Won)-Elo2)))
		log.Printf("New: %v %v", New1, New2)

		Additive := 0
		if New1-Team1EloAvg >= 0 {
			Additive = New1 - Team1EloAvg
		} else {
			Additive = New2 - Team2EloAvg
		}

		for pi, p := range game.Players {
			if p.Usertype == "winner" {
				P[p.ID].Autowon++
				P[p.ID].Elo += Additive
				G[gamei].Players[pi].ElloDiff = Additive
			} else if p.Usertype == "loser" {
				P[p.ID].Autolost++
				P[p.ID].Elo += -Additive + game.GameTime/600
				G[gamei].Players[pi].ElloDiff = -Additive + game.GameTime/600
			}
			P[p.ID].Autoplayed++
		}
	}
}

func EloRecalcHandler(w http.ResponseWriter, r *http.Request) {
	rows, derr := dbpool.Query(context.Background(), `
				SELECT
					games.id as gid, gametime,
					players, teams, usertype,
					array_agg(row_to_json(p))::text[] as pnames
				FROM games
				JOIN players as p ON p.id = any(games.players)
				WHERE deleted = false AND hidden = false AND calculated = true AND finished = true
				GROUP BY gid
				ORDER BY timestarted
				LIMIT 5`)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msg": "No games played"})
		} else {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database query error: " + derr.Error()})
		}
		return
	}
	defer rows.Close()
	Games := []*EloGame{}
	Players := map[int]*Elo{}
	for rows.Next() {
		var g EloGame
		var players []int
		var teams []int
		var usertype []string
		var playerinfo []string
		err := rows.Scan(&g.ID, &g.GameTime, &players, &teams, &usertype, &playerinfo)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database scan error: " + err.Error()})
			return
		}
		for _, pv := range playerinfo {
			var e Elo
			err := json.Unmarshal([]byte(pv), &e)
			if err != nil {
				basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Json error: " + err.Error()})
				return
			}
			Players[e.ID] = &e
		}
		for pslt, pid := range players {
			if pid == -1 {
				continue
			}
			var p EloGamePlayer
			p.Usertype = usertype[pslt]
			p.ID = pid
			p.Team = teams[pslt]
			p.ElloDiff = 0
			g.Players = append(g.Players, p)
		}
		Games = append(Games, &g)
	}
	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"nocenter": true, "msg": template.HTML("<pre>" + spew.Sdump(Players) + spew.Sdump(Games) + "</pre>")})
	CalcEloForAll(Games, Players)
	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"nocenter": true, "msg": template.HTML("<pre>" + spew.Sdump(Players) + spew.Sdump(Games) + "</pre>")})
}
