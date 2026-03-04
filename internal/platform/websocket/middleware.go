package websocket

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Handler func(*Client)

func (h *Hub) Handler(handler Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		client := NewClient(h, conn)
		h.Register(client)

		go client.WritePump()
		handler(client)
	}
}

func (h *Hub) BroadcastMessage(msgType string, data interface{}) error {
	return h.BroadcastJSON(map[string]interface{}{
		"type": msgType,
		"data": data,
	})
}
