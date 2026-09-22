package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/allocator"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/firewall"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ipresolver"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func setupTestRouter(t *testing.T) (*chiMuxWrapper, *database.ManagerDB, *engine.MockEngine, *firewall.MockFirewallDriver, []byte, string) {
	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}

	jwtSecret := []byte("test-jwt-secret-key-32bytes-long!")
	pepper := "test-pepper-1234"
	rateLimiter := auth.NewRateLimiter(5, 5*time.Minute, 10, 5*time.Minute, 15*time.Minute)
	resolver := ipresolver.NewResolver("direct", nil)
	mockEngine := engine.NewMockEngine()
	mockFw := firewall.NewMockFirewallDriver()
	pa := allocator.NewPortAllocator()

	r := NewRouter(RouterOptions{
		DB:            db,
		IPResolver:    resolver,
		RateLimiter:   rateLimiter,
		Engine:        mockEngine,
		Firewall:      mockFw,
		PortAllocator: pa,
		DataDir:       "data_test",
		JWTSecret:     jwtSecret,
		Pepper:        pepper,
		WebFS:         nil,
	})

	return &chiMuxWrapper{r}, db, mockEngine, mockFw, jwtSecret, pepper
}

type chiMuxWrapper struct {
	http.Handler
}

func TestAPISetupAndAuthFlow(t *testing.T) {
	router, db, _, _, _, _ := setupTestRouter(t)
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
	router, db, _, mockFw, jwtSecret, pepper := setupTestRouter(t)
	defer db.Close()

	ctx := t.Context()
	u, _ := db.CreateUser(ctx, "admin", "hash", models.RoleAdmin)
	token, _ := auth.GenerateJWT(jwtSecret, u.ID, u.Username, u.Role, 1*time.Hour)

	// 1. Suggest free ports
	req := httptest.NewRequest("GET", "/api/servers/suggest-ports", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("suggest ports failed: %d", w.Code)
	}
	var ports map[string]int
	_ = json.NewDecoder(w.Body).Decode(&ports)
	if ports["port"] <= 0 || ports["portv6"] <= 0 {
		t.Errorf("invalid suggested ports: %+v", ports)
	}

	// 2. Create Server
	srv := &models.Server{
		ID:              "bsm-world",
		Name:            "Survival World",
		Version:         "1.21.0.03",
		Port:            19132,
		PortV6:          19133,
		Mode:            "survival",
		Difficulty:      "normal",
		PortGateEnabled: true,
		PortGateMode:    "passphrase",
		PortGateTimeout: 3600,
		Status:          "stopped",
	}
	body, _ := json.Marshal(srv)
	req = httptest.NewRequest("POST", "/api/servers", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create server failed: %d, body: %s", w.Code, w.Body.String())
	}

	// 2. Start Server
	req = httptest.NewRequest("POST", "/api/servers/bsm-world/start", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("start server failed: %d", w.Code)
	}

	// 3. Send Console Command
	cmdReq := map[string]string{"command": "time set day"}
	cmdBody, _ := json.Marshal(cmdReq)
	req = httptest.NewRequest("POST", "/api/servers/bsm-world/command", bytes.NewReader(cmdBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("send command failed: %d", w.Code)
	}

	// 4. Create Port Gate Key
	keyHash := auth.HashAccessKey(pepper, "open-sesame")
	srvID := "bsm-world"
	_ = db.CreatePortGateKey(ctx, &models.PortGateKey{
		ServerID:  &srvID,
		Label:     "Secret Door",
		KeyHash:   keyHash,
		KeyPrefix: "open-",
		IsActive:  true,
	})

	// 5. Test Knock Config
	req = httptest.NewRequest("GET", "/api/knock/bsm-world/config", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("knock config failed: %d", w.Code)
	}

	// 6. Test Knock Attempt
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

	// Verify firewall rule was created for 203.0.113.50
	if !mockFw.HasRule("203.0.113.50", 19132) {
		t.Fatalf("expected firewall rule for 203.0.113.50:19132")
	}

	var knockRes map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&knockRes)
	sessionToken := knockRes["session_token"].(string)

	// 7. Test Heartbeat (Mobile Roaming with IP change to 198.51.100.99)
	req = httptest.NewRequest("POST", "/api/knock/bsm-world/heartbeat", nil)
	req.RemoteAddr = "198.51.100.99:55210"
	req.Header.Set("X-Knock-Token", sessionToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("heartbeat failed: %d, body: %s", w.Code, w.Body.String())
	}

	// Verify old IP rule revoked and new IP rule added in firewall
	if mockFw.HasRule("203.0.113.50", 19132) {
		t.Fatalf("expected old IP 203.0.113.50 rule to be revoked")
	}
	if !mockFw.HasRule("198.51.100.99", 19132) {
		t.Fatalf("expected new IP 198.51.100.99 rule to be allowed")
	}

	// 8. Admin List Leases
	req = httptest.NewRequest("GET", "/api/servers/bsm-world/leases", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list leases failed: %d", w.Code)
	}
	var leases []models.PortGateLease
	_ = json.NewDecoder(w.Body).Decode(&leases)
	if len(leases) != 1 || leases[0].IPAddress != "198.51.100.99" {
		t.Fatalf("unexpected leases: %+v", leases)
	}

	// 9. Admin Revoke Lease
	req = httptest.NewRequest("POST", "/api/servers/bsm-world/leases/1/revoke", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke lease failed: %d", w.Code)
	}
	if mockFw.HasRule("198.51.100.99", 19132) {
		t.Fatalf("expected revoked lease firewall rule to be removed")
	}

	// 10. Stop Server
	req = httptest.NewRequest("POST", "/api/servers/bsm-world/stop", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("stop server failed: %d", w.Code)
	}
}

