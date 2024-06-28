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
	client := &WSHubClient{hub: hub, conn: conn, send: make(chan any), username: username}
	client.hub.connect <- client
}

func WSLobbyUpdateLobby(lobby map[string]any) {
	LobbyWSHub.bcast <- map[string]any{
		"type": "LobbyUpdate",
		"data": lobby,
	}
}
