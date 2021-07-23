package main

import (
	_ "log"
	"net/http"
	_ "strconv"
	"time"

	"github.com/gorilla/websocket"
)

func APIwsWatch(w http.ResponseWriter, r *http.Request) {
	// ws, err := upgrader.Upgrade(w, r, nil)
	// if err != nil {
	// 	if _, ok := err.(websocket.HandshakeError); !ok {
	// 		log.Println(err)
	// 	}
	// 	return
	// }
	// 
	// var lastMod time.Time
	// if n, err := strconv.ParseInt(r.FormValue("lastMod"), 16, 64); err == nil {
	// 	lastMod = time.Unix(0, n)
	// }
	// 
	// go writer(ws, lastMod)
	// reader(ws)
}

func writer(ws *websocket.Conn, lastMod time.Time) {
	// lastError := ""
	// pingTicker := time.NewTicker(pingPeriod)
	// fileTicker := time.NewTicker(filePeriod)
	// defer func() {
	// 	pingTicker.Stop()
	// 	fileTicker.Stop()
	// 	ws.Close()
	// }()
	// for {
	// 	select {
	// 	case <-fileTicker.C:
	// 		var p []byte
	// 		var err error
	// 
	// 		p, lastMod, err = readFileIfModified(lastMod)
	// 
	// 		if err != nil {
	// 			if s := err.Error(); s != lastError {
	// 				lastError = s
	// 				p = []byte(lastError)
	// 			}
	// 		} else {
	// 			lastError = ""
	// 		}
	// 
	// 		if p != nil {
	// 			ws.SetWriteDeadline(time.Now().Add(writeWait))
	// 			if err := ws.WriteMessage(websocket.TextMessage, p); err != nil {
	// 				return
	// 			}
	// 		}
	// 	case <-pingTicker.C:
	// 		ws.SetWriteDeadline(time.Now().Add(writeWait))
	// 		if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
	// 			return
	// 		}
	// 	}
	// }
}
