package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
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
	if a != 0 {
		return true
	}
	return false
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
	log.Println(count)
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
	r = map[string]interface{}{
		"Rooms": rooms,
		"Motd":  string(motd[:]),
	}
	return r
}

func lobbyHandler(w http.ResponseWriter, r *http.Request) {
	basicLayoutLookupRespond("lobby", w, r, LobbyLookup())
}
