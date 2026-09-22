package api

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Internal web dashboard
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WSMessage represents a client/server WebSocket packet.
type WSMessage struct {
	Type    string `json:"type"` // "history", "log", "command", "error"
	Payload string `json:"payload"`
}

// ConsoleWS handles real-time interactive terminal console streaming.
func (h *ServerHandler) ConsoleWS(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil || server == nil {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ConsoleWS] WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// 1. Send recent 1,000-line history
	history := h.engine.GetRecentLogs(serverID)
	for _, line := range history {
		msg := WSMessage{Type: "history", Payload: line}
		if err := conn.WriteJSON(msg); err != nil {
			return
		}
	}

	// 2. Subscribe to real-time logs
	logChan, unsubscribe := h.engine.SubscribeLogs(serverID)
	defer unsubscribe()

	// 3. Sender goroutine (logs to client)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case line, ok := <-logChan:
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteJSON(WSMessage{Type: "log", Payload: line}); err != nil {
					return
				}
			case <-r.Context().Done():
				return
			}
		}
	}()

	// 4. Receiver loop (commands from client)
	for {
		var wsMsg WSMessage
		if err := conn.ReadJSON(&wsMsg); err != nil {
			break
		}

		if wsMsg.Type == "command" && wsMsg.Payload != "" {
			if err := h.engine.SendConsoleCommand(r.Context(), server, wsMsg.Payload); err != nil {
				_ = conn.WriteJSON(WSMessage{Type: "error", Payload: "Failed to send command: " + err.Error()})
			}
		}
	}

	<-done
}
