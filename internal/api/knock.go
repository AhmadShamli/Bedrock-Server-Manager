package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/go-chi/chi/v5"
)

type KnockHandler struct {
	db          *database.ManagerDB
	rateLimiter *auth.RateLimiter
	pepper      string
}

func NewKnockHandler(db *database.ManagerDB, rateLimiter *auth.RateLimiter, pepper string) *KnockHandler {
	return &KnockHandler{
		db:          db,
		rateLimiter: rateLimiter,
		pepper:      pepper,
	}
}

// KnockConfigResponse conveys public port gate configuration to visitors.
type KnockConfigResponse struct {
	ServerID                 string `json:"server_id"`
	ServerName               string `json:"server_name"`
	Port                     int    `json:"port"`
	PortGateEnabled          bool   `json:"port_gate_enabled"`
	PortGateMode             string `json:"port_gate_mode"`
	HeartbeatIntervalSeconds int    `json:"heartbeat_interval_seconds"`
}

// GetConfig returns public knock settings for a given server.
func (h *KnockHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	// Default heartbeat interval is 10s, configurable in system_settings
	hbSec := 10
	if val, err := h.db.GetSetting(r.Context(), "heartbeat_interval_seconds"); err == nil && val != "" {
		if s, err := strconv.Atoi(val); err == nil && s > 0 {
			hbSec = s
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(KnockConfigResponse{
		ServerID:                 server.ID,
		ServerName:               server.Name,
		Port:                     server.Port,
		PortGateEnabled:          server.PortGateEnabled,
		PortGateMode:             server.PortGateMode,
		HeartbeatIntervalSeconds: hbSec,
	})
}

// KnockRequest payload.
type KnockRequest struct {
	Gamertag   string `json:"gamertag"`
	Passphrase string `json:"passphrase"`
}

// Knock handles port-knocking authorization.
func (h *KnockHandler) Knock(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	if !server.PortGateEnabled {
		http.Error(w, `{"error": "Port gating is not enabled on this server"}`, http.StatusBadRequest)
		return
	}

	clientIP := GetClientIP(r).String()
	now := time.Now().UTC()

	// Rate limiter check
	rateCheck := h.rateLimiter.CheckKnockAttempt(clientIP, now)
	if rateCheck.Blocked {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":               rateCheck.Reason,
			"retry_after_seconds": rateCheck.RetryAfterSeconds,
		})
		return
	}

	var req KnockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	var matchingKey *models.PortGateKey
	leaseDuration := time.Duration(server.PortGateTimeout) * time.Second

	// Validate Passphrase if mode is 'passphrase' or 'combined'
	if server.PortGateMode == models.PortGateModePassphrase || server.PortGateMode == models.PortGateModeCombined {
		if req.Passphrase == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Passphrase is required"})
			return
		}

		keys, err := h.db.ListPortGateKeys(r.Context(), &server.ID)
		if err != nil {
			http.Error(w, `{"error": "Failed to query access keys"}`, http.StatusInternalServerError)
			return
		}

		for _, k := range keys {
			if !k.IsActive {
				continue
			}
			if k.MaxUses > 0 && k.UsedCount >= k.MaxUses {
				continue
			}
			if k.ExpiresAt != nil && now.After(*k.ExpiresAt) {
				continue
			}
			if auth.VerifyAccessKey(h.pepper, req.Passphrase, k.KeyHash) {
				keyCopy := k
				matchingKey = &keyCopy
				if k.LeaseDurationSeconds > 0 {
					leaseDuration = time.Duration(k.LeaseDurationSeconds) * time.Second
				}
				break
			}
		}

		if matchingKey == nil {
			h.rateLimiter.RecordKnockFailure(clientIP, now)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid access passphrase"})
			return
		}
	}

	// Validate Gamertag if mode is 'gamertag' or 'combined'
	if server.PortGateMode == models.PortGateModeGamertag || server.PortGateMode == models.PortGateModeCombined {
		if req.Gamertag == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Gamertag is required"})
			return
		}
	}

	// Success!
	h.rateLimiter.RecordKnockSuccess(clientIP)
	if matchingKey != nil {
		_ = h.db.IncrementKeyUsage(r.Context(), matchingKey.ID)
	}

	// Generate session token for mobile roaming handoff & heartbeat
	sessionToken, _ := auth.GenerateRandomToken(16)
	sessionTokenHash := auth.HashToken(sessionToken)

	var keyID *int64
	if matchingKey != nil {
		keyID = &matchingKey.ID
	}

	lease := &models.PortGateLease{
		ServerID:         server.ID,
		KeyID:            keyID,
		IPAddress:        clientIP,
		Gamertag:         req.Gamertag,
		KnockMethod:      server.PortGateMode,
		SessionTokenHash: sessionTokenHash,
		GrantedAt:        now,
		ExpiresAt:        now.Add(leaseDuration),
		Comment:          "Web knock authorization",
		Status:           "active",
	}

	if err := h.db.CreatePortGateLease(r.Context(), lease); err != nil {
		http.Error(w, `{"error": "Failed to record lease"}`, http.StatusInternalServerError)
		return
	}

	// Set signed session cookie for mobile roaming & heartbeat
	http.SetCookie(w, &http.Cookie{
		Name:     "bsm_knock_" + server.ID,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(leaseDuration.Seconds()),
	})

	// Generate direct Minecraft launcher URI: minecraft://?addExternalServer=<Name>|<Host>:<Port>
	hostHeader := r.Host
	directURL := fmt.Sprintf("minecraft://?addExternalServer=%s|%s:%d", server.Name, hostHeader, server.Port)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":            true,
		"ip_address":         clientIP,
		"expires_at":         database.FormatTime(lease.ExpiresAt),
		"expires_in_seconds": int(leaseDuration.Seconds()),
		"session_token":      sessionToken,
		"direct_launch_url":  directURL,
		"server_name":        server.Name,
		"server_port":        server.Port,
	})
}

