package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/bds"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/firewall"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ipresolver"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/turnstile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/version"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/webhook"
)

type SystemHandler struct {
	db          *database.ManagerDB
	dataDir     string
	bdsCache    *bds.CacheManager
	bdsResolver *bds.Resolver
	firewall    firewall.FirewallDriver
	ipResolver  *ipresolver.Resolver
	rateLimiter       *auth.RateLimiter
	turnstileVerifier *turnstile.Verifier
	startTime         time.Time
}

func (h *SystemHandler) SetTurnstileVerifier(tv *turnstile.Verifier) {
	h.turnstileVerifier = tv
}

func NewSystemHandler(db *database.ManagerDB, dataDir string, bdsCache *bds.CacheManager, bdsResolver *bds.Resolver, fw firewall.FirewallDriver, ipRes *ipresolver.Resolver, rl *auth.RateLimiter) *SystemHandler {
	return &SystemHandler{
		db:          db,
		dataDir:     dataDir,
		bdsCache:    bdsCache,
		bdsResolver: bdsResolver,
		firewall:    fw,
		ipResolver:  ipRes,
		rateLimiter: rl,
		startTime:   time.Now(),
	}
}

// Health checks system status.
func (h *SystemHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"version": version.Version,
		"uptime":  time.Since(h.startTime).String(),
	})
}

// Version returns application version and repository metadata.
func (h *SystemHandler) Version(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"version":        version.Version,
		"app_name":       version.AppName,
		"author":         version.Author,
		"repository_url": version.RepositoryURL,
	})
}

// SystemInfo returns server system and Go runtime stats.
func (h *SystemHandler) SystemInfo(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fwName := "unknown"
	if h.firewall != nil {
		fwName = h.firewall.Name()
	}
	proxyMode := "direct"
	if h.ipResolver != nil {
		proxyMode = h.ipResolver.ProxyMode()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"version":         version.Version,
		"app_name":        version.AppName,
		"go_version":      runtime.Version(),
		"goroutines":      runtime.NumGoroutine(),
		"os":              runtime.GOOS,
		"arch":            runtime.GOARCH,
		"uptime_sec":      int64(time.Since(h.startTime).Seconds()),
		"alloc_mb":        float64(m.Alloc) / (1024 * 1024),
		"sys_mb":          float64(m.Sys) / (1024 * 1024),
		"firewall_driver": fwName,
		"proxy_mode":      proxyMode,
	})
}

