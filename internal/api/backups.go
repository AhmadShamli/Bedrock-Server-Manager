package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/addon"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/backup"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/configfile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/go-chi/chi/v5"
)

type BackupHandler struct {
	db            *database.ManagerDB
	eng           engine.ServerEngine
	dataDir       string
	serverHandler *ServerHandler
}

func NewBackupHandler(db *database.ManagerDB, eng engine.ServerEngine, dataDir string) *BackupHandler {
	return &BackupHandler{
		db:      db,
		eng:     eng,
		dataDir: dataDir,
	}
}

func (h *BackupHandler) SetServerHandler(sh *ServerHandler) {
	h.serverHandler = sh
}

// ListBackups returns all backups for the given server.
func (h *BackupHandler) ListBackups(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	backups, err := h.db.ListBackups(r.Context(), serverID)
	if err != nil {
		http.Error(w, `{"error": "Failed to list backups"}`, http.StatusInternalServerError)
		return
	}
	if backups == nil {
		backups = []models.Backup{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(backups)
}

// CreateBackupRequest payload.
type CreateBackupRequest struct {
	Type     string `json:"type"`
	IsLocked bool   `json:"is_locked"`
}

// CreateBackup triggers a zero-downtime hot backup of the server's world state.
func (h *BackupHandler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil || server == nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	var req CreateBackupRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Type == "" {
		req.Type = "manual"
	}

	serverDir := filepath.Join(h.dataDir, "servers", server.ID)
	backupDir := filepath.Join(h.dataDir, "backups", server.ID)

	b, err := backup.CreateHotBackup(r.Context(), server, serverDir, backupDir, h.eng, req.Type, req.IsLocked, h.db)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Backup creation failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	claims := GetUserClaims(r)
	actorName := "user"
	if claims != nil {
		actorName = claims.Username
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: actorName,
		Action:    "backup_created",
		Target:    server.ID,
		Details:   fmt.Sprintf(`{"backup_id": %d, "filename": "%s", "size_bytes": %d}`, b.ID, b.Filename, b.SizeBytes),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(b)
}

// ToggleLock toggles pin/lock protection on a backup.
func (h *BackupHandler) ToggleLock(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	backupIDStr := chi.URLParam(r, "backupId")
	backupID, err := strconv.ParseInt(backupIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid backup ID"}`, http.StatusBadRequest)
		return
	}

	b, err := h.db.GetBackup(r.Context(), backupID)
	if err != nil || b == nil || b.ServerID != serverID {
		http.Error(w, `{"error": "Backup not found"}`, http.StatusNotFound)
		return
	}

	newLockedState := !b.IsLocked
	if err := h.db.ToggleBackupLock(r.Context(), backupID, newLockedState); err != nil {
		http.Error(w, `{"error": "Failed to update backup lock status"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":        backupID,
		"is_locked": newLockedState,
	})
}

// DeleteBackup deletes a backup archive and record.
func (h *BackupHandler) DeleteBackup(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	backupIDStr := chi.URLParam(r, "backupId")
	backupID, err := strconv.ParseInt(backupIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid backup ID"}`, http.StatusBadRequest)
		return
	}

	b, err := h.db.GetBackup(r.Context(), backupID)
	if err != nil || b == nil || b.ServerID != serverID {
		http.Error(w, `{"error": "Backup not found"}`, http.StatusNotFound)
		return
	}

	if b.IsLocked {
		http.Error(w, `{"error": "Cannot delete a pinned/locked backup. Unlock it first."}`, http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(h.dataDir, "backups", serverID, b.Filename)
	_ = os.Remove(filePath)

	if err := h.db.DeleteBackup(r.Context(), backupID); err != nil {
		http.Error(w, `{"error": "Failed to delete backup from database"}`, http.StatusInternalServerError)
		return
	}

	claims := GetUserClaims(r)
	actorName := "user"
	if claims != nil {
		actorName = claims.Username
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: actorName,
		Action:    "backup_deleted",
		Target:    serverID,
		Details:   fmt.Sprintf(`{"backup_id": %d, "filename": "%s"}`, b.ID, b.Filename),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// DownloadBackup streams the backup zip archive to the client.
func (h *BackupHandler) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	backupIDStr := chi.URLParam(r, "backupId")
	backupID, err := strconv.ParseInt(backupIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid backup ID"}`, http.StatusBadRequest)
		return
	}

	b, err := h.db.GetBackup(r.Context(), backupID)
	if err != nil || b == nil || b.ServerID != serverID {
		http.Error(w, `{"error": "Backup not found"}`, http.StatusNotFound)
		return
	}

	filePath := filepath.Join(h.dataDir, "backups", serverID, b.Filename)
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, `{"error": "Backup file not found on disk"}`, http.StatusNotFound)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", b.Filename))
	w.Header().Set("Content-Type", "application/zip")
	_, _ = io.Copy(w, file)
}

// RestoreBackup safely rolls back the server's world to the selected backup.
func (h *BackupHandler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil || server == nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	backupIDStr := chi.URLParam(r, "backupId")
	backupID, err := strconv.ParseInt(backupIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid backup ID"}`, http.StatusBadRequest)
		return
	}

	b, err := h.db.GetBackup(r.Context(), backupID)
	if err != nil || b == nil || b.ServerID != serverID {
		http.Error(w, `{"error": "Backup not found"}`, http.StatusNotFound)
		return
	}

	serverDir := filepath.Join(h.dataDir, "servers", server.ID)
	backupZipPath := filepath.Join(h.dataDir, "backups", server.ID, b.Filename)

	actorName := "user"
	claims := GetUserClaims(r)
	if claims != nil {
		actorName = claims.Username
	}

	restoreAction := func() error {
		return backup.RestoreBackup(r.Context(), server, serverDir, backupZipPath, h.eng)
	}

	var restoreErr error
	if h.serverHandler != nil {
		restoreErr = h.serverHandler.WithServerStopped(r.Context(), server, "user", actorName, "backup_restore", GetClientIP(r).String(), restoreAction)
	} else {
		restoreErr = restoreAction()
	}

	if restoreErr != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, restoreErr.Error()), http.StatusBadRequest)
		return
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: actorName,
		Action:    "backup_restored",
		Target:    server.ID,
		Details:   fmt.Sprintf(`{"backup_id": %d, "filename": "%s"}`, b.ID, b.Filename),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ExportWorld streams the active world as a downloadable .mcworld archive.
func (h *BackupHandler) ExportWorld(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil || server == nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	serverDir := filepath.Join(h.dataDir, "servers", server.ID)
	levelName := addon.GetActiveLevelName(serverDir)
	tempFile, err := os.CreateTemp("", fmt.Sprintf("%s_export_*.mcworld", server.ID))
	if err != nil {
		http.Error(w, `{"error": "Failed to create temporary export file"}`, http.StatusInternalServerError)
		return
	}
	tempExportFile := tempFile.Name()
	_ = tempFile.Close()
	defer os.Remove(tempExportFile)

	if err := backup.ExportWorld(serverDir, levelName, tempExportFile); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to export world: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	file, err := os.Open(tempExportFile)
	if err != nil {
		http.Error(w, `{"error": "Failed to read export file"}`, http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.mcworld\"", server.Name))
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.Copy(w, file)
}

// ImportWorld handles uploading a .mcworld or .zip archive and extracting it into worlds.
func (h *BackupHandler) ImportWorld(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil || server == nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	if server.Status == models.ServerStatusRunning {
		http.Error(w, `{"error": "Server must be stopped before importing a new world"}`, http.StatusBadRequest)
		return
	}

	// 500MB max world upload
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		http.Error(w, `{"error": "Invalid multipart form or world file too large"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("world_file")
	if err != nil {
		http.Error(w, `{"error": "world_file parameter is required"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	tempFile, err := os.CreateTemp("", "world_upload_*.tmp")
	if err != nil {
		http.Error(w, `{"error": "Failed to create temp buffer file"}`, http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	size, err := io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, `{"error": "Failed to write temp world file"}`, http.StatusInternalServerError)
		return
	}

	serverDir := filepath.Join(h.dataDir, "servers", server.ID)
	worldName := r.FormValue("world_name")
	if worldName == "" {
		worldName = addon.GetActiveLevelName(serverDir)
	}
	cleanWorldName := filepath.Clean(worldName)
	if strings.Contains(cleanWorldName, "..") || strings.Contains(cleanWorldName, "/") || strings.Contains(cleanWorldName, "\\") || filepath.IsAbs(cleanWorldName) {
		http.Error(w, `{"error": "Invalid world_name: path traversal detected"}`, http.StatusBadRequest)
		return
	}

	if err := backup.ImportWorld(serverDir, cleanWorldName, tempFile, size); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to import world: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Ensure active addons remain synced with the imported world
	addon.SyncActivePacksToWorlds(serverDir)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"world_name": worldName,
		"filename":   header.Filename,
		"size_bytes": size,
	})
}

// ResetWorldRequest configures options when resetting a server's world data.
type ResetWorldRequest struct {
	CreateBackup bool    `json:"create_backup"`
	KeepAddons   *bool   `json:"keep_addons"`
	Seed         *string `json:"seed"`
}

// ResetWorld wipes all world data for the instance so it generates a clean new world.
// If the server is currently running, it is gracefully stopped, cleaned, and restarted.
func (h *BackupHandler) ResetWorld(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), serverID)
	if err != nil || server == nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	var req ResetWorldRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	keepAddons := true
	if req.KeepAddons != nil {
		keepAddons = *req.KeepAddons
	}

	serverDir := filepath.Join(h.dataDir, "servers", server.ID)
	backupDir := filepath.Join(h.dataDir, "backups", server.ID)
	levelName := addon.GetActiveLevelName(serverDir)

	// 1. Optional pre-reset backup if worlds directory exists
	var createdBackup *models.Backup
	if req.CreateBackup {
		if _, err := os.Stat(filepath.Join(serverDir, "worlds")); err == nil {
			if b, bErr := backup.CreateHotBackup(r.Context(), server, serverDir, backupDir, h.eng, "pre-reset", false, h.db); bErr == nil {
				createdBackup = b
			}
		}
	}

	// 2. Capture active addon pack bindings before wiping worlds/
	var behaviorRefs, resourceRefs []addon.WorldPackRef
	if keepAddons {
		behaviorRefs, _ = addon.GetWorldPackRefs(serverDir, levelName, "behavior")
		resourceRefs, _ = addon.GetWorldPackRefs(serverDir, levelName, "resource")
	}

	wasRunning := false
	if h.eng != nil {
		if st, err := h.eng.GetServerStatus(r.Context(), server); err == nil && (st == models.ServerStatusRunning || st == models.ServerStatusStarting) {
			wasRunning = true
		} else if server.Status == models.ServerStatusRunning || server.Status == models.ServerStatusStarting {
			wasRunning = true
		}
	} else if server.Status == models.ServerStatusRunning || server.Status == models.ServerStatusStarting {
		wasRunning = true
	}

	actorName := "user"
	claims := GetUserClaims(r)
	if claims != nil {
		actorName = claims.Username
	}

	resetAction := func() error {
		// Wipe existing world data cleanly
		if err := backup.ResetWorld(serverDir); err != nil {
			return fmt.Errorf("failed to reset world data: %w", err)
		}

		// Optional seed update for the new world
		if req.Seed != nil {
			newSeed := strings.TrimSpace(*req.Seed)
			server.Seed = newSeed
			_ = h.db.UpdateServer(r.Context(), server)

			propPath := filepath.Join(serverDir, "server.properties")
			updated, _ := configfile.UpdateExistingPropertyFile(propPath, map[string]string{
				"level-seed": newSeed,
			})
			if !updated {
				existingProps, existingKeys, ok, _ := configfile.ReadPendingProperties(serverDir)
				if !ok || existingProps == nil {
					existingProps, existingKeys = configfile.MergeDefaultProperties(make(map[string]string), nil, server)
				}
				mergedProps, mergedKeys := configfile.MergeProperties(existingProps, existingKeys, map[string]string{
					"level-seed": newSeed,
				})
				_ = configfile.WritePendingProperties(serverDir, mergedProps, mergedKeys)
			}
		}

		// Restore active addon pack bindings into the fresh world directory
		if keepAddons {
			if len(behaviorRefs) > 0 {
				_ = addon.SaveWorldPackRefs(serverDir, levelName, "behavior", behaviorRefs)
			}
			if len(resourceRefs) > 0 {
				_ = addon.SaveWorldPackRefs(serverDir, levelName, "resource", resourceRefs)
			}
			addon.SyncActivePacksToWorlds(serverDir)
		}
		return nil
	}

	if h.serverHandler != nil {
		if err := h.serverHandler.WithServerStopped(r.Context(), server, "user", actorName, "world_reset", GetClientIP(r).String(), resetAction); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
	} else {
		if err := resetAction(); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
	}

	restarted := wasRunning

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: actorName,
		Action:    "world_reset",
		Target:    server.ID,
		Details:   fmt.Sprintf(`{"world_name": "%s", "keep_addons": %t, "create_backup": %t, "restarted": %t, "seed": "%s"}`, levelName, keepAddons, req.CreateBackup, restarted, server.Seed),
		ClientIP:  GetClientIP(r).String(),
	})

	resp := map[string]interface{}{
		"success":     true,
		"world_name":  levelName,
		"keep_addons": keepAddons,
		"restarted":   restarted,
		"seed":        server.Seed,
	}
	if createdBackup != nil {
		resp["backup"] = createdBackup
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