// Heartbeat detects mobile IP roaming and auto-updates the active lease.
func (h *KnockHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	cookieName := "bsm_knock_" + serverID

	var sessionToken string
	if cookie, err := r.Cookie(cookieName); err == nil {
		sessionToken = cookie.Value
	}
	if sessionToken == "" {
		sessionToken = r.Header.Get("X-Knock-Token")
	}

	if sessionToken == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "No active knock session found"})
		return
	}

	tokenHash := auth.HashToken(sessionToken)
	lease, err := h.db.GetActiveLeaseBySessionToken(r.Context(), serverID, tokenHash)
	if err != nil || lease == nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Lease expired or not found"})
		return
	}

	currentIP := GetClientIP(r).String()
	ipUpdated := false

	if lease.IPAddress != currentIP {
		// Roaming detected! Hot-swap IP
		oldIP := lease.IPAddress
		if err := h.db.UpdateLeaseIP(r.Context(), lease.ID, currentIP); err == nil {
			ipUpdated = true
			_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
				ActorType: "player",
				ActorName: lease.Gamertag,
				Action:    "knock_roaming_handoff",
				Target:    serverID,
				Details:   fmt.Sprintf(`{"old_ip": "%s", "new_ip": "%s"}`, oldIP, currentIP),
				ClientIP:  currentIP,
			})
		}
	}

	remaining := int(time.Until(lease.ExpiresAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":             "active",
		"ip_address":         currentIP,
		"ip_updated":         ipUpdated,
		"expires_in_seconds": remaining,
	})
}

// Status returns active lease status for the client.
func (h *KnockHandler) Status(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	currentIP := GetClientIP(r).String()
	lease, err := h.db.GetActiveLeaseByIP(r.Context(), serverID, currentIP)
	if err != nil || lease == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"active":     false,
			"ip_address": currentIP,
		})
		return
	}

	remaining := int(time.Until(lease.ExpiresAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	directURL := fmt.Sprintf("minecraft://?addExternalServer=%s|%s:%d", server.Name, r.Host, server.Port)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"active":             true,
		"ip_address":         currentIP,
		"gamertag":           lease.Gamertag,
		"expires_in_seconds": remaining,
		"direct_launch_url":  directURL,
	})
}
