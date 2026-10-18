package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/firewall"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ipresolver"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/turnstile"
)

func TestTurnstileAPIProtection(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// Mock Cloudflare siteverify endpoint
	cfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		token := r.FormValue("response")
		secret := r.FormValue("secret")

		w.Header().Set("Content-Type", "application/json")
		if secret == "test-cf-secret" && token == "valid-turnstile-token" {
			_ = json.NewEncoder(w).Encode(turnstile.SiteVerifyResponse{
				Success: true,
			})
		} else {
			_ = json.NewEncoder(w).Encode(turnstile.SiteVerifyResponse{
				Success:    false,
				ErrorCodes: []string{"invalid-input-response"},
			})
		}
	}))
	defer cfServer.Close()

	verifier := turnstile.NewVerifier()
	verifier.SetVerifyURL(cfServer.URL)

	ipRes := ipresolver.NewResolver("direct", nil)
	rateLimiter := auth.NewRateLimiter(5, 5*time.Minute, 10, 5*time.Minute, 15*time.Minute)
	mockEngine := engine.NewMockEngine()
	mockFw := firewall.NewMockFirewallDriver()

	router := NewRouter(RouterOptions{
		DB:                db,
		IPResolver:        ipRes,
		RateLimiter:       rateLimiter,
		Engine:            mockEngine,
		Firewall:          mockFw,
		DataDir:           t.TempDir(),
		JWTSecret:         []byte("12345678901234567890123456789012"),
		Pepper:            "test-pepper",
		TurnstileVerifier: verifier,
	})

	// 1. Verify GET /api/system/turnstile-config (Default disabled)
	req := httptest.NewRequest(http.MethodGet, "/api/system/turnstile-config", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var conf turnstile.Config
	if err := json.Unmarshal(rr.Body.Bytes(), &conf); err != nil {
		t.Fatalf("failed to parse turnstile config: %v", err)
	}
	if conf.Enabled {
		t.Errorf("expected Turnstile to be disabled by default")
	}

	// 2. Enable Turnstile in database
	_ = db.SetSetting(ctx, turnstile.SettingTurnstileEnabled, "true")
	_ = db.SetSetting(ctx, turnstile.SettingTurnstileSiteKey, "test-cf-sitekey")
	_ = db.SetSetting(ctx, turnstile.SettingTurnstileSecretKey, "test-cf-secret")
	_ = db.SetSetting(ctx, turnstile.SettingTurnstileLoginEnabled, "true")
	_ = db.SetSetting(ctx, turnstile.SettingTurnstileKnockEnabled, "true")

	// Create a test user for login test
	pwHash, _ := auth.HashPassword("password123", 8)
	_, _ = db.CreateUser(ctx, "testuser", pwHash, models.RoleUser)

	// 3. Attempt login without token -> Should fail with 403 Forbidden
	loginPayload := map[string]interface{}{
		"username": "testuser",
		"password": "password123",
	}
	body, _ := json.Marshal(loginPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden without token, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// 4. Attempt login with invalid token -> Should fail with 403 Forbidden
	loginPayload["turnstile_token"] = "invalid-token"
	body, _ = json.Marshal(loginPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden with invalid token, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// 5. Attempt login with valid token -> Should succeed with 200 OK
	loginPayload["turnstile_token"] = "valid-turnstile-token"
	body, _ = json.Marshal(loginPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK with valid token, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// 6. Test Knock portal protection
	// Create server with port gate enabled
	server := &models.Server{
		ID:              "test-srv-turnstile",
		Name:            "Turnstile Server",
		Port:            19132,
		PortGateEnabled: true,
		PortGateMode:    models.PortGateModeGamertag,
		PortGateTimeout: 3600,
	}
	_ = db.CreateServer(ctx, server)

	// Knock without token -> Should fail with 403 Forbidden
	knockPayload := map[string]interface{}{
		"gamertag": "TestSteve",
	}
	body, _ = json.Marshal(knockPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/knock/test-srv-turnstile/knock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden on knock without token, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// Knock with valid token -> Should succeed with 200 OK
	knockPayload["turnstile_token"] = "valid-turnstile-token"
	body, _ = json.Marshal(knockPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/knock/test-srv-turnstile/knock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK on knock with valid token, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}
