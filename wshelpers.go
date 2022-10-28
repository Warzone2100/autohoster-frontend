package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 5 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

type WSHub struct {
	clients     map[*WSHubClient]bool
	clientsLock sync.RWMutex
	bcast       chan interface{}
	connect     chan *WSHubClient
	disconnect  chan *WSHubClient
}

type WSHubClient struct {
	hub      *WSHub
	conn     *websocket.Conn
	send     chan interface{}
	username string
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*WSHubClient]bool),
		bcast:      make(chan interface{}),
		connect:    make(chan *WSHubClient),
		disconnect: make(chan *WSHubClient),
	}
}

func (hub *WSHub) Run() {
	for {
		select {
		case client := <-hub.connect:
			hub.clientsLock.Lock()
			hub.clients[client] = true
			hub.clientsLock.Unlock()
			go client.ClientRead()
			go client.ClientWrite()
		case client := <-hub.disconnect:
			hub.clientsLock.Lock()
			if _, ok := hub.clients[client]; ok {
				delete(hub.clients, client)
				close(client.send)
			}
			hub.clientsLock.Unlock()
		case message := <-hub.bcast:
			hub.clientsLock.Lock()
			for client := range hub.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(hub.clients, client)
				}
			}
			hub.clientsLock.Unlock()
		}
	}
}

func (client *WSHubClient) ClientRead() {
	defer func() {
		client.hub.disconnect <- client
		client.conn.Close()
	}()
	for {
		msgtype, msgba, err := client.conn.ReadMessage()
		msg := string(msgba)
		if err != nil || (msgtype == websocket.TextMessage && msg == "{\"action\": \"disconnect\"}") {
			log.Printf("Client [%s] disconnected", client.username)
			break
		}
	}
}

func (client *WSHubClient) ClientWrite() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()
	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			msg, err := json.Marshal(message)
			if err != nil {
				log.Printf("Failed to marshal message to websocket: %s", err.Error())
				return
			}
			w, err := client.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(msg)
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
