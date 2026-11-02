package reload

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	WebSocketPath = "/__andurel/events"

	writeWait = 10 * time.Second

	pongWait = 60 * time.Second

	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketHandler struct {
	broadcaster *Broadcaster
}

func NewWebSocketHandler(broadcaster *Broadcaster) *WebSocketHandler {
	return &WebSocketHandler{
		broadcaster: broadcaster,
	}
}

func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer func() { _ = conn.Close() }() // Best-effort connection cleanup.

	reloadCh := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(reloadCh)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer cancel()
		if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			return
		}
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(pongWait))
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reloadCh:
			if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte("r")); err != nil {
				return
			}
		case <-ticker.C:
			if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
