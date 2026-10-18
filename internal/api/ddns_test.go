package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ddns"
)

func setupTestDDNSHandler(t *testing.T) (*DDNSHandler, *database.ManagerDB, *ddns.Manager) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.OpenManagerDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	cfg := ddns.Config{
		Enabled:         true,
		Provider:        ddns.ProviderDuckDNS,
		Domain:          "mybds.duckdns.org",
		AuthToken:       "secret-token",
		EnableIPv4:      true,
		IntervalMinutes: 10,
	}

	mgr := ddns.NewManager(cfg, nil, nil, nil)
	mgr.SetRetryDelays(nil)
	handler := NewDDNSHandler(db, mgr)
	return handler, db, mgr
}

func TestDDNSHandler_GetStatusAndConfig(t *testing.T) {
	handler, db, _ := setupTestDDNSHandler(t)
	defer db.Close()

	// 1. Test GetStatus
	req := httptest.NewRequest(http.MethodGet, "/api/ddns/status", nil)
	w := httptest.NewRecorder()
	handler.GetStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var status ddns.Status
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to parse status json: %v", err)
	}
	if !status.Enabled || status.Provider != ddns.ProviderDuckDNS {
		t.Fatalf("unexpected status values: %+v", status)
	}

	// 2. Test GetConfig (redaction)
	reqCfg := httptest.NewRequest(http.MethodGet, "/api/ddns/config", nil)
	wCfg := httptest.NewRecorder()
	handler.GetConfig(wCfg, reqCfg)

	if wCfg.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wCfg.Code)
	}

	var cfg ddns.Config
	if err := json.Unmarshal(wCfg.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("failed to parse config json: %v", err)
	}
	if cfg.AuthToken != "******" {
		t.Fatalf("expected masked token '******', got: %s", cfg.AuthToken)
	}
}

func TestDDNSHandler_UpdateConfig(t *testing.T) {
	handler, db, mgr := setupTestDDNSHandler(t)
	defer db.Close()

	updatePayload := ddns.Config{
		Enabled:             true,
		Provider:            ddns.ProviderCloudflare,
		Domain:              "play.example.com",
		ZoneID:              "zone-123",
		AuthToken:           "******", // Should preserve existing secret
		EnableIPv4:          true,
		EnableIPv6:          true,
		IntervalMinutes:     15,
		AutoSyncGameAddress: true,
	}

	body, _ := json.Marshal(updatePayload)
	req := httptest.NewRequest(http.MethodPost, "/api/ddns/config", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.UpdateConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	updated := mgr.GetConfig()
	if updated.Provider != ddns.ProviderCloudflare || updated.Domain != "play.example.com" {
		t.Fatalf("manager config not updated properly: %+v", updated)
	}
	if updated.AuthToken != "secret-token" {
		t.Fatalf("expected preserved secret-token, got: %s", updated.AuthToken)
	}

	// Verify persistence in DB
	storedDomain, _ := db.GetSetting(context.Background(), "ddns_domain")
	if storedDomain != "play.example.com" {
		t.Fatalf("expected db domain play.example.com, got: %s", storedDomain)
	}
}