func TestAPICopyConfigs(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()
	ctx := t.Context()

	// 1. Create admin user and token
	adminUser, err := db.CreateUser(ctx, "admin_copy", "hashed", models.RoleAdmin)
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}
	token, _ := auth.GenerateJWT(jwtSecret, adminUser.ID, adminUser.Username, adminUser.Role, 1*time.Hour)

	// 2. Create source and target servers in DB
	srcServer := &models.Server{
		ID:     "srv-alpha",
		Name:   "Server Alpha",
		Port:   19132,
		PortV6: 19133,
		Status: models.ServerStatusStopped,
	}
	_ = db.CreateServer(ctx, srcServer)

	tgtServer := &models.Server{
		ID:     "srv-beta",
		Name:   "Server Beta",
		Port:   19142,
		PortV6: 19143,
		Status: models.ServerStatusStopped,
	}
	_ = db.CreateServer(ctx, tgtServer)

	// Prepare data_test/servers/srv-alpha/allowlist.json
	srcDir := "data_test/servers/srv-alpha"
	_ = os.MkdirAll(srcDir, 0755)
	defer os.RemoveAll("data_test")

	_ = os.WriteFile(srcDir+"/allowlist.json", []byte(`[{"name":"Steve","ignoresPlayerLimit":false}]`), 0644)

	// 3. Make copy-configs request
	reqBody := []byte(`{"target_server_ids":["srv-beta"],"copy_allowlist":true,"mode":"replace"}`)
	req := httptest.NewRequest("POST", "/api/servers/srv-alpha/copy-configs", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("copy-configs failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Verify target srv-beta has allowlist.json
	tgtAllowlist := "data_test/servers/srv-beta/allowlist.json"
	data, err := os.ReadFile(tgtAllowlist)
	if err != nil {
		t.Fatalf("expected target allowlist to exist: %v", err)
	}
	if !bytes.Contains(data, []byte("Steve")) {
		t.Fatalf("expected target allowlist to contain Steve, got %s", string(data))
	}
}

func TestAPIGlobalPlayers(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()
	ctx := t.Context()

	// 1. Create admin user and token
	adminUser, err := db.CreateUser(ctx, "admin_gp", "hashed", models.RoleAdmin)
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}
	token, _ := auth.GenerateJWT(jwtSecret, adminUser.ID, adminUser.Username, adminUser.Role, 1*time.Hour)

	// 2. Create a server
	server := &models.Server{
		ID:     "srv-sync-test",
		Name:   "Sync Test Realm",
		Port:   19132,
		PortV6: 19133,
		Status: models.ServerStatusStopped,
	}
	_ = db.CreateServer(ctx, server)

	serverDir := "data_test/servers/srv-sync-test"
	_ = os.MkdirAll(serverDir, 0755)
	defer os.RemoveAll("data_test")

	// 3. Create Global Player via POST /api/global-players
	createBody := []byte(`{"name":"UniversalPlayer","xuid":"999999999","is_allowlisted":true,"permission":"operator"}`)
	req := httptest.NewRequest("POST", "/api/global-players", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create global player failed: %d, body: %s", w.Code, w.Body.String())
	}

	// 4. List Global Players
	req = httptest.NewRequest("GET", "/api/global-players", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list global players failed: %d", w.Code)
	}
	var gps []*models.GlobalPlayer
	_ = json.NewDecoder(w.Body).Decode(&gps)
	if len(gps) != 1 || gps[0].Name != "UniversalPlayer" {
		t.Fatalf("unexpected global players list: %+v", gps)
	}

	// 5. Trigger Sync on server via POST /api/servers/srv-sync-test/sync-global
	req = httptest.NewRequest("POST", "/api/servers/srv-sync-test/sync-global", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("sync-global failed: %d, body: %s", w.Code, w.Body.String())
	}

	// Verify server disk allowlist now contains UniversalPlayer
	alData, err := os.ReadFile(serverDir + "/allowlist.json")
	if err != nil || !bytes.Contains(alData, []byte("UniversalPlayer")) {
		t.Fatalf("expected server allowlist to contain UniversalPlayer: %v", err)
	}

	// 6. Promote local player to global via POST /api/servers/srv-sync-test/players/promote-global
	promoteBody := []byte(`{"name":"LocalPromoted","xuid":"888888","is_allowlisted":true,"permission":"member"}`)
	req = httptest.NewRequest("POST", "/api/servers/srv-sync-test/players/promote-global", bytes.NewReader(promoteBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("promote player failed: %d, body: %s", w.Code, w.Body.String())
	}

	// Verify player is now in global list
	promoted, err := db.GetGlobalPlayerByName(ctx, "LocalPromoted")
	if err != nil || promoted.Name != "LocalPromoted" {
		t.Fatalf("expected LocalPromoted in DB: %v", err)
	}

	// 7. Remove from global via POST /api/global-players/remove-by-name
	removeBody := []byte(`{"name":"LocalPromoted"}`)
	req = httptest.NewRequest("POST", "/api/global-players/remove-by-name", bytes.NewReader(removeBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("remove-by-name failed: %d", w.Code)
	}

	afterRemove, _ := db.GetGlobalPlayerByName(ctx, "LocalPromoted")
	if afterRemove != nil {
		t.Fatalf("expected LocalPromoted to be removed from global")
	}
}
