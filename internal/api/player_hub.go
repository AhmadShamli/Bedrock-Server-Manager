package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/player"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/raknet"
	"github.com/go-chi/chi/v5"
)

type PlayerHubHandler struct {
	serverHandler *ServerHandler
	playerManager *player.Manager
}

func NewPlayerHubHandler(sh *ServerHandler, pm *player.Manager) *PlayerHubHandler {
	return &PlayerHubHandler{
		serverHandler: sh,
		playerManager: pm,
	}
}

// GetPlayers returns the online players for a server.
func (h *PlayerHubHandler) GetPlayers(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	online := h.playerManager.GetOnlinePlayers(serverID)
	if online == nil {
		online = []player.OnlinePlayer{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"online_players": online,
		"online_count":   len(online),
	})
}

// GetChatFeed returns recent in-game chat messages.
func (h *PlayerHubHandler) GetChatFeed(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	feed := h.playerManager.GetChatFeed(serverID, limit)
	if feed == nil {
		feed = []player.ChatMessage{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(feed)
}

// Broadcast sends an announcement message to the in-game chat via `say` or `tellraw`.
func (h *PlayerHubHandler) Broadcast(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.serverHandler.db.GetServer(r.Context(), serverID)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	var payload struct {
		Message string `json:"message"`
		Target  string `json:"target"` // "" for global broadcast, or gamertag for direct tell
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Message) == "" {
		http.Error(w, `{"error": "Message is required"}`, http.StatusBadRequest)
		return
	}

	cleanMsg := strings.TrimSpace(payload.Message)
	target := strings.TrimSpace(payload.Target)

	var targetSelector string
	var formattedText string

	if target != "" {
		if strings.Contains(target, " ") && !strings.HasPrefix(target, "\"") {
			targetSelector = fmt.Sprintf("%q", target)
		} else {
			targetSelector = target
		}
		formattedText = fmt.Sprintf("§d[BSM Admin -> You] §f%s", cleanMsg)
	} else {
		targetSelector = "@a"
		formattedText = fmt.Sprintf("§6[BSM Broadcast] §f%s", cleanMsg)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]interface{}{
		"rawtext": []map[string]string{
			{"text": formattedText},
		},
	}); err != nil {
		http.Error(w, `{"error": "Failed to encode message"}`, http.StatusInternalServerError)
		return
	}

	cmd := fmt.Sprintf("tellraw %s %s", targetSelector, strings.TrimSpace(buf.String()))

	if err := h.serverHandler.engine.SendConsoleCommand(r.Context(), server, cmd); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to send message: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	if h.playerManager != nil {
		sender := "Server"
		if target != "" {
			sender = fmt.Sprintf("Server -> %s", target)
		}
		h.playerManager.AddChatMessage(serverID, sender, cleanMsg)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

// KickPlayer executes `kick <gamertag> <reason>`.
func (h *PlayerHubHandler) KickPlayer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.serverHandler.db.GetServer(r.Context(), serverID)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	var payload struct {
		Gamertag string `json:"gamertag"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Gamertag == "" {
		http.Error(w, `{"error": "Gamertag is required"}`, http.StatusBadRequest)
		return
	}

	cmd := fmt.Sprintf("kick %s %s", payload.Gamertag, payload.Reason)
	if err := h.serverHandler.engine.SendConsoleCommand(r.Context(), server, cmd); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Kick failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "kicked"})
}

// OpPlayer grants operator status (`op <gamertag>`).
func (h *PlayerHubHandler) OpPlayer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.serverHandler.db.GetServer(r.Context(), serverID)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	var payload struct {
		Gamertag string `json:"gamertag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Gamertag == "" {
		http.Error(w, `{"error": "Gamertag is required"}`, http.StatusBadRequest)
		return
	}

	cmd := fmt.Sprintf("op %s", payload.Gamertag)
	if err := h.serverHandler.engine.SendConsoleCommand(r.Context(), server, cmd); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Op failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "opped"})
}

// DeopPlayer removes operator status (`deop <gamertag>`).
func (h *PlayerHubHandler) DeopPlayer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.serverHandler.db.GetServer(r.Context(), serverID)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	var payload struct {
		Gamertag string `json:"gamertag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Gamertag == "" {
		http.Error(w, `{"error": "Gamertag is required"}`, http.StatusBadRequest)
		return
	}

	cmd := fmt.Sprintf("deop %s", payload.Gamertag)
	if err := h.serverHandler.engine.SendConsoleCommand(r.Context(), server, cmd); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Deop failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deopped"})
}

// Ping performs a RakNet UDP Ping against the server.
func (h *PlayerHubHandler) Ping(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.serverHandler.db.GetServer(r.Context(), serverID)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	res, err := raknet.PingServer("127.0.0.1", server.Port, 1500*time.Millisecond)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"reachable": false,
			"error":     err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"reachable":      true,
		"ping_data":      res,
		"latency_ms":     res.LatencyMs,
		"online_players": res.OnlinePlayers,
		"max_players":    res.MaxPlayers,
		"motd":           res.ServerName,
		"version":        res.Version,
	})
}
