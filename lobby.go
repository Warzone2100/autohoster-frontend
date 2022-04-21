package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type LobbyRoom struct {
	StructVersion  uint32
	GameName       [64]byte
	DW             [2]uint32
	HostIP         [40]byte
	MaxPlayers     uint32
	CurrentPlayers uint32
	DWFlags        [4]uint32
	SecHost        [2][40]byte
	Extra          [159]byte
	MapName        [40]byte
	HostName       [40]byte
	Version        [64]byte
	Mods           [255]byte
	VersionMajor   uint32
	VersionMinor   uint32
	Private        uint32
	Pure           uint32
	ModsCount      uint32
	GameID         uint32
	Limits         uint32
	Future1        uint32
	Future2        uint32
}

type LobbyRoomPretty struct {
	GameID         uint32
	GameName       string
	MapName        string
	HostName       string
	Version        string
	Private        bool
	Pure           bool
	MaxPlayers     uint32
	CurrentPlayers uint32
}

func btoi(a uint32) bool {
	return a != 0
}

var lobbyIgnoreIPS []string

func loadLobbyIgnores(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	ignored := []string{}
	for scanner.Scan() {
		ignored = append(ignored, scanner.Text())
	}
	lobbyIgnoreIPS = ignored
	return scanner.Err()
}

func LobbyLookup() map[string]interface{} {
	conn, connerr := net.Dial("tcp", "lobby.wz2100.net:9990")
	if connerr != nil {
		log.Println(connerr.Error())
		return map[string]interface{}{}
	}
	defer conn.Close()
	fmt.Fprintf(conn, "list\n")
	var count uint32
	err := binary.Read(conn, binary.BigEndian, &count)
	if err != nil {
		fmt.Println(err.Error())
		return map[string]interface{}{}
	}
	// log.Println(count)
	var rooms []LobbyRoomPretty
	for i := uint32(0); i < count; i++ {
		var room LobbyRoom
		err := binary.Read(conn, binary.BigEndian, &room)
		if err != nil {
			fmt.Println(err.Error())
			return map[string]interface{}{}
		}
		// log.Println(string(room.GameName[:]), string(room.HostName[:]))
		roomp := LobbyRoomPretty{
			room.GameID,
			string(room.GameName[:bytes.IndexByte(room.GameName[:], 0)]),
			string(room.MapName[:bytes.IndexByte(room.MapName[:], 0)]),
			string(room.HostName[:bytes.IndexByte(room.HostName[:], 0)]),
			string(room.Version[:bytes.IndexByte(room.Version[:], 0)]),
			btoi(room.Private),
			btoi(room.Pure),
			room.MaxPlayers,
			room.CurrentPlayers,
		}
		rooms = append(rooms, roomp)
	}
	var lobbyCode uint32
	err = binary.Read(conn, binary.BigEndian, &lobbyCode)
	if err != nil {
		fmt.Println(err.Error())
		return map[string]interface{}{}
	}
	var motdlen uint32
	err = binary.Read(conn, binary.BigEndian, &motdlen)
	if err != nil {
		fmt.Println(err.Error())
		return map[string]interface{}{}
	}
	motd := make([]byte, motdlen)
	err = binary.Read(conn, binary.BigEndian, &motd)
	if err != nil {
		fmt.Println(err.Error())
		return map[string]interface{}{}
	}
	var r map[string]interface{}

	var lobbyFlags uint32
	err = binary.Read(conn, binary.BigEndian, &lobbyFlags)
	if err != nil {

	} else {
		if (lobbyFlags & 1) == 1 {
			rooms = []LobbyRoomPretty{}
		}
		err := binary.Read(conn, binary.BigEndian, &count)
		if err != nil {
			fmt.Println(err.Error())
			return map[string]interface{}{}
		}
		for i := uint32(0); i < count; i++ {
			var room LobbyRoom
			err := binary.Read(conn, binary.BigEndian, &room)
			if err != nil {
				fmt.Println(err.Error())
				return map[string]interface{}{}
			}
			for _, i := range lobbyIgnoreIPS {
				s := strings.Split(i, " ")
				if len(s) > 1 && isMatch(string(room.HostIP[:]), s[1]) {
					continue
				}
			}
			roomp := LobbyRoomPretty{
				room.GameID,
				string(room.GameName[:bytes.IndexByte(room.GameName[:], 0)]),
				string(room.MapName[:bytes.IndexByte(room.MapName[:], 0)]),
				string(room.HostName[:bytes.IndexByte(room.HostName[:], 0)]),
				string(room.Version[:bytes.IndexByte(room.Version[:], 0)]),
				btoi(room.Private),
				btoi(room.Pure),
				room.MaxPlayers,
				room.CurrentPlayers,
			}
			rooms = append(rooms, roomp)
		}
	}
	r = map[string]interface{}{
		"Rooms": rooms,
		"Motd":  string(motd[:]),
	}
	return r
}

func lobbyPooler() {
	for {
		if len(LobbyWSHub.clients) != 0 {
			WSLobbyUpdateLobby(LobbyLookup())
		}
		time.Sleep(1 * time.Second)
	}
}

func lobbyHandler(w http.ResponseWriter, r *http.Request) {
	s, reqres := RequestHosters()
	var rooms []interface{}
	if s {
		json.Unmarshal([]byte(reqres), &rooms)
	}
	basicLayoutLookupRespond("lobby", w, r, map[string]interface{}{"Lobby": LobbyLookup(), "Hoster": rooms})
}

func isMatch(s string, p string) bool {
	runeInput := []rune(s)
	runePattern := []rune(p)
	lenInput := len(runeInput)
	lenPattern := len(runePattern)
	isMatchingMatrix := make([][]bool, lenInput+1)
	for i := range isMatchingMatrix {
		isMatchingMatrix[i] = make([]bool, lenPattern+1)
	}
	isMatchingMatrix[0][0] = true
	for i := 1; i < lenInput; i++ {
		isMatchingMatrix[i][0] = false
	}
	if lenPattern > 0 {
		if runePattern[0] == '*' {
			isMatchingMatrix[0][1] = true
		}
	}
	for j := 2; j <= lenPattern; j++ {
		if runePattern[j-1] == '*' {
			isMatchingMatrix[0][j] = isMatchingMatrix[0][j-1]
		}
	}
	for i := 1; i <= lenInput; i++ {
		for j := 1; j <= lenPattern; j++ {
			if runePattern[j-1] == '*' {
				isMatchingMatrix[i][j] = isMatchingMatrix[i-1][j] || isMatchingMatrix[i][j-1]
			}
			if runePattern[j-1] == '?' || runeInput[i-1] == runePattern[j-1] {
				isMatchingMatrix[i][j] = isMatchingMatrix[i-1][j-1]
			}
		}
	}
	return isMatchingMatrix[lenInput][lenPattern]
}
