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

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/backup"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/go-chi/chi/v5"
)

type BackupHandler struct {
	db      *database.ManagerDB
	eng     engine.ServerEngine
	dataDir string
}

func NewBackupHandler(db *database.ManagerDB, eng engine.ServerEngine, dataDir string) *BackupHandler {
	return &BackupHandler{
		db:      db,
		eng:     eng,
		dataDir: dataDir,
	}
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

	if err := backup.RestoreBackup(r.Context(), server, serverDir, backupZipPath, h.eng); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
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
	tempFile, err := os.CreateTemp("", fmt.Sprintf("%s_export_*.mcworld", server.ID))
	if err != nil {
		http.Error(w, `{"error": "Failed to create temporary export file"}`, http.StatusInternalServerError)
		return
	}
	tempExportFile := tempFile.Name()
	_ = tempFile.Close()
	defer os.Remove(tempExportFile)

	if err := backup.ExportWorld(serverDir, "Bedrock level", tempExportFile); err != nil {
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

	worldName := r.FormValue("world_name")
	if worldName == "" {
		worldName = "Bedrock level"
	}
	cleanWorldName := filepath.Clean(worldName)
	if strings.Contains(cleanWorldName, "..") || strings.Contains(cleanWorldName, "/") || strings.Contains(cleanWorldName, "\\") || filepath.IsAbs(cleanWorldName) {
		http.Error(w, `{"error": "Invalid world_name: path traversal detected"}`, http.StatusBadRequest)
		return
	}

	serverDir := filepath.Join(h.dataDir, "servers", server.ID)
	if err := backup.ImportWorld(serverDir, cleanWorldName, tempFile, size); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to import world: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"world_name": worldName,
		"filename":   header.Filename,
		"size_bytes": size,
	})
}
