package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ddns"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// DDNSHandler exposes HTTP endpoints for managing Dynamic DNS.
type DDNSHandler struct {
	db      *database.ManagerDB
	manager *ddns.Manager
}

// NewDDNSHandler initializes the API handler.
func NewDDNSHandler(db *database.ManagerDB, manager *ddns.Manager) *DDNSHandler {
	return &DDNSHandler{
		db:      db,
		manager: manager,
	}
}

// LoadDDNSConfig builds a ddns.Config from database system_settings.
func LoadDDNSConfig(ctx context.Context, db *database.ManagerDB) ddns.Config {
	cfg := ddns.Config{
		Enabled:             false,
		Provider:            ddns.ProviderCloudflare,
		EnableIPv4:          true,
		EnableIPv6:          false,
		IntervalMinutes:     5,
		AutoSyncGameAddress: true,
		WebhookMethod:       "GET",
	}

	if db == nil {
		return cfg
	}

	settings, err := db.GetAllSettings(ctx)
	if err != nil {
		return cfg
	}

	if val, ok := settings["ddns_enabled"]; ok {
		cfg.Enabled = val == "true" || val == "1"
	}
	if val, ok := settings["ddns_provider"]; ok && val != "" {
		cfg.Provider = val
	}
	if val, ok := settings["ddns_domain"]; ok {
		cfg.Domain = val
	}
	if val, ok := settings["ddns_enable_ipv4"]; ok {
		cfg.EnableIPv4 = val == "true" || val == "1"
	}
	if val, ok := settings["ddns_enable_ipv6"]; ok {
		cfg.EnableIPv6 = val == "true" || val == "1"
	}
	if val, ok := settings["ddns_interval_minutes"]; ok {
		if v, err := strconv.Atoi(val); err == nil && v > 0 {
			cfg.IntervalMinutes = v
		}
	}
	if val, ok := settings["ddns_auto_sync_game_address"]; ok {
		cfg.AutoSyncGameAddress = val == "true" || val == "1"
	}
	if val, ok := settings["ddns_auth_token"]; ok {
		cfg.AuthToken = val
	}
	if val, ok := settings["ddns_zone_id"]; ok {
		cfg.ZoneID = val
	}
	if val, ok := settings["ddns_username"]; ok {
		cfg.Username = val
	}
	if val, ok := settings["ddns_password"]; ok {
		cfg.Password = val
	}
	if val, ok := settings["ddns_webhook_url"]; ok {
		cfg.WebhookURL = val
	}
	if val, ok := settings["ddns_webhook_method"]; ok && val != "" {
		cfg.WebhookMethod = val
	}

	return cfg
}

// GetStatus returns the current runtime DDNS status and detected IPs.
func (h *DDNSHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		http.Error(w, `{"error": "DDNS manager is not initialized"}`, http.StatusInternalServerError)
		return
	}

	status := h.manager.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

// GetConfig returns the persisted DDNS configuration with secrets redacted.
func (h *DDNSHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		http.Error(w, `{"error": "DDNS manager is not initialized"}`, http.StatusInternalServerError)
		return
	}

	cfg := h.manager.GetConfig()
	safeCfg := cfg
	if safeCfg.AuthToken != "" {
		safeCfg.AuthToken = "******"
	}
	if safeCfg.Password != "" {
		safeCfg.Password = "******"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(safeCfg)
}

// UpdateConfig updates DDNS settings in DB and applies them to the runtime manager.
func (h *DDNSHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req ddns.Config
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	currentCfg := h.manager.GetConfig()

	// Handle masked secrets
	if req.AuthToken == "******" || req.AuthToken == "" {
		req.AuthToken = currentCfg.AuthToken
	}
	if req.Password == "******" || req.Password == "" {
		req.Password = currentCfg.Password
	}

	if req.IntervalMinutes <= 0 {
		req.IntervalMinutes = 5
	}
	if req.Provider == "" {
		req.Provider = ddns.ProviderCloudflare
	}

	// Persist to system_settings
	_ = h.db.SetSetting(ctx, "ddns_enabled", strconv.FormatBool(req.Enabled))
	_ = h.db.SetSetting(ctx, "ddns_provider", req.Provider)
	_ = h.db.SetSetting(ctx, "ddns_domain", strings.TrimSpace(req.Domain))
	_ = h.db.SetSetting(ctx, "ddns_enable_ipv4", strconv.FormatBool(req.EnableIPv4))
	_ = h.db.SetSetting(ctx, "ddns_enable_ipv6", strconv.FormatBool(req.EnableIPv6))
	_ = h.db.SetSetting(ctx, "ddns_interval_minutes", strconv.Itoa(req.IntervalMinutes))
	_ = h.db.SetSetting(ctx, "ddns_auto_sync_game_address", strconv.FormatBool(req.AutoSyncGameAddress))
	_ = h.db.SetSetting(ctx, "ddns_auth_token", req.AuthToken)
	_ = h.db.SetSetting(ctx, "ddns_zone_id", strings.TrimSpace(req.ZoneID))
	_ = h.db.SetSetting(ctx, "ddns_username", strings.TrimSpace(req.Username))
	_ = h.db.SetSetting(ctx, "ddns_password", req.Password)
	_ = h.db.SetSetting(ctx, "ddns_webhook_url", strings.TrimSpace(req.WebhookURL))
	_ = h.db.SetSetting(ctx, "ddns_webhook_method", req.WebhookMethod)

	// Apply to live manager
	h.manager.UpdateConfig(req)

	claims := GetUserClaims(r)
	actorName := "admin"
	if claims != nil {
		actorName = claims.Username
	}
	_ = h.db.CreateAuditLog(ctx, &models.AuditLog{
		ActorType: "user",
		ActorName: actorName,
		Action:    "ddns_config_updated",
		Target:    "system",
		Details:   fmt.Sprintf(`{"enabled": %t, "provider": "%s", "domain": "%s", "interval_minutes": %d}`, req.Enabled, req.Provider, req.Domain, req.IntervalMinutes),
		ClientIP:  GetClientIP(r).String(),
	})

	safeCfg := req
	if safeCfg.AuthToken != "" {
		safeCfg.AuthToken = "******"
	}
	if safeCfg.Password != "" {
		safeCfg.Password = "******"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(safeCfg)
}

// Sync triggers an immediate on-demand synchronization pass.
func (h *DDNSHandler) Sync(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		http.Error(w, `{"error": "DDNS manager is not initialized"}`, http.StatusInternalServerError)
		return
	}

	result, err := h.manager.Sync(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
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
		Action:    "ddns_sync_triggered",
		Target:    "system",
		Details:   fmt.Sprintf(`{"provider": "%s", "domain": "%s"}`, h.manager.GetConfig().Provider, h.manager.GetConfig().Domain),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"result":  result,
		"status":  h.manager.GetStatus(),
	})
}

// Test verifies provider credentials without altering the persistent manager state.
func (h *DDNSHandler) Test(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		http.Error(w, `{"error": "DDNS manager is not initialized"}`, http.StatusInternalServerError)
		return
	}

	var req ddns.Config
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	currentCfg := h.manager.GetConfig()
	if req.AuthToken == "******" || req.AuthToken == "" {
		req.AuthToken = currentCfg.AuthToken
	}
	if req.Password == "******" || req.Password == "" {
		req.Password = currentCfg.Password
	}

	res, err := h.manager.Test(r.Context(), &req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"result":  res,
	})
}
