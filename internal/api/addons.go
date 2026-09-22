package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/addon"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/go-chi/chi/v5"
)

type AddonHandler struct {
	db      *database.ManagerDB
	dataDir string
}

func NewAddonHandler(db *database.ManagerDB, dataDir string) *AddonHandler {
	return &AddonHandler{
		db:      db,
		dataDir: dataDir,
	}
}

func (h *AddonHandler) List(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	serverDir := filepath.Join(h.dataDir, "servers", serverID)

	packs, err := addon.ListInstalledPacks(serverDir)
	if err != nil {
		http.Error(w, `{"error": "Failed to list addons"}`, http.StatusInternalServerError)
		return
	}
	if packs == nil {
		packs = []addon.InstalledPack{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(packs)
}

func (h *AddonHandler) Install(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	serverDir := filepath.Join(h.dataDir, "servers", serverID)

	// 100MB max pack upload
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, `{"error": "Invalid multipart form or file too large"}`, http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("addon_file")
	if err != nil {
		http.Error(w, `{"error": "addon_file parameter is required"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	tempFile, err := os.CreateTemp("", "pack_upload_*.tmp")
	if err != nil {
		http.Error(w, `{"error": "Failed to create temp file"}`, http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	size, err := io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, `{"error": "Failed to write temp file"}`, http.StatusInternalServerError)
		return
	}

	pack, err := addon.InstallPack(serverDir, tempFile, size)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(pack)
}

func (h *AddonHandler) Delete(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	packType := chi.URLParam(r, "type")
	folder := chi.URLParam(r, "folder")
	serverDir := filepath.Join(h.dataDir, "servers", serverID)

	if err := addon.DeletePack(serverDir, packType, folder); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to delete addon: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
