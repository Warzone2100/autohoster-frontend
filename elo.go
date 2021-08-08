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
	ID         int `json:"ID"`
	Elo        int `json:"elo"`
	Elo2       int `json:"elo2"`
	Autowon    int `json:"autowon"`
	Autolost   int `json:"autolost"`
	Autoplayed int `json:"autoplayed"`
	Userid     int `json:"f1"`
}

type EloGamePlayer struct {
	ID       int
	Team     int
	Usertype string
	EloDiff  int
}

type EloGame struct {
	ID       int
	GameTime int
	Base     int
	IsFFA    bool
	Players  []EloGamePlayer
}

func CalcElo(G *EloGame, P map[int]*Elo) {
	Team1ID := []int{}
	Team2ID := []int{}
	if G.IsFFA == false {
		for _, p := range G.Players {
			if p.Team == 0 {
				Team1ID = append(Team1ID, p.ID)
			} else if p.Team == 1 {
				Team2ID = append(Team2ID, p.ID)
			}
		}
	} else {
		fighterscount := 0
		for _, p := range G.Players {
			if p.Usertype == "winner" || p.Usertype == "loser" {
				fighterscount++
			}
		}
		if fighterscount == 2 {
			Team1ID = append(Team1ID, G.Players[0].ID)
			Team2ID = append(Team2ID, G.Players[1].ID)
		} else {
			log.Printf("FFA game with not 2 players: %d", G.ID)
			return
		}
	}
	if len(Team1ID) != len(Team2ID) {
		log.Printf("Incorrect length: %d", G.ID)
		return
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
	if G.Players[0].Usertype == "winner" {
		SecondTeamFoundLost := false
		for i, p := range G.Players {
			if i == 0 {
				continue
			}
			if (p.Team != G.Players[0].Team || G.IsFFA) && p.Usertype == "loser" {
				SecondTeamFoundLost = true
				break
			}
		}
		Team1Won = 1
		if !SecondTeamFoundLost {
			log.Printf("Game %d is sus", G.ID)
			return
		}
	} else if G.Players[0].Usertype == "loser" {
		SecondTeamFoundWon := false
		for i, p := range G.Players {
			if i == 0 {
				continue
			}
			if (p.Team != G.Players[0].Team || G.IsFFA) && p.Usertype == "winner" {
				SecondTeamFoundWon = true
				break
			}
		}
		Team2Won = 1
		if !SecondTeamFoundWon {
			log.Printf("Game %d is sus", G.ID)
			return
		}
	}

	Team1EloAvg := Team1EloSum / len(Team1ID)
	Team2EloAvg := Team2EloSum / len(Team2ID)
	log.Printf("Processing game %d", G.ID)
	log.Printf("Team won: %v %v", Team2Won, Team1Won)
	log.Printf("Team avg: %v %v", Team1EloAvg, Team2EloAvg)
	K := float64(20)
	Chance1 := 1 / (1 + math.Pow(float64(10), float64(Team1EloAvg-Team2EloAvg)/float64(400)))
	Chance2 := 1 / (1 + math.Pow(float64(10), float64(Team2EloAvg-Team1EloAvg)/float64(400)))
	log.Printf("Chances: %v %v", Chance1, Chance2)
	diff1 := int(math.Round(K * Chance1))
	diff2 := int(math.Round(K * Chance2))
	log.Printf("diff: %v %v", diff1, diff2)
	var Additive int
	var Timeitive int
	if G.Players[0].Usertype == "winner" {
		Additive = diff1
		Timeitive = diff2
	} else {
		Additive = diff2
		Timeitive = diff1
	}
	calcelo2 := true
	for _, p := range G.Players {
		if P[p.ID].Userid == -1 || P[p.ID].Userid == 0 {
			calcelo2 = false
			break
		}
	}
	for pi, p := range G.Players {
		log.Printf("Applying elo to player %d [%s]", pi, p.Usertype)
		if p.Usertype == "winner" {
			P[p.ID].Elo += Additive
			if calcelo2 {
				P[p.ID].Elo2 += Additive
			}
			P[p.ID].Autowon++
			G.Players[pi].EloDiff = Additive
			P[p.ID].Autoplayed++
			log.Printf(" === %d applying additive %d uid %d", pi, Additive, P[p.ID].Userid)
		} else if p.Usertype == "loser" {
			P[p.ID].Elo -= Additive
			if calcelo2 {
				P[p.ID].Elo2 -= Additive
			}
			P[p.ID].Elo += int(math.Round((float64(Timeitive) / float64(60)) * (float64(G.GameTime) / (float64(90000) - 10))))
			if calcelo2 {
				P[p.ID].Elo2 += int(math.Round((float64(Timeitive) / float64(60)) * (float64(G.GameTime) / (float64(90000) - 10))))
			}
			log.Printf(" === %d applying additive %d", pi, Additive)
			log.Printf(" === %d applying time bonus (%d/60) = %d :: (%d/60000) = %d [[[%d]]]", pi, Timeitive, (float64(Timeitive) / float64(60)), float64(G.GameTime), (float64(G.GameTime) / float64(60000)), math.Round((float64(Timeitive)/float64(60))*(float64(G.GameTime)/float64(60000)-5)))
			P[p.ID].Autolost++
			G.Players[pi].EloDiff -= Additive
			G.Players[pi].EloDiff += int(math.Round((float64(Timeitive) / float64(60)) * (float64(G.GameTime) / (float64(90000) - 10))))

			P[p.ID].Autoplayed++
		}
	}
}

func CalcEloForAll(G []*EloGame, P map[int]*Elo) {
	for _, p := range P {
		p.Elo = 1400
		if p.Userid != -1 && p.Userid != 0 {
			p.Elo2 = 1400
		} else {
			p.Elo2 = 0
		}
		p.Autowon = 0
		p.Autolost = 0
		p.Autoplayed = 0
	}
	for gamei, _ := range G {
		CalcElo(G[gamei], P)
	}
}

func EloRecalcHandler(w http.ResponseWriter, r *http.Request) {
	rows, derr := dbpool.Query(context.Background(), `
				SELECT
					games.id as gid, gametime, alliancetype,
					players, teams, usertype,
					array_agg(to_json(p)::jsonb || to_json(row(coalesce((SELECT id AS userid FROM users WHERE p.id = users.wzprofile2), -1)))::jsonb)::text[] as pnames
				FROM games
				JOIN players as p ON p.id = any(games.players)
				WHERE deleted = false AND hidden = false AND calculated = true AND finished = true
				GROUP BY gid
				ORDER BY timestarted`)
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
		var alliance int
		err := rows.Scan(&g.ID, &g.GameTime, &alliance, &players, &teams, &usertype, &playerinfo)
		if err != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": true, "msg": "Database scan error: " + err.Error()})
			return
		}
		g.IsFFA = alliance == 0
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
			if pid == -1 || pid == 370 {
				continue
			}
			var p EloGamePlayer
			p.Usertype = usertype[pslt]
			p.ID = pid
			p.Team = teams[pslt]
			p.EloDiff = 0
			g.Players = append(g.Players, p)
		}
		Games = append(Games, &g)
	}
	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"nocenter": true, "msg": template.HTML("<pre>" + spew.Sdump(Players) + spew.Sdump(Games) + "</pre>")})
	CalcEloForAll(Games, Players)
	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"nocenter": true, "msg": template.HTML("<pre>" + spew.Sdump(Players) + spew.Sdump(Games) + "</pre>")})
	for _, p := range Players {
		log.Printf("Updating player %d: elo %d elo2 %d autowon %d autolost %d autoplayed %d", p.ID, p.Elo, p.Elo2, p.Autoplayed, p.Autowon, p.Autolost)
		tag, derr := dbpool.Exec(context.Background(), "UPDATE players SET elo = $1, elo2 = $2, autoplayed = $3, autowon = $4, autolost = $5 WHERE id = $6",
			p.Elo, p.Elo2, p.Autoplayed, p.Autowon, p.Autolost, p.ID)
		if derr != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Database call error: " + derr.Error()})
			return
		}
		if tag.RowsAffected() != 1 {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Database insert error, rows affected " + string(tag)})
			return
		}
	}
	for _, g := range Games {
		var elodiffs []int
		for _, p := range g.Players {
			elodiffs = append(elodiffs, p.EloDiff)
		}
		log.Printf("Updating game %d: elodiff %v ", g.ID, elodiffs)
		tag, derr := dbpool.Exec(context.Background(), "UPDATE games SET elodiff = $1 WHERE id = $2",
			elodiffs, g.ID)
		if derr != nil {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Database call error: " + derr.Error()})
			return
		}
		if tag.RowsAffected() != 1 {
			basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{"msgred": 1, "msg": "Database insert error, rows affected " + string(tag)})
			return
		}
	}
}