// ListAuditLogs returns paginated audit logs.
func (h *SystemHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 200 {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	logs, err := h.db.ListAuditLogs(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, `{"error": "Failed to query audit logs"}`, http.StatusInternalServerError)
		return
	}

	if logs == nil {
		logs = []models.AuditLog{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(logs)
}

// GetSettings returns current system settings.
func (h *SystemHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.db.GetAllSettings(r.Context())
	if err != nil {
		http.Error(w, `{"error": "Failed to load settings"}`, http.StatusInternalServerError)
		return
	}

	// Redact sensitive settings
	safeSettings := make(map[string]string)
	for k, v := range settings {
		if k == "jwt_secret" || k == "pepper" || k == "curseforge_api_key" || k == turnstile.SettingTurnstileSecretKey || k == "ddns_auth_token" || k == "ddns_password" {
			if v != "" {
				safeSettings[k] = "******"
			} else {
				safeSettings[k] = ""
			}
		} else {
			safeSettings[k] = v
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(safeSettings)
}

// UpdateSetting updates a specific system setting.
func (h *SystemHandler) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Key == "" {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	// Do not allow overwriting jwt_secret or pepper directly via this API
	if payload.Key == "jwt_secret" || payload.Key == "pepper" {
		http.Error(w, `{"error": "Cannot modify core secrets"}`, http.StatusForbidden)
		return
	}

	// Ignore placeholder masked values
	if (payload.Key == "curseforge_api_key" || payload.Key == turnstile.SettingTurnstileSecretKey || payload.Key == "ddns_auth_token" || payload.Key == "ddns_password") && payload.Value == "******" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ignored"})
		return
	}

	if err := h.db.SetSetting(r.Context(), payload.Key, payload.Value); err != nil {
		http.Error(w, `{"error": "Failed to update setting"}`, http.StatusInternalServerError)
		return
	}

	if h.rateLimiter != nil && (payload.Key == "rate_limit_max_attempts" || payload.Key == "circuit_breaker_threshold" || payload.Key == "circuit_breaker_lockout_minutes") {
		maxAtt, distThresh, distLock := h.rateLimiter.Config()
		if payload.Key == "rate_limit_max_attempts" {
			if v, err := strconv.Atoi(payload.Value); err == nil && v > 0 {
				maxAtt = v
			}
		} else if payload.Key == "circuit_breaker_threshold" {
			if v, err := strconv.Atoi(payload.Value); err == nil && v > 0 {
				distThresh = v
			}
		} else if payload.Key == "circuit_breaker_lockout_minutes" {
			if v, err := strconv.Atoi(payload.Value); err == nil && v > 0 {
				distLock = time.Duration(v) * time.Minute
			}
		}
		h.rateLimiter.UpdateConfig(maxAtt, distThresh, distLock)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// GetBDSStatus returns EULA status, VC++ runtime status, BDS cache stats, and platform capabilities.
func (h *SystemHandler) GetBDSStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eulaAccepted, _ := bds.IsEULAAccepted(ctx, h.db)
	eulaAcceptedAt, _ := h.db.GetSetting(ctx, bds.SettingEULAAcceptedAt)
	eulaAcceptedBy, _ := h.db.GetSetting(ctx, bds.SettingEULAAcceptedBy)

	var vcInstalled bool = true
	var vcError string
	if runtime.GOOS == "windows" {
		if err := engine.CheckVCRuntime(); err != nil {
			vcInstalled = false
			vcError = err.Error()
		}
	}

	var cacheSize int64
	var versionCount int
	var cachedVersions []string
	if h.bdsCache != nil {
		cacheSize, versionCount, _ = h.bdsCache.GetCacheSize()
		cachedVersions, _ = h.bdsCache.GetCachedVersions()
	}

	res := map[string]interface{}{
		"is_windows":       runtime.GOOS == "windows",
		"eula_accepted":    eulaAccepted,
		"eula_accepted_at": eulaAcceptedAt,
		"eula_accepted_by": eulaAcceptedBy,
		"eula_url":         bds.MinecraftEULAURL,
		"privacy_url":      bds.MicrosoftPrivacyURL,
		"vc_installed":     vcInstalled,
		"vc_error":         vcError,
		"cache_bytes":      cacheSize,
		"cache_count":      versionCount,
		"cached_versions":  cachedVersions,
		"data_dir":         h.dataDir,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// AcceptBDSEULA records user acceptance of the Minecraft EULA and Privacy Policy.
func (h *SystemHandler) AcceptBDSEULA(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	username := "admin"
	if claims != nil {
		username = claims.Username
	}

	if err := bds.AcceptEULA(r.Context(), h.db, username); err != nil {
		http.Error(w, `{"error": "Failed to accept EULA"}`, http.StatusInternalServerError)
		return
	}

	var userID *int64
	if claims != nil {
		userID = &claims.UserID
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    userID,
		ActorType: "user",
		ActorName: username,
		Action:    "bds_eula_accept",
		Target:    "system",
		Details:   `{"eula": "accepted"}`,
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// PurgeBDSCache purges unused cached BDS versions, protecting versions currently in use or designated for rollback.
func (h *SystemHandler) PurgeBDSCache(w http.ResponseWriter, r *http.Request) {
	if h.bdsCache == nil {
		http.Error(w, `{"error": "BDS cache manager not initialized"}`, http.StatusBadRequest)
		return
	}

	servers, err := h.db.ListServers(r.Context())
	if err != nil {
		http.Error(w, `{"error": "Failed to inspect servers"}`, http.StatusInternalServerError)
		return
	}

	var protected []string
	for _, s := range servers {
		if s.InstalledBDSVersion != "" {
			protected = append(protected, s.InstalledBDSVersion)
		}
		if s.PreviousBDSVersion != "" {
			protected = append(protected, s.PreviousBDSVersion)
		}
	}

	if err := h.bdsCache.PurgeUnused(protected); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to purge cache: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "purged"})
}

// CheckBDSVersions resolves the latest and preview official Windows BDS versions.
func (h *SystemHandler) CheckBDSVersions(w http.ResponseWriter, r *http.Request) {
	if h.bdsResolver == nil {
		http.Error(w, `{"error": "BDS resolver not initialized"}`, http.StatusBadRequest)
		return
	}

	force := r.URL.Query().Get("force") == "true" || r.URL.Query().Get("force") == "1"
	rel, errRel := h.bdsResolver.ResolveVersion(r.Context(), "latest", force)
	prev, _ := h.bdsResolver.ResolveVersion(r.Context(), "preview", force)

	if errRel != nil && rel == nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to resolve BDS version: %s"}`, errRel.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"latest":  rel,
		"preview": prev,
	})
}

// SetLoopbackExemption applies UWP loopback exemption so local Minecraft client can connect on Windows (Decision 25).
func (h *SystemHandler) SetLoopbackExemption(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "windows" {
		http.Error(w, `{"error": "Loopback exemption is only supported on Windows hosts"}`, http.StatusBadRequest)
		return
	}

	cmd := exec.CommandContext(r.Context(), "CheckNetIsolation.exe", "LoopbackExempt", "-a", "-n=Microsoft.MinecraftUWP_8wekyb3d8bbwe")
	out, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to set loopback exemption: %s (output: %s)"}`, err.Error(), string(out)), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "output": strings.TrimSpace(string(out))})
}

// AddDefenderExclusion adds a Windows Defender folder exclusion for the active data directory (Decision 28).
func (h *SystemHandler) AddDefenderExclusion(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "windows" {
		http.Error(w, `{"error": "Windows Defender exclusion is only supported on Windows hosts"}`, http.StatusBadRequest)
		return
	}

	absDataDir, err := filepath.Abs(h.dataDir)
	if err != nil {
		absDataDir = h.dataDir
	}

	psScript := fmt.Sprintf("Add-MpPreference -ExclusionPath '%s'", absDataDir)
	cmd := exec.CommandContext(r.Context(), "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to add Defender exclusion (requires Administrator): %s (output: %s)"}`, err.Error(), string(out)), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "exclusion_path": absDataDir})
}

// TestDiscordWebhook dispatches a sample test notification to a Discord webhook URL.
func (h *SystemHandler) TestDiscordWebhook(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		WebhookURL string `json:"webhook_url"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	webhookURL := strings.TrimSpace(payload.WebhookURL)
	if webhookURL == "" {
		stored, err := h.db.GetSetting(r.Context(), "discord_webhook_url")
		if err == nil && stored != "" {
			webhookURL = stored
		}
	}

	if webhookURL == "" {
		http.Error(w, `{"error": "No Discord Webhook URL provided or configured in settings"}`, http.StatusBadRequest)
		return
	}

	dispatcher := webhook.NewDispatcher()
	embed := webhook.Embed{
		Title:       "🔔 BSM Outbound Webhook Test",
		Description: "Your Discord webhook integration with Bedrock Server Manager is working successfully!",
		Color:       webhook.ColorEmerald,
		Fields: []webhook.EmbedField{
			{Name: "Status", Value: "Connected & Verified", Inline: true},
			{Name: "Server Time", Value: time.Now().UTC().Format(time.RFC3339), Inline: true},
		},
	}

	if err := dispatcher.Send(r.Context(), webhookURL, embed); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to send Discord webhook: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Test notification sent successfully!"})
}

// GetTurnstilePublicConfig returns public Cloudflare Turnstile configuration (enabled state, site key, and per-flow flags).
func (h *SystemHandler) GetTurnstilePublicConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := turnstile.LoadConfig(r.Context(), h.db)
	if err != nil {
		http.Error(w, `{"error": "Failed to load turnstile configuration"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

// TestTurnstile allows administrators to verify a Turnstile Site Key, Secret Key, and client response token.
func (h *SystemHandler) TestTurnstile(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		SiteKey   string `json:"site_key"`
		SecretKey string `json:"secret_key"`
		Token     string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	secretKey := strings.TrimSpace(payload.SecretKey)
	if secretKey == "" || secretKey == "******" {
		stored, _ := h.db.GetSetting(r.Context(), turnstile.SettingTurnstileSecretKey)
		secretKey = strings.TrimSpace(stored)
	}

	if secretKey == "" {
		http.Error(w, `{"error": "Turnstile Secret Key is required or must be configured"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(payload.Token) == "" {
		http.Error(w, `{"error": "Test response token is required. Solve the challenge first."}`, http.StatusBadRequest)
		return
	}

	verifier := h.turnstileVerifier
	if verifier == nil {
		verifier = turnstile.NewVerifier()
	}

	clientIP := GetClientIP(r).String()
	res, err := verifier.Verify(r.Context(), secretKey, payload.Token, clientIP, "")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Cloudflare Turnstile API error: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

