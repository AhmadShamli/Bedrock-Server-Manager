package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/configfile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
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

	// Read permissions.json for OP status
	permsMap := make(map[string]string)
	if h.serverHandler != nil && h.serverHandler.dataDir != "" {
		if permPath, err := configfile.SafePath(h.serverHandler.dataDir, serverID, "permissions.json"); err == nil {
			if perms, err := configfile.ReadPermissions(permPath); err == nil {
				for _, p := range perms {
					if p.XUID != "" {
						permsMap[p.XUID] = p.Permission
						permsMap[strings.ToLower(p.XUID)] = p.Permission
					}
				}
			}
		}
	}

	for i := range online {
		perm := permsMap[online[i].XUID]
		if perm == "" {
			perm = permsMap[strings.ToLower(online[i].Gamertag)]
		}
		if perm != "" {
			online[i].Permission = perm
			online[i].IsOp = (perm == "operator")
		} else if online[i].Permission == "" {
			online[i].Permission = "member"
			online[i].IsOp = false
		}
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

	if h.playerManager != nil {
		h.playerManager.SetPlayerOp(serverID, payload.Gamertag, true)
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

	if h.playerManager != nil {
		h.playerManager.SetPlayerOp(serverID, payload.Gamertag, false)
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

// BanPlayerRequest represents the payload to ban a player.
type BanPlayerRequest struct {
	Gamertag  string `json:"gamertag"`
	XUID      string `json:"xuid"`
	Reason    string `json:"reason"`
	Scope     string `json:"scope"` // "instance" (default) or "global"
	BanIP     bool   `json:"ban_ip"`
	IPAddress string `json:"ip_address"`
}

// ActivePlayerInfo represents an online player across any server instance.
type ActivePlayerInfo struct {
	ServerID   string    `json:"server_id"`
	ServerName string    `json:"server_name"`
	Gamertag   string    `json:"gamertag"`
	XUID       string    `json:"xuid"`
	JoinedAt   time.Time `json:"joined_at"`
	Permission string    `json:"permission,omitempty"`
	IsOp       bool      `json:"is_op"`
}

// GetAllActivePlayers returns real-time online players across all servers.
func (h *PlayerHubHandler) GetAllActivePlayers(w http.ResponseWriter, r *http.Request) {
	online := h.playerManager.GetAllOnlinePlayers()
	serverNameMap := make(map[string]string)
	if h.serverHandler != nil && h.serverHandler.db != nil {
		if servers, err := h.serverHandler.db.ListServers(r.Context()); err == nil {
			for _, s := range servers {
				serverNameMap[s.ID] = s.Name
			}
		}
	}

	result := make([]ActivePlayerInfo, 0, len(online))
	for _, p := range online {
		sName := serverNameMap[p.ServerID]
		if sName == "" {
			sName = p.ServerID
		}
		result = append(result, ActivePlayerInfo{
			ServerID:   p.ServerID,
			ServerName: sName,
			Gamertag:   p.Gamertag,
			XUID:       p.XUID,
			JoinedAt:   p.JoinedAt,
			Permission: p.Permission,
			IsOp:       p.IsOp,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// BanPlayer bans a player on the current instance or globally across all instances.
func (h *PlayerHubHandler) BanPlayer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	var server *models.Server
	if serverID != "" {
		s, err := h.serverHandler.db.GetServer(r.Context(), serverID)
		if err != nil {
			http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
			return
		}
		server = s
	}

	var payload BanPlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Gamertag) == "" {
		http.Error(w, `{"error": "Gamertag is required"}`, http.StatusBadRequest)
		return
	}

	claims := GetUserClaims(r)
	bannedBy := "admin"
	if claims != nil && claims.Username != "" {
		bannedBy = claims.Username
	}

	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		reason = "Banned by administrator"
	}

	isGlobal := serverID == "" || strings.EqualFold(strings.TrimSpace(payload.Scope), "global")
	var banServerID *string
	if !isGlobal && serverID != "" {
		banServerID = &serverID
	}

	ban := &models.BannedPlayer{
		ServerID: banServerID,
		Gamertag: strings.TrimSpace(payload.Gamertag),
		XUID:     strings.TrimSpace(payload.XUID),
		Reason:   reason,
		BannedBy: bannedBy,
	}

	if err := h.serverHandler.db.CreatePlayerBan(r.Context(), ban); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to create player ban: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// 1. Kick player from current server if running
	if server != nil && server.Status == models.ServerStatusRunning {
		_ = h.serverHandler.engine.SendConsoleCommand(r.Context(), server, fmt.Sprintf("kick %s %s", payload.Gamertag, reason))
	}

	// 2. Remove player from allowlist.json on current server
	if serverID != "" && h.serverHandler.dataDir != "" {
		if alPath, err := configfile.SafePath(h.serverHandler.dataDir, serverID, "allowlist.json"); err == nil {
			if al, err := configfile.ReadAllowlist(alPath); err == nil {
				filtered := make([]configfile.AllowlistEntry, 0, len(al))
				for _, entry := range al {
					if !strings.EqualFold(entry.Name, payload.Gamertag) && (payload.XUID == "" || entry.XUID != payload.XUID) {
						filtered = append(filtered, entry)
					}
				}
				if len(filtered) != len(al) {
					_ = configfile.WriteAllowlist(alPath, filtered)
					if server != nil && server.Status == models.ServerStatusRunning {
						_ = h.serverHandler.engine.SendConsoleCommand(r.Context(), server, "allowlist reload")
					}
				}
			}
		}
	}

	// 3. If global scope, kick across all other running servers and remove from all allowlists & global_players
	if isGlobal {
		_ = h.serverHandler.db.DeleteGlobalPlayerByName(r.Context(), payload.Gamertag)
		allServers, err := h.serverHandler.db.ListServers(r.Context())
		if err == nil {
			for _, s := range allServers {
				if s.ID == serverID {
					continue
				}
				if s.Status == models.ServerStatusRunning {
					_ = h.serverHandler.engine.SendConsoleCommand(r.Context(), &s, fmt.Sprintf("kick %s %s", payload.Gamertag, reason))
				}
				if h.serverHandler.dataDir != "" {
					if sPath, err := configfile.SafePath(h.serverHandler.dataDir, s.ID, "allowlist.json"); err == nil {
						if sal, err := configfile.ReadAllowlist(sPath); err == nil {
							sFiltered := make([]configfile.AllowlistEntry, 0, len(sal))
							for _, se := range sal {
								if !strings.EqualFold(se.Name, payload.Gamertag) && (payload.XUID == "" || se.XUID != payload.XUID) {
									sFiltered = append(sFiltered, se)
								}
							}
							if len(sFiltered) != len(sal) {
								_ = configfile.WriteAllowlist(sPath, sFiltered)
								if s.Status == models.ServerStatusRunning {
									_ = h.serverHandler.engine.SendConsoleCommand(r.Context(), &s, "allowlist reload")
								}
							}
						}
					}
				}
			}
		}
	}

	// 4. Optionally ban IP in Port Gate
	ipBanned := false
	targetIP := strings.TrimSpace(payload.IPAddress)
	if payload.BanIP {
		if targetIP == "" {
			leases, err := h.serverHandler.db.ListActiveLeases(r.Context(), serverID)
			if err == nil {
				for _, l := range leases {
					if strings.EqualFold(l.Gamertag, payload.Gamertag) {
						targetIP = l.IPAddress
						break
					}
				}
			}
		}

		if targetIP != "" {
			pgBan := &models.PortGateBanRule{
				ServerID:   banServerID,
				IPOrSubnet: targetIP,
				Reason:     fmt.Sprintf("Player ban: %s (%s)", payload.Gamertag, reason),
				BannedBy:   bannedBy,
			}
			if err := h.serverHandler.db.CreatePortGateBan(r.Context(), pgBan); err == nil {
				ipBanned = true
				_, _ = h.serverHandler.db.RevokeMatchingLeases(r.Context(), banServerID, targetIP)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "banned",
		"gamertag":  payload.Gamertag,
		"scope":     payload.Scope,
		"ip_banned": ipBanned,
		"banned_ip": targetIP,
		"ban_id":    ban.ID,
	})
}

// ListBans returns active player bans for this server (instance + global).
func (h *PlayerHubHandler) ListBans(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	bans, err := h.serverHandler.db.ListPlayerBans(r.Context(), &serverID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to list bans: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bans)
}

// UnbanPlayer unbans a player by gamertag or ID.
func (h *PlayerHubHandler) UnbanPlayer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	var payload struct {
		ID       int64  `json:"id"`
		Gamertag string `json:"gamertag"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	if payload.ID > 0 {
		_ = h.serverHandler.db.DeletePlayerBan(r.Context(), payload.ID)
	} else if payload.Gamertag != "" {
		_ = h.serverHandler.db.DeletePlayerBanByGamertag(r.Context(), &serverID, payload.Gamertag)
	} else {
		http.Error(w, `{"error": "ID or gamertag is required"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "unbanned"})
}

// ListAllBans returns all player bans across the entire manager.
func (h *PlayerHubHandler) ListAllBans(w http.ResponseWriter, r *http.Request) {
	bans, err := h.serverHandler.db.ListPlayerBans(r.Context(), nil)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to list all bans: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bans)
}

// DeleteBan deletes a player ban rule by ID.
func (h *PlayerHubHandler) DeleteBan(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, `{"error": "Invalid ban ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.serverHandler.db.DeletePlayerBan(r.Context(), id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to delete ban: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
