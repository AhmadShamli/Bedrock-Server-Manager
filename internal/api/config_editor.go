package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/clone"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/configfile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/preset"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/updater"
	"github.com/go-chi/chi/v5"
)

// GetProperties loads server.properties for a server.
func (h *ServerHandler) GetProperties(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	propPath, err := configfile.SafePath(h.dataDir, serverID, "server.properties")
	if err != nil {
		http.Error(w, `{"error": "Invalid path"}`, http.StatusBadRequest)
		return
	}

	props, keys, err := configfile.ReadProperties(propPath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to read properties: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"properties": props,
		"keys":       keys,
	})
}

// UpdateProperties saves changes to server.properties.
func (h *ServerHandler) UpdateProperties(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	propPath, err := configfile.SafePath(h.dataDir, serverID, "server.properties")
	if err != nil {
		http.Error(w, `{"error": "Invalid path"}`, http.StatusBadRequest)
		return
	}

	var payload struct {
		Properties map[string]string `json:"properties"`
		Keys       []string          `json:"keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, http.StatusBadRequest)
		return
	}

	if err := configfile.WriteProperties(propPath, payload.Properties, payload.Keys); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to write properties: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "update_properties",
		Target:    serverID,
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

// GetAllowlist loads allowlist.json.
func (h *ServerHandler) GetAllowlist(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	filePath, err := configfile.SafePath(h.dataDir, serverID, "allowlist.json")
	if err != nil {
		http.Error(w, `{"error": "Invalid path"}`, http.StatusBadRequest)
		return
	}

	list, err := configfile.ReadAllowlist(filePath)
	if err != nil {
		http.Error(w, `{"error": "Failed to read allowlist"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// UpdateAllowlist saves allowlist.json.
func (h *ServerHandler) UpdateAllowlist(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	filePath, err := configfile.SafePath(h.dataDir, serverID, "allowlist.json")
	if err != nil {
		http.Error(w, `{"error": "Invalid path"}`, http.StatusBadRequest)
		return
	}

	var list []configfile.AllowlistEntry
	if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, http.StatusBadRequest)
		return
	}

	if err := configfile.WriteAllowlist(filePath, list); err != nil {
		http.Error(w, `{"error": "Failed to save allowlist"}`, http.StatusInternalServerError)
		return
	}

	// Live reload if server is active
	if srv, err := h.db.GetServer(r.Context(), serverID); err == nil && srv.Status == models.ServerStatusRunning && h.engine != nil {
		_ = h.engine.SendConsoleCommand(r.Context(), srv, "allowlist reload")
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "update_allowlist",
		Target:    serverID,
		Details:   fmt.Sprintf(`{"count": %d}`, len(list)),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

// GetPermissions loads permissions.json.
func (h *ServerHandler) GetPermissions(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	filePath, err := configfile.SafePath(h.dataDir, serverID, "permissions.json")
	if err != nil {
		http.Error(w, `{"error": "Invalid path"}`, http.StatusBadRequest)
		return
	}

	list, err := configfile.ReadPermissions(filePath)
	if err != nil {
		http.Error(w, `{"error": "Failed to read permissions"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// UpdatePermissions saves permissions.json.
func (h *ServerHandler) UpdatePermissions(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	filePath, err := configfile.SafePath(h.dataDir, serverID, "permissions.json")
	if err != nil {
		http.Error(w, `{"error": "Invalid path"}`, http.StatusBadRequest)
		return
	}

	var list []configfile.PermissionEntry
	if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, http.StatusBadRequest)
		return
	}

	if err := configfile.WritePermissions(filePath, list); err != nil {
		http.Error(w, `{"error": "Failed to save permissions"}`, http.StatusInternalServerError)
		return
	}

	// Live reload if server is active
	if srv, err := h.db.GetServer(r.Context(), serverID); err == nil && srv.Status == models.ServerStatusRunning && h.engine != nil {
		_ = h.engine.SendConsoleCommand(r.Context(), srv, "permission reload")
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "update_permissions",
		Target:    serverID,
		Details:   fmt.Sprintf(`{"count": %d}`, len(list)),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

// Clone handles 1-click server cloning.
func (h *ServerHandler) Clone(w http.ResponseWriter, r *http.Request) {
	srcID := chi.URLParam(r, "id")
	var payload struct {
		NewID   string `json:"new_id"`
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.NewID == "" || payload.NewName == "" {
		http.Error(w, `{"error": "New ID and Name are required"}`, http.StatusBadRequest)
		return
	}

	cloned, err := clone.CloneServer(r.Context(), srcID, payload.NewID, payload.NewName, h.dataDir, h.db, h.allocator, h.engine)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "clone_server",
		Target:    cloned.ID,
		Details:   fmt.Sprintf(`{"source_id": "%s", "new_name": "%s"}`, srcID, payload.NewName),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(cloned)
}

// Export streams a zip bundle of the server files.
func (h *ServerHandler) Export(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-export.zip"`, serverID))

	if err := clone.ExportServer(serverID, h.dataDir, w); err != nil {
		http.Error(w, `{"error": "Export failed"}`, http.StatusInternalServerError)
		return
	}
}

// CopyConfigs selectively syncs allowlists, permissions, or properties to target servers.
func (h *ServerHandler) CopyConfigs(w http.ResponseWriter, r *http.Request) {
	srcID := chi.URLParam(r, "id")
	var opts clone.CopyConfigOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	result, err := clone.CopyConfigs(r.Context(), srcID, opts, h.dataDir, h.db, h.engine)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	targetsJSON, _ := json.Marshal(opts.TargetServerIDs)
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "copy_configs",
		Target:    srcID,
		Details:   fmt.Sprintf(`{"targets": %s, "allowlist": %t, "permissions": %t, "properties": %t, "mode": "%s"}`, string(targetsJSON), opts.CopyAllowlist, opts.CopyPermissions, opts.CopyProperties, opts.Mode),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// ListPresets returns server creation presets.
func ListPresets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(preset.GetPresets())
}

// CheckUpdates checks if newer BDS versions are available.
func CheckUpdates(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	if version == "" {
		version = "latest"
	}

	checker := updater.NewChecker()
	info, err := checker.CheckUpdate(r.Context(), version)
	if err != nil {
		http.Error(w, `{"error": "Version check failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}
