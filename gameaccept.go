package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/gddo/httputil/header"
	"github.com/gorilla/mux"
)

type malformedRequest struct {
	status int
	msg    string
}

func (mr *malformedRequest) Error() string {
	return mr.msg
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	if r.Header.Get("Content-Type") != "" {
		value, _ := header.ParseValueAndParams(r.Header, "Content-Type")
		if value != "application/json" {
			log.Print("Content type: " + r.Header.Get("Content-Type"))
			msg := "Content-Type header is not application/json"
			return &malformedRequest{status: http.StatusUnsupportedMediaType, msg: msg}
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 41943040) // 40 megabytes
	dec := json.NewDecoder(r.Body)
	// dec.DisallowUnknownFields()
	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return &malformedRequest{status: http.StatusBadRequest, msg: msg}

		case errors.Is(err, io.ErrUnexpectedEOF):
			return &malformedRequest{status: http.StatusBadRequest, msg: "Request body contains badly-formed JSON"}

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return &malformedRequest{status: http.StatusBadRequest, msg: msg}

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return &malformedRequest{status: http.StatusBadRequest, msg: msg}

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return &malformedRequest{status: http.StatusBadRequest, msg: msg}

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 40MB"
			return &malformedRequest{status: http.StatusRequestEntityTooLarge, msg: msg}

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		return &malformedRequest{status: http.StatusBadRequest, msg: msg}
	}
	return nil
}

func ParseMilliTimestamp(tm int64) time.Time {
	sec := tm / 1000
	msec := tm % 1000
	return time.Unix(sec, msec*int64(time.Millisecond))
}

type JSONgamePlayer struct {
	Playnum  float64 `json:"playnum"`
	Name     string  `json:"name"`
	Hash     string  `json:"hash"`
	Team     float64 `json:"team"`
	Position float64 `json:"position"`
	Colour   float64 `json:"colour"`
	Score    float64 `json:"score"`
	Kills    float64 `json:"kills"`
	Power    float64 `json:"power"`
	Droid    float64 `json:"droid"`
	//	DroidLoss   float64 `json:"droidLoss"`   deprecated in ver 7
	DroidHp      float64 `json:"hp"` // added in ver 7
	DroidLost    float64 `json:"droidLost"`
	DroidBuilt   float64 `json:"droidBuilt"`
	Rescount     float64 `json:"researchComplete"`
	Struct       float64 `json:"struct"`
	StructBuilt  float64 `json:"structBuilt"`
	StructLost   float64 `json:"structureLost"`
	StructKilled float64 `json:"structureKill"` // added in ver 7
	Usertype     string  `json:"usertype"`
	SummExp      float64 `json:"summExp"` // added in ver 7
	OilRigs      float64 `json:"oilRigs"` // added in ver 7
}

type JSONgameCore struct {
	MapName        string  `json:"mapName"`
	MapHash        string  `json:"mapHash"`
	MultiTechLevel float64 `json:"multiTechLevel"`
	TimeGameEnd    float64 `json:"timeGameEnd"`
	Version        string  `json:"version"`
	AlliancesType  float64 `json:"alliancesType"`
	BaseType       float64 `json:"baseType"`
	PowerType      float64 `json:"powerType"`
	Scavengers     float64 `json:"scavengers"`
	IdleTime       float64 `json:"idleTime"`
	StartDate      float64 `json:"startDate"`
	EndDate        float64 `json:"endDate"`
	GameLimit      float64 `json:"gameLimit"`
	GameDir        string  `json:"gamedir"`
	DebugTriggered bool    `json:"debugtriggered"`
	Mod            string  `json:"mod"`        // added in ver 9
	WaveNumber     float64 `json:"numberWave"` // added in ver 9 set for waves
}

type JSONgame struct {
	JSONversion float64
	GameTime    float64          `json:"gameTime"`
	PlayerData  []JSONgamePlayer `json:"playerData"`
	Game        JSONgameCore     `json:"game"`
}

type JSONgameWithRes struct {
	JSONversion      float64
	GameTime         float64          `json:"gameTime"`
	PlayerData       []JSONgamePlayer `json:"playerData"`
	Game             JSONgameCore     `json:"game"`
	ResearchComplete []interface{}    `json:"researchComplete"`
}

func GameAcceptCreateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("Wrong method on game creating [%s]", r.Method)
		return
	}
	if r.Header.Get("CF-Connecting-IP") != "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	h := JSONgame{}
	err := decodeJSONBody(w, r, &h)
	if err != nil {
		log.Printf("Can not parse form data [%s]", err.Error())
		return
	}
	if h.JSONversion < 4 {
		io.WriteString(w, "-1")
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
	sort.Slice(h.PlayerData, func(i, j int) bool {
		return h.PlayerData[i].Position < h.PlayerData[j].Position
	})
	// spew.Dump(h)

	tx, derr := dbpool.Begin(context.Background())
	if derr != nil {
		log.Printf("Error [%s]", derr.Error())
		io.WriteString(w, "-1")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(context.Background())

	tdbteams := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tdbcolour := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tdbplayers := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	for _, p := range h.PlayerData {
		if p.Name == "Autohoster" && p.Hash == "a0c124533ddcaf5a19cc7d593c33d750680dc428b0021672e0b86a9b0dcfd711" {
			continue
		}
		if p.Name == "" || p.Hash == "" {
			continue
		}
		playerID := 0
		perr := tx.QueryRow(context.Background(), `
			INSERT INTO players as p (name, hash)
			VALUES ($1::text, $2::text)
			ON CONFLICT (hash) DO
				UPDATE SET name = $1::text
			RETURNING id;`, p.Name, p.Hash).Scan(&playerID)
		if perr != nil {
			log.Printf("Error [%s]", perr.Error())
			io.WriteString(w, "-1")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if p.Position < 0 || p.Position > 11 {
			log.Printf("Index of array is not in limits! (%d) [%s] (%d)", int(p.Playnum), p.Name, int(p.Position))
			continue
		}
		tdbplayers[int(p.Position)] = playerID
		tdbteams[int(p.Position)] = int(p.Team)
		tdbcolour[int(p.Position)] = int(p.Colour)
	}
	// spew.Dump(tdbplayers)

	gameid := -1
	starttime := ParseMilliTimestamp(int64(h.Game.StartDate))
	log.Println(starttime.Format("2006-01-02 15:04:05"))
	derr = tx.QueryRow(context.Background(), `INSERT INTO games
		(timestarted, gametime, players, teams, colour, mapname, maphash, powerlevel, baselevel, scavs, alliancetype, version, gamedir) VALUES
		($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id`, starttime.Format("2006-01-02 15:04:05"), h.GameTime, tdbplayers, tdbteams, tdbcolour,
		h.Game.MapName, h.Game.MapHash, h.Game.PowerType, h.Game.BaseType, h.Game.Scavengers != 0, h.Game.AlliancesType, h.Game.Version, h.Game.GameDir).Scan(&gameid)
	if derr != nil {
		log.Printf("Error [%s]", derr.Error())
		io.WriteString(w, "-1")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	derr = tx.Commit(context.Background())
	if err != nil {
		log.Printf("Error [%s]", derr.Error())
		io.WriteString(w, "-1")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	io.WriteString(w, strconv.Itoa(gameid))
	w.WriteHeader(http.StatusOK)
	// WSLobbyNewAutohosterRoom(h, gameid)
}

func GameAcceptFrameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("Wrong method on game creating [%s]", r.Method)
		return
	}
	if r.Header.Get("CF-Connecting-IP") != "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	params := mux.Vars(r)
	gid := params["gid"]
	h := JSONgame{}
	err := decodeJSONBody(w, r, &h)
	if err != nil {
		log.Printf("Can not parse form data [%s]", err.Error())
		return
	}
	if h.JSONversion < 4 {
		io.WriteString(w, "-1")
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
	sort.Slice(h.PlayerData, func(i, j int) bool {
		return h.PlayerData[i].Position < h.PlayerData[j].Position
	})
	tbdkills := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdpower := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdscore := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbddroid := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	//tbddroidloss := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbddroidlost := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbddroidbuilt := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbddroidhp := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdstruct := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdstructbuilt := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdstructlost := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdstructkilled := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdrescount := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdsummexp := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdoilrigs := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	for _, p := range h.PlayerData {
		if p.Name == "Autohoster" && p.Hash == "a0c124533ddcaf5a19cc7d593c33d750680dc428b0021672e0b86a9b0dcfd711" {
			continue
		}
		if p.Name == "" || p.Hash == "" {
			continue
		}
		if p.Position < 0 || p.Position > 11 {
			log.Printf("Index of array is not in limits! (%d) [%s] (%d)", int(p.Playnum), p.Name, int(p.Position))
			continue
		}
		tbdkills[int(p.Position)] = int(p.Kills)
		tbdpower[int(p.Position)] = int(p.Power)
		tbdscore[int(p.Position)] = int(p.Score)
		tbddroid[int(p.Position)] = int(p.Droid)
		//tbddroidloss[int(p.Position)] = int(p.DroidLoss)
		tbddroidlost[int(p.Position)] = int(p.DroidLost)
		tbddroidbuilt[int(p.Position)] = int(p.DroidBuilt)
		tbddroidhp[int(p.Position)] = int(p.DroidHp)
		tbdstruct[int(p.Position)] = int(p.Struct)
		tbdstructbuilt[int(p.Position)] = int(p.StructBuilt)
		tbdstructlost[int(p.Position)] = int(p.StructLost)
		tbdstructkilled[int(p.Position)] = int(p.StructKilled)
		tbdrescount[int(p.Position)] = int(p.Rescount)
		tbdsummexp[int(p.Position)] = int(p.SummExp)
		tbdoilrigs[int(p.Position)] = int(p.OilRigs)
	}
	tag, derr := dbpool.Exec(context.Background(), `
INSERT INTO frames (game, gametime, kills, power, score, droid, droidlost, droidbuilt, struct, structbuilt, structlost, rescount, structkilled, summexp, oilrigs, droidhp)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`, gid, h.GameTime,
		tbdkills, tbdpower, tbdscore, tbddroid, tbddroidlost, tbddroidbuilt, tbdstruct, tbdstructbuilt, tbdstructlost, tbdrescount, tbdstructkilled, tbdsummexp, tbdoilrigs, tbddroidhp)
	if derr != nil {
		log.Printf("Can not upload frame [%s]", derr.Error())
		io.WriteString(w, "err")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() != 1 {
		log.Printf("Can not upload frame [%s]", derr.Error())
		io.WriteString(w, "err")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "ok")
	w.WriteHeader(http.StatusOK)
	// gidn, err := strconv.Atoi(gid)
	// if err != nil {
	// 	return
	// }
	// WSLobbyUpdateAutohosterRoom(h, gidn)
}

func GameAcceptEndHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("Wrong method on game creating [%s]", r.Method)
		return
	}
	if r.Header.Get("CF-Connecting-IP") != "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	params := mux.Vars(r)
	gid := params["gid"]
	h := JSONgameWithRes{}
	err := decodeJSONBody(w, r, &h)
	if err != nil {
		log.Printf("Can not parse form data [%s]", err.Error())
		return
	}
	if h.JSONversion < 4 {
		io.WriteString(w, "-1")
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
	sort.Slice(h.PlayerData, func(i, j int) bool {
		return h.PlayerData[i].Position < h.PlayerData[j].Position
	})
	tbdkills := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdpower := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdscore := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbddroid := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbddroidlost := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbddroidbuilt := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbddroidhp := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdstruct := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdstructbuilt := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdstructlost := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdstructkilled := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdrescount := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdsummexp := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	tbdoilrigs := [11]int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	var tbdusertype [11]string
	gidnum, _ := strconv.Atoi(gid)
	eg := EloGame{
		ID:       gidnum,
		GameTime: int(h.GameTime),
		Base:     int(h.Game.BaseType),
		Players:  []EloGamePlayer{},
	}
	pls := map[int]*Elo{}
	for _, p := range h.PlayerData {
		if p.Name == "Autohoster" && p.Hash == "a0c124533ddcaf5a19cc7d593c33d750680dc428b0021672e0b86a9b0dcfd711" {
			continue
		}
		if p.Name == "" || p.Hash == "" {
			continue
		}
		if p.Position < 0 || p.Position > 11 {
			log.Printf("Index of array is not in limits! (%d) [%s] (%d)", int(p.Playnum), p.Name, int(p.Position))
			continue
		}
		tbdkills[int(p.Position)] = int(p.Kills)
		tbdpower[int(p.Position)] = int(p.Power)
		tbdscore[int(p.Position)] = int(p.Score)
		tbddroid[int(p.Position)] = int(p.Droid)
		tbddroidlost[int(p.Position)] = int(p.DroidLost)
		tbddroidbuilt[int(p.Position)] = int(p.DroidBuilt)
		tbddroidhp[int(p.Position)] = int(p.DroidHp)
		tbdstruct[int(p.Position)] = int(p.Struct)
		tbdstructbuilt[int(p.Position)] = int(p.StructBuilt)
		tbdstructlost[int(p.Position)] = int(p.StructLost)
		tbdstructkilled[int(p.Position)] = int(p.StructKilled)
		tbdrescount[int(p.Position)] = int(p.Rescount)
		tbdsummexp[int(p.Position)] = int(p.SummExp)
		tbdoilrigs[int(p.Position)] = int(p.OilRigs)
		tbdusertype[int(p.Position)] = p.Usertype
		var playerID, elo, elo2, eap, eal, eaw, euid int
		perr := dbpool.QueryRow(context.Background(), `
			SELECT id, elo, elo2, autoplayed, autolost, autowon, coalesce((SELECT id FROM users WHERE players.id = users.wzprofile2), -1) FROM players WHERE hash = $1;`, p.Hash).Scan(&playerID, &elo, &elo2, &eap, &eal, &eaw, &euid)
		if perr != nil {
			log.Printf("Error [%s]", perr.Error())
			io.WriteString(w, "err")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		eg.Players = append(eg.Players, EloGamePlayer{ID: playerID, Team: int(p.Team), Usertype: p.Usertype, EloDiff: 0})
		pls[playerID] = &Elo{ID: playerID, Elo: elo, Elo2: elo2, Autoplayed: eap, Autolost: eal, Autowon: eaw, Userid: euid}
	}
	tbdreslog, _ := json.Marshal(h.ResearchComplete)
	calculating := !h.Game.DebugTriggered
	if calculating {
		CalcElo(&eg, pls)
	}
	var elodiff []int
	for _, eee := range eg.Players {
		elodiff = append(elodiff, eee.EloDiff)
	}
	var ratingdiff []int
	for _, eee := range eg.Players {
		ratingdiff = append(ratingdiff, eee.RatingDiff)
	}
	tag, derr := dbpool.Exec(context.Background(), `
	UPDATE games SET finished = true, timeended = now(), gametime = $1, kills = $2, power = $3, score = $4, units = $5, unitslost = $6, unitbuilt = $7, structs = $8, structbuilt = $9, structurelost = $10, rescount = $11, usertype = $12, researchlog = $13, elodiff = $14, structkilled = $15, summexp = $16, oilrigs = $17, unithp = $18, calculated = $19, debugtriggered = $20, ratingdiff = $21
	WHERE id = $22`, h.GameTime, tbdkills, tbdpower, tbdscore, tbddroid, tbddroidlost, tbddroidbuilt, tbdstruct, tbdstructbuilt, tbdstructlost, tbdrescount, tbdusertype, string(tbdreslog), elodiff, tbdstructkilled, tbdsummexp, tbdoilrigs, tbddroidhp, calculating, h.Game.DebugTriggered, ratingdiff, gid)
	if derr != nil {
		log.Printf("Can not upload frame [%s]", derr.Error())
		io.WriteString(w, "err")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() != 1 {
		log.Printf("Can not upload frame [%s]", derr.Error())
		io.WriteString(w, "err")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if calculating {
		for _, p := range pls {
			log.Printf("Updating player %d: elo %d elo2 %d autowon %d autolost %d autoplayed %d", p.ID, p.Elo, p.Elo2, p.Autoplayed, p.Autowon, p.Autolost)
			tag, derr := dbpool.Exec(context.Background(), "UPDATE players SET elo = $1, elo2 = $2, autoplayed = $3, autowon = $4, autolost = $5 WHERE id = $6",
				p.Elo, p.Elo2, p.Autoplayed, p.Autowon, p.Autolost, p.ID)
			if derr != nil {
				log.Printf("sql error [%s]", derr.Error())
				io.WriteString(w, "err")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if tag.RowsAffected() != 1 {
				log.Printf("Database insert error, rows affected [%s]", string(tag))
				io.WriteString(w, "err")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}
	io.WriteString(w, "ok")
	w.WriteHeader(http.StatusOK)
	// WSLobbyEndAutohosterRoom(gidnum)
}
