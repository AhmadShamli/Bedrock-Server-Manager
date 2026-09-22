package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ipresolver"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func setupTestRouter(t *testing.T) (*chiMuxWrapper, *database.ManagerDB, []byte, string) {
	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}

	jwtSecret := []byte("test-jwt-secret-key-32bytes-long!")
	pepper := "test-pepper-1234"
	rateLimiter := auth.NewRateLimiter(5, 5*time.Minute, 10, 5*time.Minute, 15*time.Minute)
	resolver := ipresolver.NewResolver("direct", nil)

	r := NewRouter(RouterOptions{
		DB:          db,
		IPResolver:  resolver,
		RateLimiter: rateLimiter,
		JWTSecret:   jwtSecret,
		Pepper:      pepper,
		WebFS:       nil,
	})

	return &chiMuxWrapper{r}, db, jwtSecret, pepper
}

type chiMuxWrapper struct {
	http.Handler
}

func TestAPISetupAndAuthFlow(t *testing.T) {
	router, db, _, _ := setupTestRouter(t)
	defer db.Close()

	// 1. Check setup status initially -> needs_setup: true
	req := httptest.NewRequest("GET", "/api/setup/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var statusRes map[string]bool
	_ = json.NewDecoder(w.Body).Decode(&statusRes)
	if !statusRes["needs_setup"] {
		t.Errorf("expected needs_setup to be true")
	}

	// 2. Run initial setup
	setupPayload := SetupRequest{
		Username: "masteradmin",
		Password: "Password1234!",
	}
	body, _ := json.Marshal(setupPayload)
	req = httptest.NewRequest("POST", "/api/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("setup failed: code %d, body: %s", w.Code, w.Body.String())
	}
	var setupRes map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&setupRes)
	token, ok := setupRes["token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected token in setup response")
	}

	// 3. Second setup call should fail with 409 Conflict
	req = httptest.NewRequest("POST", "/api/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", w.Code)
	}

	// 4. Test /api/auth/me with the issued token
	req = httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("me endpoint failed: %d, body: %s", w.Code, w.Body.String())
	}

	// 5. Test login
	loginPayload := LoginRequest{
		Username: "masteradmin",
		Password: "Password1234!",
	}
	lBody, _ := json.Marshal(loginPayload)
	req = httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(lBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d, body: %s", w.Code, w.Body.String())
	}
}

func TestServerAndKnockFlow(t *testing.T) {
	router, db, jwtSecret, pepper := setupTestRouter(t)
	defer db.Close()

	// Create admin user & token
	ctx := t.Context()
	u, _ := db.CreateUser(ctx, "admin", "hash", models.RoleAdmin)
	token, _ := auth.GenerateJWT(jwtSecret, u.ID, u.Username, u.Role, 1*time.Hour)

	// 1. Create Server via API
	srv := models.Server{
		ID:              "bsm-world",
		Name:            "Bedrock Realm",
		Port:            19132,
		PortV6:          19133,
		PortGateEnabled: true,
		PortGateMode:    models.PortGateModePassphrase,
		PortGateTimeout: 3600,
	}
	body, _ := json.Marshal(srv)
	req := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create server failed: %d, body: %s", w.Code, w.Body.String())
	}

	// 2. Add an access key for this server
	keyHash := auth.HashAccessKey(pepper, "open-sesame")
	srvID := "bsm-world"
	_ = db.CreatePortGateKey(ctx, &models.PortGateKey{
		ServerID:  &srvID,
		Label:     "Secret Door",
		KeyHash:   keyHash,
		KeyPrefix: "open-",
		IsActive:  true,
	})

	// 3. Test Knock Config
	req = httptest.NewRequest("GET", "/api/knock/bsm-world/config", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("knock config failed: %d", w.Code)
	}

	// 4. Test Knock Attempt
	knockReq := KnockRequest{
		Passphrase: "open-sesame",
	}
	kBody, _ := json.Marshal(knockReq)
	req = httptest.NewRequest("POST", "/api/knock/bsm-world", bytes.NewReader(kBody))
	req.RemoteAddr = "203.0.113.50:44321"
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("knock authorization failed: %d, body: %s", w.Code, w.Body.String())
	}

	var knockRes map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&knockRes)
	sessionToken := knockRes["session_token"].(string)

	// 5. Test Heartbeat (Mobile Roaming with IP change)
	req = httptest.NewRequest("POST", "/api/knock/bsm-world/heartbeat", nil)
	req.RemoteAddr = "198.51.100.99:55210" // Simulates player moving to 5G network
	req.Header.Set("X-Knock-Token", sessionToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("heartbeat failed: %d, body: %s", w.Code, w.Body.String())
	}
	var hbRes map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&hbRes)
	if !hbRes["ip_updated"].(bool) {
		t.Errorf("expected ip_updated to be true on roaming")
	}
}
