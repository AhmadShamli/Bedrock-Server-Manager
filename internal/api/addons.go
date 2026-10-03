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
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/addon"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/marketplace"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
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

	claims := GetUserClaims(r)
	if claims != nil {
		_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
			UserID:    &claims.UserID,
			ActorType: "user",
			ActorName: claims.Username,
			Action:    "addon_install_upload",
			Target:    serverID,
			Details:   fmt.Sprintf(`{"pack": %q, "type": %q, "version": %q}`, pack.Name, pack.Type, pack.Version),
			ClientIP:  GetClientIP(r).String(),
			Timestamp: time.Now(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(pack)
}

// InstallFromURL allows administrators to install an addon directly from a web URL.
func (h *AddonHandler) InstallFromURL(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil || claims.Role != models.RoleAdmin {
		http.Error(w, `{"error": "Admin privileges required to install addons from URL"}`, http.StatusForbidden)
		return
	}

	serverID := chi.URLParam(r, "id")
	serverDir := filepath.Join(h.dataDir, "servers", serverID)

	var payload struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.URL) == "" {
		http.Error(w, `{"error": "A valid 'url' parameter is required"}`, http.StatusBadRequest)
		return
	}

	pack, err := addon.InstallFromURL(r.Context(), serverDir, strings.TrimSpace(payload.URL))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Installation from URL failed: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    &claims.UserID,
		ActorType: "user",
		ActorName: claims.Username,
		Action:    "addon_install_url",
		Target:    serverID,
		Details:   fmt.Sprintf(`{"url": %q, "pack": %q, "type": %q, "version": %q}`, payload.URL, pack.Name, pack.Type, pack.Version),
		ClientIP:  GetClientIP(r).String(),
		Timestamp: time.Now(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(pack)
}

// MarketplaceSearch searches CurseForge and/or Modrinth for Bedrock addons.
func (h *AddonHandler) MarketplaceSearch(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(r.URL.Query().Get("provider"))
	if provider == "" {
		provider = "all"
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	category := strings.ToLower(r.URL.Query().Get("category"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}

	cfKey, _ := h.db.GetSetting(r.Context(), "curseforge_api_key")
	cfConfigured := cfKey != ""

	var allItems []marketplace.MarketplaceItem
	var totalCount int

	// Search Modrinth
	if provider == "modrinth" || provider == "all" {
		mrResult, err := marketplace.SearchModrinth(r.Context(), nil, query, category, page, pageSize)
		if err == nil && mrResult != nil {
			allItems = append(allItems, mrResult.Items...)
			totalCount += mrResult.Total
		}
	}

	// Search CurseForge
	if provider == "curseforge" || provider == "all" {
		if cfConfigured {
			cfResult, err := marketplace.SearchCurseForge(r.Context(), nil, cfKey, query, category, page, pageSize)
			if err == nil && cfResult != nil {
				allItems = append(allItems, cfResult.Items...)
				totalCount += cfResult.Total
			}
		} else if provider == "curseforge" {
			http.Error(w, `{"error": "CurseForge API key is not configured. Please configure it in System Settings or Marketplace."}`, http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"provider":              provider,
		"items":                 allItems,
		"total":                 totalCount,
		"curseforge_configured": cfConfigured,
	})
}

// MarketplaceInstall downloads and installs a pack selected from CurseForge or Modrinth.
func (h *AddonHandler) MarketplaceInstall(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	serverDir := filepath.Join(h.dataDir, "servers", serverID)

	var payload struct {
		Provider    string `json:"provider"`
		ItemID      string `json:"item_id"`
		FileID      int64  `json:"file_id"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error": "Invalid JSON payload"}`, http.StatusBadRequest)
		return
	}

	provider := strings.ToLower(payload.Provider)
	downloadURL := payload.DownloadURL

	if downloadURL == "" {
		if provider == "curseforge" {
			cfKey, _ := h.db.GetSetting(r.Context(), "curseforge_api_key")
			if cfKey == "" {
				http.Error(w, `{"error": "CurseForge API key is not configured"}`, http.StatusBadRequest)
				return
			}
			modID, err := strconv.ParseInt(payload.ItemID, 10, 64)
			if err != nil {
				http.Error(w, `{"error": "Invalid CurseForge mod ID"}`, http.StatusBadRequest)
				return
			}
			resolvedURL, err := marketplace.GetCurseForgeDownloadURL(r.Context(), nil, cfKey, modID, payload.FileID)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			downloadURL = resolvedURL
		} else if provider == "modrinth" {
			resolvedURL, _, _, err := marketplace.GetModrinthDownloadURL(r.Context(), nil, payload.ItemID)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error": "Modrinth download resolution failed: %s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			downloadURL = resolvedURL
		} else {
			http.Error(w, `{"error": "Unknown marketplace provider: must be curseforge or modrinth"}`, http.StatusBadRequest)
			return
		}
	}

	pack, err := addon.InstallFromURL(r.Context(), serverDir, downloadURL)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to install addon: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	claims := GetUserClaims(r)
	if claims != nil {
		_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
			UserID:    &claims.UserID,
			ActorType: "user",
			ActorName: claims.Username,
			Action:    "addon_marketplace_install",
			Target:    serverID,
			Details:   fmt.Sprintf(`{"provider": %q, "item_id": %q, "pack": %q, "type": %q}`, provider, payload.ItemID, pack.Name, pack.Type),
			ClientIP:  GetClientIP(r).String(),
			Timestamp: time.Now(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(pack)
}

// GetMarketplaceConfig returns the configuration status of marketplace providers.
func (h *AddonHandler) GetMarketplaceConfig(w http.ResponseWriter, r *http.Request) {
	cfKey, _ := h.db.GetSetting(r.Context(), "curseforge_api_key")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"curseforge_configured": cfKey != "",
	})
}

// UpdateMarketplaceConfig updates provider settings (Admin only).
func (h *AddonHandler) UpdateMarketplaceConfig(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil || claims.Role != models.RoleAdmin {
		http.Error(w, `{"error": "Admin privileges required to configure marketplace"}`, http.StatusForbidden)
		return
	}

	var payload struct {
		CurseForgeAPIKey string `json:"curseforge_api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	cleanKey := strings.TrimSpace(payload.CurseForgeAPIKey)
	if err := h.db.SetSetting(r.Context(), "curseforge_api_key", cleanKey); err != nil {
		http.Error(w, `{"error": "Failed to save CurseForge API key"}`, http.StatusInternalServerError)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    &claims.UserID,
		ActorType: "user",
		ActorName: claims.Username,
		Action:    "marketplace_config_update",
		Target:    "system",
		Details:   fmt.Sprintf(`{"curseforge_configured": %t}`, cleanKey != ""),
		ClientIP:  GetClientIP(r).String(),
		Timestamp: time.Now(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":                "updated",
		"curseforge_configured": cleanKey != "",
	})
}

func (h *AddonHandler) Delete(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	packType := chi.URLParam(r, "type")
	folder := chi.URLParam(r, "folder")
	serverDir := filepath.Join(h.dataDir, "servers", serverID)

	if err := addon.DeletePack(serverDir, packType, folder); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to delete addon: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	claims := GetUserClaims(r)
	if claims != nil {
		_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
			UserID:    &claims.UserID,
			ActorType: "user",
			ActorName: claims.Username,
			Action:    "addon_delete",
			Target:    serverID,
			Details:   fmt.Sprintf(`{"folder": %q, "type": %q}`, folder, packType),
			ClientIP:  GetClientIP(r).String(),
			Timestamp: time.Now(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
