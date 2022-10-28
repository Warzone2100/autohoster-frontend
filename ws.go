package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024 * 16,
	}
)

func APIWSHub(hub *WSHub, w http.ResponseWriter, r *http.Request) {
	username := sessionGetUsername(r)
	if username == "" {
		respondWithUnauthorized(w, r)
		return
	}
	hub.clientsLock.RLock()
	var d *WSHubClient
	d = nil
	for i := range hub.clients {
		if i.username == username {
			d = i
			break
		}
	}
	hub.clientsLock.RUnlock()
	if d != nil {
		hub.disconnect <- d
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to accept lobby listener websocket: %s", err.Error())
		return
	}
	client := &WSHubClient{hub: hub, conn: conn, send: make(chan interface{}), username: username}
	client.hub.connect <- client
}

func WSLobbyUpdateLobby(lobby map[string]interface{}) {
	LobbyWSHub.bcast <- map[string]interface{}{
		"type": "LobbyUpdate",
		"data": lobby,
	}
}

func WSLobbyNewAutohosterRoom(room JSONgame, dbgid int) {
	in := layouts.Lookup("roomAutohoster")
	if in == nil {
		log.Print("Failed to find layout [roomAutohoster]!")
		return
	}
	LobbyWSHub.bcast <- map[string]interface{}{
		"type": "AutohosterRoomNew",
		"gid":  dbgid,
		"data": basicLayoutLookupExecuteAnonymus(in, room),
	}
}

func WSLobbyUpdateAutohosterRoom(room JSONgame, dbgid int) {
	in := layouts.Lookup("roomAutohoster")
	if in == nil {
		log.Print("Failed to find layout [roomAutohoster]!")
		return
	}
	LobbyWSHub.bcast <- map[string]interface{}{
		"type": "AutohosterRoomUpdate",
		"gid":  dbgid,
		"data": basicLayoutLookupExecuteAnonymus(in, room),
	}
}

func WSLobbyEndAutohosterRoom(dbgid int) {
	LobbyWSHub.bcast <- map[string]interface{}{
		"type": "AutohosterRoomEnd",
		"gid":  dbgid,
	}
}
