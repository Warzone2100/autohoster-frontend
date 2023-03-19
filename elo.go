package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync/atomic"

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
	Userid     int `json:"userid"`
	Timeplayed int `json:"timeplayed"`
}

type EloGamePlayer struct {
	ID         int
	Team       int
	Usertype   string
	EloDiff    int
	RatingDiff int
}

type EloGame struct {
	ID          int
	GameTime    int
	Base        int
	IsFFA       bool
	Players     []EloGamePlayer
	Timestarted int
	Mod         string
}

func EloDiff(K, e1, e2 int) float64 {
	return float64(K) * (1 / (1 + math.Pow(float64(10), float64(e1-e2)/float64(400))))
}

func CalcElo(G *EloGame, P map[int]*Elo) (calclog string) {
	calclog += fmt.Sprintf("Processing game %d\n", G.ID)
	if G.GameTime < 1000*60*2 {
		calclog += "Game is too fast to be calculated\n"
		return
	}
	for _, p := range G.Players {
		P[p.ID].Autoplayed++
		P[p.ID].Timeplayed += G.GameTime / 1000
	}
	if G.Mod == "masterbal" && G.Timestarted > 1666528200 {
		calclog += "Game is played with draft balance\n"
		return
	}
	if len(G.Players) == 1 {
		calclog += "Only one player found, ignoring\n"
		return
	}
	if len(G.Players) == 2 && G.Players[0].ID == G.Players[1].ID {
		// P[G.Players[0].ID].Elo -= 40
		// P[G.Players[0].ID].Autolost += 2
		// G.Players[0].EloDiff = -20
		// G.Players[1].EloDiff = -20
		// if P[G.Players[0].ID].Userid != -1 && P[G.Players[0].ID].Userid != 0 {
		// 	P[G.Players[0].ID].Elo2 -= 40
		// }
		calclog += "Duel with only one profile detected, bonk does not apply\n"
		return
	}
	Team1ID := []int{}
	Team2ID := []int{}
	if !G.IsFFA {
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
			calclog += "FFA game with not 2 players?!\n"
			log.Printf("FFA game with not 2 players: %d", G.ID)
			return
		}
	}
	if len(Team1ID) != len(Team2ID) {
		log.Printf("Incorrect length: %d", G.ID)
		return
	}
	for _, nid := range Team1ID {
		for _, nnid := range Team2ID {
			if nid == nnid {
				calclog += "Game is sus, one player in both teams?!"
				return
			}
		}
	}
	Team1EloSum := 0
	Team2EloSum := 0
	for _, p := range Team1ID {
		Team1EloSum += P[p].Elo
	}
	for _, p := range Team2ID {
		Team2EloSum += P[p].Elo
	}
	calclog += fmt.Sprintf("Elo sum: %d %d\n", Team1EloSum, Team2EloSum)
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
	_ = Team1Won
	_ = Team2Won
	Team1EloAvg := Team1EloSum / len(Team1ID)
	Team2EloAvg := Team2EloSum / len(Team2ID)
	calclog += fmt.Sprintf("Elo avg: %d %d\n", Team1EloAvg, Team2EloAvg)
	K := 20
	diff1 := int(EloDiff(K, Team1EloAvg, Team2EloAvg))
	diff2 := int(EloDiff(K, Team2EloAvg, Team1EloAvg))
	calclog += fmt.Sprintf("Elo diff: %d %d\n", diff1, diff2)
	var Additive int
	var Timeitive int
	if G.Players[0].Usertype == "winner" {
		Additive = diff1
		Timeitive = diff2
	} else {
		Additive = diff2
		Timeitive = diff1
	}
	calclog += fmt.Sprintf("Additive: %d\n", Additive)
	calclog += fmt.Sprintf("Timeitive: %d\n", Timeitive)
	calcelo2 := true
	for _, p := range G.Players {
		if P[p.ID].Userid == -1 || P[p.ID].Userid == 0 {
			calcelo2 = false
			break
		}
	}
	calclog += fmt.Sprintf("Elo 2: %s\n", spew.Sprint(calcelo2))
	var RAdditive int
	var RTimeitive int
	if calcelo2 {
		Team1RatingSum := 0
		Team2RatingSum := 0
		for _, p := range Team1ID {
			Team1RatingSum += P[p].Elo2
		}
		for _, p := range Team2ID {
			Team2RatingSum += P[p].Elo2
		}
		calclog += fmt.Sprintf("Rating sum: %d %d\n", Team1RatingSum, Team2RatingSum)
		Team1RatingAvg := Team1RatingSum / len(Team1ID)
		Team2RatingAvg := Team2RatingSum / len(Team2ID)
		calclog += fmt.Sprintf("Rating avg: %d %d\n", Team1RatingAvg, Team2RatingAvg)
		rdiff1 := int(EloDiff(K, Team1RatingAvg, Team2RatingAvg))
		rdiff2 := int(EloDiff(K, Team2RatingAvg, Team1RatingAvg))
		calclog += fmt.Sprintf("Rating diff: %d %d\n", rdiff1, rdiff2)
		if G.Players[0].Usertype == "winner" {
			RAdditive = rdiff1
			RTimeitive = rdiff2
		} else {
			RAdditive = rdiff2
			RTimeitive = rdiff1
		}
		calclog += fmt.Sprintf("Rating additive: %d\n", RAdditive)
		calclog += fmt.Sprintf("Rating timitive: %d\n", RTimeitive)
	}
	for pi, p := range G.Players {
		calclog += fmt.Sprintf("Updating player %d (id %d)\n", pi, p.ID)
		if p.Usertype == "winner" {
			P[p.ID].Autowon++
			P[p.ID].Elo += Additive
			G.Players[pi].EloDiff = Additive
			if calcelo2 {
				P[p.ID].Elo2 += RAdditive
				G.Players[pi].RatingDiff = RAdditive
				calclog += fmt.Sprintf("Player %d won %d elo and %d rating\n", pi, Additive, RAdditive)
			} else {
				calclog += fmt.Sprintf("Player %d won %d elo\n", pi, Additive)
			}
		} else if p.Usertype == "loser" {
			P[p.ID].Autolost++
			P[p.ID].Elo -= Additive
			G.Players[pi].EloDiff = -Additive
			// P[p.ID].Elo += int(math.Round((float64(Timeitive) / float64(60)) * (float64(G.GameTime) / (float64(90000) - 10))))
			if calcelo2 {
				P[p.ID].Elo2 -= RAdditive
				// P[p.ID].Elo2 += int(math.Round((float64(RTimeitive) / float64(60)) * (float64(G.GameTime) / (float64(90000) - 10))))
				G.Players[pi].RatingDiff = -RAdditive
				// G.Players[pi].EloDiff += int(math.Round((float64(RTimeitive) / float64(60)) * (float64(G.GameTime) / (float64(90000) - 10))))
				calclog += fmt.Sprintf("Player %d lost %d elo and %d rating\n", pi, Additive, RAdditive)
			} else {
				// G.Players[pi].EloDiff += int(math.Round((float64(Timeitive) / float64(60)) * (float64(G.GameTime) / (float64(90000) - 10))))
				calclog += fmt.Sprintf("Player %d lost %d elo\n", pi, Additive)
			}
		}
	}
	return calclog
}

