package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/configfile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/player"
	"github.com/go-chi/chi/v5"
)

// GlobalPlayerHandler manages global access control lists and multi-level player sync.
type GlobalPlayerHandler struct {
	db      *database.ManagerDB
	engine  engine.ServerEngine
	dataDir string
}

// NewGlobalPlayerHandler creates a GlobalPlayerHandler.
func NewGlobalPlayerHandler(db *database.ManagerDB, eng engine.ServerEngine, dataDir string) *GlobalPlayerHandler {
	return &GlobalPlayerHandler{
		db:      db,
		engine:  eng,
		dataDir: dataDir,
	}
}

// List returns all globally registered players.
func (h *GlobalPlayerHandler) List(w http.ResponseWriter, r *http.Request) {
	players, err := h.db.ListGlobalPlayers(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to list global players: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	if players == nil {
		players = []*models.GlobalPlayer{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(players)
}

// Get retrieves a single global player by ID.
func (h *GlobalPlayerHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid player ID"}`, http.StatusBadRequest)
		return
	}

	p, err := h.db.GetGlobalPlayer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Global player not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

// Create registers or updates a global player.
func (h *GlobalPlayerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p models.GlobalPlayer
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil || strings.TrimSpace(p.Name) == "" {
		http.Error(w, `{"error": "Gamertag name is required"}`, http.StatusBadRequest)
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	p.XUID = strings.TrimSpace(p.XUID)
	if p.Permission == "" {
		p.Permission = "member"
	}

	saved, err := h.db.UpsertGlobalPlayer(r.Context(), &p)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to save global player: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "upsert_global_player",
		Target:    saved.Name,
		Details:   fmt.Sprintf(`{"xuid": "%s", "role": "%s", "allowlist": %t}`, saved.XUID, saved.Permission, saved.IsAllowlisted),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(saved)
}

// Update modifies an existing global player.
func (h *GlobalPlayerHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid player ID"}`, http.StatusBadRequest)
		return
	}

	existing, err := h.db.GetGlobalPlayer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Global player not found"}`, http.StatusNotFound)
		return
	}

	var p models.GlobalPlayer
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(p.Name) != "" {
		existing.Name = strings.TrimSpace(p.Name)
	}
	if p.XUID != "" {
		existing.XUID = strings.TrimSpace(p.XUID)
	}
	existing.IsAllowlisted = p.IsAllowlisted
	if p.Permission != "" {
		existing.Permission = p.Permission
	}
	existing.IgnoresPlayerLimit = p.IgnoresPlayerLimit

	saved, err := h.db.UpdateGlobalPlayer(r.Context(), existing)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to update global player: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "update_global_player",
		Target:    saved.Name,
		Details:   fmt.Sprintf(`{"xuid": "%s", "role": "%s", "allowlist": %t}`, saved.XUID, saved.Permission, saved.IsAllowlisted),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(saved)
}

// Delete removes a player from the global access list.
func (h *GlobalPlayerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid player ID"}`, http.StatusBadRequest)
		return
	}

	existing, _ := h.db.GetGlobalPlayer(r.Context(), id)
	targetName := fmt.Sprint(id)
	if existing != nil {
		targetName = existing.Name
	}

	if err := h.db.DeleteGlobalPlayer(r.Context(), id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to remove global player: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "delete_global_player",
		Target:    targetName,
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// RemoveByName deletes a player from the global list using their gamertag name.
func (h *GlobalPlayerHandler) RemoveByName(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Name) == "" {
		http.Error(w, `{"error": "Player name is required"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteGlobalPlayerByName(r.Context(), strings.TrimSpace(payload.Name)); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to remove global player: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "remove_global_player_by_name",
		Target:    payload.Name,
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}

// SyncAll merges global access lists into all registered servers.
func (h *GlobalPlayerHandler) SyncAll(w http.ResponseWriter, r *http.Request) {
	reports, err := player.SyncAllServersWithGlobal(r.Context(), h.dataDir, h.db, h.engine)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to sync all servers: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "sync_all_servers_global",
		Details:   fmt.Sprintf(`{"synced_count": %d}`, len(reports)),
		ClientIP:  GetClientIP(r).String(),
	})

	if reports == nil {
		reports = make(map[string]*player.SyncReport)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reports)
}

// SyncServer triggers synchronization for a single server instance.
func (h *GlobalPlayerHandler) SyncServer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	report, err := player.SyncServerWithGlobal(r.Context(), h.dataDir, serverID, h.db, h.engine)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to sync server: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "sync_server_global",
		Target:    serverID,
		Details:   fmt.Sprintf(`{"allowlist_added": %d, "permissions_updated": %d}`, len(report.AllowlistAdded), len(report.PermissionsUpdated)),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

// PromotePlayer promotes a player from a server-specific list to the global list.
func (h *GlobalPlayerHandler) PromotePlayer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	var payload struct {
		Name               string `json:"name"`
		XUID               string `json:"xuid"`
		Permission         string `json:"permission"`
		IsAllowlisted      bool   `json:"is_allowlisted"`
		IgnoresPlayerLimit bool   `json:"ignores_player_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, http.StatusBadRequest)
		return
	}

	resolvedName := strings.TrimSpace(payload.Name)
	cleanXUID := strings.TrimSpace(payload.XUID)

	// If name is blank or identical to XUID, attempt to resolve actual Gamertag from local allowlist.json
	if cleanXUID != "" && (resolvedName == "" || resolvedName == cleanXUID) {
		if alPath, err := configfile.SafePath(h.dataDir, serverID, "allowlist.json"); err == nil {
			if entries, err := configfile.ReadAllowlist(alPath); err == nil {
				for _, entry := range entries {
					if entry.XUID == cleanXUID && entry.Name != "" {
						resolvedName = entry.Name
						break
					}
				}
			}
		}
	}

	if resolvedName == "" {
		http.Error(w, `{"error": "Player name or resolvable XUID is required"}`, http.StatusBadRequest)
		return
	}

	gp := &models.GlobalPlayer{
		Name:               resolvedName,
		XUID:               cleanXUID,
		Permission:         payload.Permission,
		IsAllowlisted:      payload.IsAllowlisted,
		IgnoresPlayerLimit: payload.IgnoresPlayerLimit,
	}
	if gp.Permission == "" {
		gp.Permission = "member"
	}

	saved, err := h.db.UpsertGlobalPlayer(r.Context(), gp)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to promote player: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "promote_player_to_global",
		Target:    serverID,
		Details:   fmt.Sprintf(`{"player": "%s", "role": "%s", "allowlist": %t}`, saved.Name, saved.Permission, saved.IsAllowlisted),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(saved)
}

// SetRole changes a global player's permission role (operator, member, visitor) and propagates to servers.
func (h *GlobalPlayerHandler) SetRole(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid player ID"}`, http.StatusBadRequest)
		return
	}

	var payload struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Role) == "" {
		http.Error(w, `{"error": "Role is required"}`, http.StatusBadRequest)
		return
	}

	existing, err := h.db.GetGlobalPlayer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Global player not found"}`, http.StatusNotFound)
		return
	}

	existing.Permission = strings.ToLower(strings.TrimSpace(payload.Role))
	saved, err := h.db.UpdateGlobalPlayer(r.Context(), existing)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to update global player: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Propagate to all servers
	allServers, err := h.db.ListServers(r.Context())
	if err == nil {
		for _, s := range allServers {
			if s.Status == models.ServerStatusRunning && h.engine != nil {
				if existing.Permission == "operator" {
					_ = h.engine.SendConsoleCommand(r.Context(), &s, "op "+existing.Name)
				} else {
					_ = h.engine.SendConsoleCommand(r.Context(), &s, "deop "+existing.Name)
				}
			}
			// Update permissions.json
			if h.dataDir != "" && existing.XUID != "" {
				permPath := filepath.Join(h.dataDir, "servers", s.ID, "permissions.json")
				if perms, err := configfile.ReadPermissions(permPath); err == nil {
					found := false
					for i := range perms {
						if perms[i].XUID == existing.XUID {
							perms[i].Permission = existing.Permission
							found = true
							break
						}
					}
					if !found {
						perms = append(perms, configfile.PermissionEntry{
							Permission: existing.Permission,
							XUID:       existing.XUID,
						})
					}
					_ = configfile.WritePermissions(permPath, perms)
					if s.Status == models.ServerStatusRunning && h.engine != nil {
						_ = h.engine.SendConsoleCommand(r.Context(), &s, "permission reload")
					}
				}
			}
		}
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "set_global_player_role",
		Target:    saved.Name,
		Details:   fmt.Sprintf(`{"role": "%s"}`, saved.Permission),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(saved)
}

