package api

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/version"
)

type SystemHandler struct {
	db        *database.ManagerDB
	startTime time.Time
}

func NewSystemHandler(db *database.ManagerDB) *SystemHandler {
	return &SystemHandler{
		db:        db,
		startTime: time.Now(),
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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"version":    version.Version,
		"app_name":   version.AppName,
		"go_version": runtime.Version(),
		"goroutines": runtime.NumGoroutine(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"uptime_sec": int64(time.Since(h.startTime).Seconds()),
		"alloc_mb":   float64(m.Alloc) / (1024 * 1024),
		"sys_mb":     float64(m.Sys) / (1024 * 1024),
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
		if k == "jwt_secret" || k == "pepper" {
			safeSettings[k] = "******"
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

	if err := h.db.SetSetting(r.Context(), payload.Key, payload.Value); err != nil {
		http.Error(w, `{"error": "Failed to update setting"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}