func CalcEloForAll(G []*EloGame, P map[int]*Elo) (calclog string) {
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
		p.Timeplayed = 0
	}
	for gamei := range G {
		calclog += CalcElo(G[gamei], P)
	}
	return calclog
}

var isEloRecalculating atomic.Bool

func EloRecalcHandler(w http.ResponseWriter, _ *http.Request) {
	if !isEloRecalculating.CompareAndSwap(false, true) {
		w.Write([]byte("Already recalculating\n\n"))
		return
	}
	rows, derr := dbpool.Query(context.Background(), `
				SELECT
					games.id as gid, gametime, alliancetype,
					players, teams, usertype,
					array_agg(to_json(p)::jsonb || json_build_object('userid', coalesce((SELECT id AS userid FROM users WHERE p.id = users.wzprofile2), -1))::jsonb)::text[] as pnames,
					EXTRACT(EPOCH FROM timestarted)::int, mod
				FROM games
				JOIN players as p ON p.id = any(games.players)
				WHERE deleted = false AND hidden = false AND calculated = true AND finished = true
				GROUP BY gid
				ORDER BY timestarted`)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			w.Write([]byte("No games\n\n"))
		} else {
			w.Write([]byte("Database query error: " + derr.Error() + "\n\n"))
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
		err := rows.Scan(&g.ID, &g.GameTime, &alliance, &players, &teams, &usertype, &playerinfo, &g.Timestarted, &g.Mod)
		if err != nil {
			w.Write([]byte("Database scan error: " + err.Error() + "\n\n"))
			return
		}
		g.IsFFA = alliance == 0
		for _, pv := range playerinfo {
			var e Elo
			err := json.Unmarshal([]byte(pv), &e)
			if err != nil {
				w.Write([]byte("JSON error: " + err.Error() + "\n\n"))
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
			p.RatingDiff = 0
			g.Players = append(g.Players, p)
		}
		Games = append(Games, &g)
	}
	calclog := CalcEloForAll(Games, Players)
	log.Println("Preparing batch update")
	b := pgx.Batch{}
	log.Printf("Generating %v player updates", len(Players))
	for _, p := range Players {
		b.Queue("UPDATE players SET elo = $1, elo2 = $2, autoplayed = $3, autowon = $4, autolost = $5, timeplayed = $6 WHERE id = $7",
			p.Elo, p.Elo2, p.Autoplayed, p.Autowon, p.Autolost, p.Timeplayed, p.ID)
	}
	log.Printf("Generating %v game updates", len(Games))
	for _, g := range Games {
		var elodiffs []int
		for _, p := range g.Players {
			elodiffs = append(elodiffs, p.EloDiff)
		}
		var ratingdiffs []int
		for _, p := range g.Players {
			ratingdiffs = append(ratingdiffs, p.RatingDiff)
		}
		b.Queue("UPDATE games SET elodiff = $1, ratingdiff = $2 WHERE id = $3", elodiffs, ratingdiffs, g.ID)
	}
	log.Println("Resetting all residual values")
	_, err := dbpool.Exec(context.Background(), "update games set ratingdiff = '{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}', elodiff = '{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}';")
	if err != nil {
		log.Println(err)
	}
	_, err = dbpool.Exec(context.Background(), "update players set autoplayed = 0, autowon = 0, autolost = 0, elo = 1400, elo2 = 0;")
	if err != nil {
		log.Println(err)
	}
	log.Printf("Batch executing %v", b.Len())
	br := dbpool.SendBatch(context.Background(), &b)
	log.Printf("Batch executed, checking results")
	for i := 0; i < b.Len(); i++ {
		_, err := br.Exec()
		if err != nil {
			log.Println(err)
			break
		}
	}
	br.Close()
	isEloRecalculating.Store(false)
	log.Println("Elo recalc done")
	w.Write([]byte(calclog))
}
