package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/firewall"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/go-chi/chi/v5"
)

type KnockHandler struct {
	db          *database.ManagerDB
	rateLimiter *auth.RateLimiter
	pepper      string
	firewall    firewall.FirewallDriver
}

func NewKnockHandler(db *database.ManagerDB, rateLimiter *auth.RateLimiter, pepper string, fw firewall.FirewallDriver) *KnockHandler {
	return &KnockHandler{
		db:          db,
		rateLimiter: rateLimiter,
		pepper:      pepper,
		firewall:    fw,
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
	ClientIP                 string `json:"client_ip"`
	AlwaysAllowed            bool   `json:"always_allowed"`
	RuleComment              string `json:"rule_comment,omitempty"`
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

	clientIP := GetClientIP(r).String()
	matchRule, alwaysAllowed, _ := h.db.IsIPAllowed(r.Context(), serverID, clientIP)
	var ruleComment string
	if matchRule != nil {
		ruleComment = matchRule.Comment
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(KnockConfigResponse{
		ServerID:                 server.ID,
		ServerName:               server.Name,
		Port:                     server.Port,
		PortGateEnabled:          server.PortGateEnabled,
		PortGateMode:             server.PortGateMode,
		HeartbeatIntervalSeconds: hbSec,
		ClientIP:                 clientIP,
		AlwaysAllowed:            alwaysAllowed,
		RuleComment:              ruleComment,
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

	// Check if caller's IP is already permanently allowed
	if matchRule, allowed, _ := h.db.IsIPAllowed(r.Context(), serverID, clientIP); allowed && matchRule != nil {
		if h.firewall != nil {
			comment := fmt.Sprintf("bsm_perm_%d", matchRule.ID)
			_ = h.firewall.AllowPort(r.Context(), matchRule.IPOrSubnet, server.Port, comment)
			if server.PortV6 > 0 {
				_ = h.firewall.AllowPort(r.Context(), matchRule.IPOrSubnet, server.PortV6, comment)
			}
		}
		directURL := fmt.Sprintf("minecraft://?addExternalServer=%s|%s:%d", server.Name, r.Host, server.Port)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":            true,
			"always_allowed":     true,
			"message":            "Your IP address is permanently authorized. Port is open without knocking.",
			"ip_address":         clientIP,
			"direct_launch_url":  directURL,
			"server_name":        server.Name,
			"server_port":        server.Port,
			"expires_in_seconds": 86400 * 365,
		})
		return
	}

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

	// Apply dynamic firewall allow rule
	if h.firewall != nil {
		comment := fmt.Sprintf("bsm_%s_%d", server.ID, lease.ID)
		_ = h.firewall.AllowPort(r.Context(), clientIP, server.Port, comment)
		if server.PortV6 > 0 {
			_ = h.firewall.AllowPort(r.Context(), clientIP, server.PortV6, comment)
		}
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

	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil || server == nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	currentIP := GetClientIP(r).String()
	ipUpdated := false

	if lease.IPAddress != currentIP {
		// Roaming detected! Hot-swap IP in firewall and database
		oldIP := lease.IPAddress
		if err := h.db.UpdateLeaseIP(r.Context(), lease.ID, currentIP); err == nil {
			ipUpdated = true
			if h.firewall != nil {
				comment := fmt.Sprintf("bsm_%s_%d", server.ID, lease.ID)
				_ = h.firewall.RevokePort(r.Context(), oldIP, server.Port, comment)
				_ = h.firewall.AllowPort(r.Context(), currentIP, server.Port, comment)
				if server.PortV6 > 0 {
					_ = h.firewall.RevokePort(r.Context(), oldIP, server.PortV6, comment)
					_ = h.firewall.AllowPort(r.Context(), currentIP, server.PortV6, comment)
				}
			}
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

	// 1. Check if caller IP is in permanent allowlist
	if matchRule, allowed, err := h.db.IsIPAllowed(r.Context(), serverID, currentIP); err == nil && allowed && matchRule != nil {
		directURL := fmt.Sprintf("minecraft://?addExternalServer=%s|%s:%d", server.Name, r.Host, server.Port)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"active":             true,
			"always_allowed":     true,
			"ip_address":         currentIP,
			"rule_comment":       matchRule.Comment,
			"is_global":          matchRule.ServerID == nil || *matchRule.ServerID == "",
			"direct_launch_url":  directURL,
			"server_name":        server.Name,
			"server_port":        server.Port,
			"expires_in_seconds": 86400 * 365,
		})
		return
	}

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

// ListLeases returns active leases for the given server.
func (h *KnockHandler) ListLeases(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	leases, err := h.db.ListActiveLeases(r.Context(), serverID)
	if err != nil {
		http.Error(w, `{"error": "Failed to list leases"}`, http.StatusInternalServerError)
		return
	}
	if leases == nil {
		leases = []models.PortGateLease{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(leases)
}

// RevokeLease cancels an active lease immediately and removes firewall rules.
func (h *KnockHandler) RevokeLease(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	leaseIDStr := chi.URLParam(r, "leaseId")
	leaseID, err := strconv.ParseInt(leaseIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid lease ID"}`, http.StatusBadRequest)
		return
	}

	lease, err := h.db.GetLease(r.Context(), leaseID)
	if err != nil || lease == nil || lease.ServerID != serverID {
		http.Error(w, `{"error": "Lease not found"}`, http.StatusNotFound)
		return
	}

	server, err := h.db.GetServer(r.Context(), serverID)
	if err == nil && server != nil && h.firewall != nil {
		comment := fmt.Sprintf("bsm_%s_%d", server.ID, lease.ID)
		_ = h.firewall.RevokePort(r.Context(), lease.IPAddress, server.Port, comment)
		if server.PortV6 > 0 {
			_ = h.firewall.RevokePort(r.Context(), lease.IPAddress, server.PortV6, comment)
		}
	}

	_ = h.db.SetLeaseStatus(r.Context(), leaseID, "revoked")

	claims := GetUserClaims(r)
	actorName := "admin"
	if claims != nil {
		actorName = claims.Username
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: actorName,
		Action:    "knock_lease_revoked",
		Target:    serverID,
		Details:   fmt.Sprintf(`{"lease_id": %d, "ip": "%s"}`, leaseID, lease.IPAddress),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// CreateManualLease allows administrators to manually grant an IP bypass.
type ManualLeaseRequest struct {
	IPAddress       string `json:"ip_address"`
	Gamertag        string `json:"gamertag"`
	DurationMinutes int    `json:"duration_minutes"`
	Comment         string `json:"comment"`
}

func (h *KnockHandler) CreateManualLease(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	var req ManualLeaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	cleanIP := strings.TrimSpace(req.IPAddress)
	parsedIP := net.ParseIP(cleanIP)
	if parsedIP == nil {
		http.Error(w, `{"error": "Valid ip_address is required"}`, http.StatusBadRequest)
		return
	}
	req.IPAddress = parsedIP.String()

	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 60 // 1 hour default
	}

	now := time.Now().UTC()
	leaseDuration := time.Duration(req.DurationMinutes) * time.Minute
	sessionToken, _ := auth.GenerateRandomToken(16)
	sessionTokenHash := auth.HashToken(sessionToken)

	lease := &models.PortGateLease{
		ServerID:         server.ID,
		IPAddress:        req.IPAddress,
		Gamertag:         req.Gamertag,
		KnockMethod:      "manual_admin",
		SessionTokenHash: sessionTokenHash,
		GrantedAt:        now,
		ExpiresAt:        now.Add(leaseDuration),
		Comment:          req.Comment,
		Status:           "active",
	}

	if err := h.db.CreatePortGateLease(r.Context(), lease); err != nil {
		http.Error(w, `{"error": "Failed to create lease"}`, http.StatusInternalServerError)
		return
	}

	if h.firewall != nil {
		comment := fmt.Sprintf("bsm_%s_%d", server.ID, lease.ID)
		_ = h.firewall.AllowPort(r.Context(), req.IPAddress, server.Port, comment)
		if server.PortV6 > 0 {
			_ = h.firewall.AllowPort(r.Context(), req.IPAddress, server.PortV6, comment)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(lease)
}

// ListAccessKeys returns all configured access keys for a server.
func (h *KnockHandler) ListAccessKeys(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	keys, err := h.db.ListPortGateKeys(r.Context(), &serverID)
	if err != nil {
		http.Error(w, `{"error": "Failed to list access keys"}`, http.StatusInternalServerError)
		return
	}
	if keys == nil {
		keys = []models.PortGateKey{}
	}
	for i := range keys {
		keys[i].KeyHash = ""
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(keys)
}

// CreateAccessKeyRequest payload for generating or saving an access key.
type CreateAccessKeyRequest struct {
	Label                string     `json:"label"`
	Passphrase           string     `json:"passphrase"`
	MaxUses              int        `json:"max_uses"`
	LeaseDurationSeconds int        `json:"lease_duration_seconds"`
	ExpiresAt            *time.Time `json:"expires_at"`
}

// CreateAccessKeyResponse returns the saved key along with the one-time plaintext passphrase.
type CreateAccessKeyResponse struct {
	models.PortGateKey
	PlaintextPassphrase string `json:"plaintext_passphrase,omitempty"`
}

// CreateAccessKey provisions a new Port Gate access key / passphrase.
func (h *KnockHandler) CreateAccessKey(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil || server == nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	var req CreateAccessKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	passphrase := req.Passphrase
	if passphrase == "" {
		passphrase, _ = auth.GenerateRandomToken(12)
	}

	keyHash := auth.HashAccessKey(h.pepper, passphrase)
	prefix := passphrase
	if len(prefix) > 6 {
		prefix = prefix[:6]
	}

	leaseSec := req.LeaseDurationSeconds
	if leaseSec <= 0 {
		leaseSec = server.PortGateTimeout
	}
	if leaseSec <= 0 {
		leaseSec = 7200
	}

	key := &models.PortGateKey{
		ServerID:             &server.ID,
		Label:                req.Label,
		KeyHash:              keyHash,
		KeyPrefix:            prefix,
		MaxUses:              req.MaxUses,
		UsedCount:            0,
		LeaseDurationSeconds: leaseSec,
		ExpiresAt:            req.ExpiresAt,
		IsActive:             true,
	}

	if err := h.db.CreatePortGateKey(r.Context(), key); err != nil {
		http.Error(w, `{"error": "Failed to create access key"}`, http.StatusInternalServerError)
		return
	}

	claims := GetUserClaims(r)
	actorName := "admin"
	var userID *int64
	if claims != nil {
		actorName = claims.Username
		userID = &claims.UserID
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    userID,
		ActorType: "user",
		ActorName: actorName,
		Action:    "port_gate_key_created",
		Target:    server.ID,
		Details:   fmt.Sprintf(`{"key_id": %d, "label": "%s"}`, key.ID, key.Label),
		ClientIP:  GetClientIP(r).String(),
	})

	resp := CreateAccessKeyResponse{
		PortGateKey:         *key,
		PlaintextPassphrase: passphrase,
	}
	resp.KeyHash = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// DeleteAccessKey revokes and removes a Port Gate access key.
func (h *KnockHandler) DeleteAccessKey(w http.ResponseWriter, r *http.Request) {
	keyIDStr := chi.URLParam(r, "keyId")
	keyID, err := strconv.ParseInt(keyIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid key ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.DeletePortGateKey(r.Context(), keyID); err != nil {
		http.Error(w, `{"error": "Failed to delete access key"}`, http.StatusInternalServerError)
		return
	}

	claims := GetUserClaims(r)
	actorName := "admin"
	var userID *int64
	if claims != nil {
		actorName = claims.Username
		userID = &claims.UserID
	}
	serverID := chi.URLParam(r, "id")
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    userID,
		ActorType: "user",
		ActorName: actorName,
		Action:    "port_gate_key_deleted",
		Target:    serverID,
		Details:   fmt.Sprintf(`{"key_id": %d}`, keyID),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ListAllowRules returns permanent allowed IP/subnet rules.
func (h *KnockHandler) ListAllowRules(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	var sIDPtr *string
	if serverID != "" {
		sIDPtr = &serverID
	}

	rules, err := h.db.ListPortGateAllowRules(r.Context(), sIDPtr)
	if err != nil {
		http.Error(w, `{"error": "Failed to list allow rules"}`, http.StatusInternalServerError)
		return
	}
	if rules == nil {
		rules = []models.PortGateAllowRule{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rules)
}

// CreateAllowRule adds a new permanent allowed IP or subnet rule.
func (h *KnockHandler) CreateAllowRule(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	var req struct {
		IPOrSubnet string  `json:"ip_or_subnet"`
		IsGlobal   bool    `json:"is_global"`
		ServerID   *string `json:"server_id,omitempty"`
		Comment    string  `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.IPOrSubnet) == "" {
		http.Error(w, `{"error": "ip_or_subnet is required"}`, http.StatusBadRequest)
		return
	}

	target := strings.TrimSpace(req.IPOrSubnet)
	// Validate IP or CIDR
	if strings.Contains(target, "/") {
		_, _, err := net.ParseCIDR(target)
		if err != nil {
			http.Error(w, `{"error": "Invalid CIDR subnet format (e.g. 192.168.1.0/24 or 10.0.0.0/16)"}`, http.StatusBadRequest)
			return
		}
	} else {
		ip := net.ParseIP(target)
		if ip == nil {
			http.Error(w, `{"error": "Invalid IP address format (e.g. 192.168.1.100 or 2001:db8::1)"}`, http.StatusBadRequest)
			return
		}
	}

	rule := models.PortGateAllowRule{
		IPOrSubnet: target,
		Comment:    strings.TrimSpace(req.Comment),
	}

	if !req.IsGlobal && serverID != "" {
		rule.ServerID = &serverID
	} else if !req.IsGlobal && req.ServerID != nil && *req.ServerID != "" {
		rule.ServerID = req.ServerID
	}

	if err := h.db.CreatePortGateAllowRule(r.Context(), &rule); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to create rule: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Apply firewall rules immediately for matching server(s)
	if h.firewall != nil {
		comment := fmt.Sprintf("bsm_perm_%d", rule.ID)
		if rule.ServerID != nil && *rule.ServerID != "" {
			if s, err := h.db.GetServer(r.Context(), *rule.ServerID); err == nil && s != nil {
				_ = h.firewall.AllowPort(r.Context(), rule.IPOrSubnet, s.Port, comment)
				if s.PortV6 > 0 {
					_ = h.firewall.AllowPort(r.Context(), rule.IPOrSubnet, s.PortV6, comment)
				}
			}
		} else {
			// Global rule: apply to all active servers
			if servers, err := h.db.ListServers(r.Context()); err == nil {
				for _, s := range servers {
					_ = h.firewall.AllowPort(r.Context(), rule.IPOrSubnet, s.Port, comment)
					if s.PortV6 > 0 {
						_ = h.firewall.AllowPort(r.Context(), rule.IPOrSubnet, s.PortV6, comment)
					}
				}
			}
		}
	}

	claims := GetUserClaims(r)
	actorName := "admin"
	if claims != nil {
		actorName = claims.Username
	}
	targetID := "global"
	if rule.ServerID != nil {
		targetID = *rule.ServerID
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: actorName,
		Action:    "portgate_allow_rule_created",
		Target:    targetID,
		Details:   fmt.Sprintf(`{"rule_id": %d, "target": "%s", "comment": "%s"}`, rule.ID, target, rule.Comment),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(rule)
}

// DeleteAllowRule revokes and deletes a permanent allow rule.
func (h *KnockHandler) DeleteAllowRule(w http.ResponseWriter, r *http.Request) {
	ruleIDStr := chi.URLParam(r, "ruleId")
	ruleID, err := strconv.ParseInt(ruleIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid rule ID"}`, http.StatusBadRequest)
		return
	}

	rule, err := h.db.GetPortGateAllowRule(r.Context(), ruleID)
	if err != nil || rule == nil {
		http.Error(w, `{"error": "Rule not found"}`, http.StatusNotFound)
		return
	}

	// Revoke firewall rules
	if h.firewall != nil {
		comment := fmt.Sprintf("bsm_perm_%d", rule.ID)
		if rule.ServerID != nil && *rule.ServerID != "" {
			if s, err := h.db.GetServer(r.Context(), *rule.ServerID); err == nil && s != nil {
				_ = h.firewall.RevokePort(r.Context(), rule.IPOrSubnet, s.Port, comment)
				if s.PortV6 > 0 {
					_ = h.firewall.RevokePort(r.Context(), rule.IPOrSubnet, s.PortV6, comment)
				}
			}
		} else {
			if servers, err := h.db.ListServers(r.Context()); err == nil {
				for _, s := range servers {
					_ = h.firewall.RevokePort(r.Context(), rule.IPOrSubnet, s.Port, comment)
					if s.PortV6 > 0 {
						_ = h.firewall.RevokePort(r.Context(), rule.IPOrSubnet, s.PortV6, comment)
					}
				}
			}
		}
	}

	if err := h.db.DeletePortGateAllowRule(r.Context(), ruleID); err != nil {
		http.Error(w, `{"error": "Failed to delete rule"}`, http.StatusInternalServerError)
		return
	}

	claims := GetUserClaims(r)
	actorName := "admin"
	if claims != nil {
		actorName = claims.Username
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: actorName,
		Action:    "portgate_allow_rule_deleted",
		Target:    fmt.Sprintf("%d", ruleID),
		Details:   fmt.Sprintf(`{"ip_or_subnet": "%s"}`, rule.IPOrSubnet),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

