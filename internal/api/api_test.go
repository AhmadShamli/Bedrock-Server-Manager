package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func TestAPIServerUpdateAndValidation(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()
	ctx := context.Background()

	// Create admin user & token
	pwHash, _ := auth.HashPassword("TestPass1234!", 8)
	user, err := db.CreateUser(ctx, "admin_upd", pwHash, models.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	token, _ := auth.GenerateJWT(jwtSecret, user.ID, user.Username, user.Role, time.Hour)

	// Create test server
	srv := &models.Server{
		ID:              "srv-upd-test",
		Name:            "Original Name",
		Version:         "1.21.0.03",
		Port:            19140,
		PortV6:          19141,
		Status:          models.ServerStatusStopped,
		Mode:            "survival",
		Difficulty:      "normal",
		MemoryLimit:     "2G",
		CPULimit:        2.0,
		AutostartOnBoot: false,
		PortGateEnabled: false,
		PortGateMode:    "passphrase",
		PortGateTimeout: 7200,
	}
	if err := db.CreateServer(ctx, srv); err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	// Partial update: rename and change mode without wiping other fields
	updBody := []byte(`{"name":"Updated Realm Name","mode":"creative","autostart_on_boot":true}`)
	req := httptest.NewRequest("PUT", "/api/servers/srv-upd-test", bytes.NewReader(updBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT /api/servers failed: %d, body: %s", w.Code, w.Body.String())
	}

	var updated models.Server
	_ = json.NewDecoder(w.Body).Decode(&updated)
	if updated.Name != "Updated Realm Name" || updated.Mode != "creative" || !updated.AutostartOnBoot {
		t.Errorf("expected updated fields, got: %+v", updated)
	}
	// Verify port was preserved
	if updated.Port != 19140 || updated.MemoryLimit != "2G" {
		t.Errorf("expected original fields to be preserved, got port=%d mem=%s", updated.Port, updated.MemoryLimit)
	}

	// Create another server to test port collision
	srv2 := &models.Server{
		ID:      "srv-upd-collision",
		Name:    "Server 2",
		Version: "1.21.0.03",
		Port:    19150,
		PortV6:  19151,
		Status:  models.ServerStatusStopped,
	}
	_ = db.CreateServer(ctx, srv2)

	// Attempt to update srv to use port 19150 -> should return 409 Conflict
	conflictBody := []byte(`{"port":19150}`)
	req = httptest.NewRequest("PUT", "/api/servers/srv-upd-test", bytes.NewReader(conflictBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for port collision, got %d", w.Code)
	}
}

func TestAPIPortGateKeysCRUD(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()
	ctx := context.Background()

	pwHash, _ := auth.HashPassword("TestPass1234!", 8)
	user, _ := db.CreateUser(ctx, "admin_keys", pwHash, models.RoleAdmin)
	token, _ := auth.GenerateJWT(jwtSecret, user.ID, user.Username, user.Role, time.Hour)

	srv := &models.Server{
		ID:              "srv-keys-test",
		Name:            "Keys Server",
		Port:            19160,
		PortV6:          19161,
		Status:          models.ServerStatusStopped,
		PortGateEnabled: true,
		PortGateTimeout: 3600,
	}
	_ = db.CreateServer(ctx, srv)

	// 1. Create Access Key via POST /api/servers/srv-keys-test/access-keys
	createPayload := []byte(`{"label":"VIP Key","passphrase":"SuperSecretVIPPassphrase","max_uses":5,"lease_duration_seconds":1800}`)
	req := httptest.NewRequest("POST", "/api/servers/srv-keys-test/access-keys", bytes.NewReader(createPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create access key failed: %d, body: %s", w.Code, w.Body.String())
	}
	var createdKey CreateAccessKeyResponse
	_ = json.NewDecoder(w.Body).Decode(&createdKey)
	if createdKey.Label != "VIP Key" || createdKey.PlaintextPassphrase != "SuperSecretVIPPassphrase" || createdKey.ID == 0 {
		t.Fatalf("unexpected created key response: %+v", createdKey)
	}

	// 2. List Access Keys via GET /api/servers/srv-keys-test/access-keys
	req = httptest.NewRequest("GET", "/api/servers/srv-keys-test/access-keys", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list access keys failed: %d", w.Code)
	}
	var keys []models.PortGateKey
	_ = json.NewDecoder(w.Body).Decode(&keys)
	if len(keys) != 1 || keys[0].Label != "VIP Key" {
		t.Fatalf("unexpected keys list: %+v", keys)
	}

	// 3. Delete Access Key via DELETE /api/servers/srv-keys-test/access-keys/{keyId}
	delURL := fmt.Sprintf("/api/servers/srv-keys-test/access-keys/%d", createdKey.ID)
	req = httptest.NewRequest("DELETE", delURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("delete access key failed: %d", w.Code)
	}

	// Verify key was removed
	keysAfter, _ := db.ListPortGateKeys(ctx, &srv.ID)
	if len(keysAfter) != 0 {
		t.Fatalf("expected 0 keys after deletion, got %d", len(keysAfter))
	}
}

func TestAPITasksCRUD(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()
	ctx := context.Background()

	pwHash, _ := auth.HashPassword("TestPass1234!", 8)
	user, _ := db.CreateUser(ctx, "admin_tasks", pwHash, models.RoleAdmin)
	token, _ := auth.GenerateJWT(jwtSecret, user.ID, user.Username, user.Role, time.Hour)

	// 1. Create Task via POST /api/tasks
	createPayload := []byte(`{"name":"Hourly Backup","cron_expr":"0 * * * *","action":"backup"}`)
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewReader(createPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create task failed: %d, body: %s", w.Code, w.Body.String())
	}
	var createdTask models.Task
	_ = json.NewDecoder(w.Body).Decode(&createdTask)
	if createdTask.ID == 0 || createdTask.Name != "Hourly Backup" {
		t.Fatalf("unexpected task: %+v", createdTask)
	}

	// 2. Get Task via GET /api/tasks/{id}
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%d", createdTask.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get task failed: %d", w.Code)
	}

	// 3. Update Task via PUT /api/tasks/{id}
	updatePayload := []byte(`{"name":"Midnight Backup","cron_expr":"0 0 * * *"}`)
	req = httptest.NewRequest("PUT", fmt.Sprintf("/api/tasks/%d", createdTask.ID), bytes.NewReader(updatePayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update task failed: %d, body: %s", w.Code, w.Body.String())
	}
	var updatedTask models.Task
	_ = json.NewDecoder(w.Body).Decode(&updatedTask)
	if updatedTask.Name != "Midnight Backup" || updatedTask.CronExpr != "0 0 * * *" || updatedTask.Action != "backup" {
		t.Fatalf("unexpected updated task: %+v", updatedTask)
	}

	// 4. Toggle Task via POST /api/tasks/{id}/toggle
	req = httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%d/toggle", createdTask.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("toggle task failed: %d", w.Code)
	}

	// 5. Delete Task via DELETE /api/tasks/{id}
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/tasks/%d", createdTask.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete task failed: %d", w.Code)
	}
}

func TestAPIUsersCRUD(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()
	ctx := context.Background()

	pwHash, _ := auth.HashPassword("TestPass1234!", 8)
	user, _ := db.CreateUser(ctx, "superadmin", pwHash, models.RoleAdmin)
	token, _ := auth.GenerateJWT(jwtSecret, user.ID, user.Username, user.Role, time.Hour)

	// 1. Create User via POST /api/users
	createPayload := []byte(`{"username":"operator1","password":"OperatorPassword123!","role":"operator"}`)
	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(createPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create user failed: %d, body: %s", w.Code, w.Body.String())
	}
	var createdUser models.User
	_ = json.NewDecoder(w.Body).Decode(&createdUser)
	if createdUser.Username != "operator1" || createdUser.Role != models.RoleOperator {
		t.Fatalf("unexpected created user: %+v", createdUser)
	}

	// 2. List Users via GET /api/users
	req = httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list users failed: %d", w.Code)
	}
	var users []models.User
	_ = json.NewDecoder(w.Body).Decode(&users)
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	// 3. Grant Server Access via PUT /api/users/{id}/servers
	_ = db.CreateServer(ctx, &models.Server{ID: "server-alpha", Name: "Alpha", Port: 19170, PortV6: 19171})
	_ = db.CreateServer(ctx, &models.Server{ID: "server-beta", Name: "Beta", Port: 19172, PortV6: 19173})

	accessPayload := []byte(`{"server_ids":["server-alpha","server-beta"]}`)
	req = httptest.NewRequest("PUT", fmt.Sprintf("/api/users/%d/servers", createdUser.ID), bytes.NewReader(accessPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update server access failed: %d", w.Code)
	}

	// 4. Get Server Access via GET /api/users/{id}/servers
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/users/%d/servers", createdUser.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get server access failed: %d", w.Code)
	}
	var serverIDs []string
	_ = json.NewDecoder(w.Body).Decode(&serverIDs)
	if len(serverIDs) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(serverIDs))
	}

	// 5. Update Password via PUT /api/users/{id}/password
	newPwPayload := []byte(`{"password":"NewOperatorPass123!"}`)
	req = httptest.NewRequest("PUT", fmt.Sprintf("/api/users/%d/password", createdUser.ID), bytes.NewReader(newPwPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update password failed: %d", w.Code)
	}

	// 6. Delete User via DELETE /api/users/{id}
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/users/%d", createdUser.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("delete user failed: %d", w.Code)
	}
}

func TestAPIManualLeaseValidation(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()
	defer os.RemoveAll("data_test")

	// Create test server
	server := &models.Server{
		ID:        "srv-lease-test",
		Name:      "Lease Test Realm",
		Port:      19132,
		PortV6:    19133,
		Status:    models.ServerStatusStopped,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.CreateServer(context.Background(), server); err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	token, _ := auth.GenerateJWT(jwtSecret, 1, "admin", models.RoleAdmin, 1*time.Hour)

	// Test 1: Invalid IP format
	invalidPayload := []byte(`{"ip_address":"not-an-ip","gamertag":"Player1","duration_minutes":30}`)
	req := httptest.NewRequest("POST", "/api/servers/srv-lease-test/leases/manual", bytes.NewReader(invalidPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid IP, got %d", w.Code)
	}

	// Test 2: Valid IPv4 format
	validPayload := []byte(`{"ip_address":"192.168.1.150","gamertag":"Player1","duration_minutes":30}`)
	req = httptest.NewRequest("POST", "/api/servers/srv-lease-test/leases/manual", bytes.NewReader(validPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid IP, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRequireServerAccessSecurity(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()
	defer os.RemoveAll("data_test")

	// Create test server
	server := &models.Server{
		ID:        "srv-secure-1",
		Name:      "Secure Realm",
		Port:      19140,
		PortV6:    19141,
		Status:    models.ServerStatusStopped,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.CreateServer(context.Background(), server); err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	// Create operator user who does NOT have access to srv-secure-1
	operatorToken, _ := auth.GenerateJWT(jwtSecret, 2, "operator_user", models.RoleOperator, 1*time.Hour)

	req := httptest.NewRequest("GET", "/api/servers/srv-secure-1/stats", nil)
	req.Header.Set("Authorization", "Bearer "+operatorToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for unassigned server, got %d", w.Code)
	}
}

func TestWebSocketOriginValidation(t *testing.T) {
	// 1. Same origin
	req1 := httptest.NewRequest("GET", "http://bsm.local:8080/api/servers/s1/console/ws", nil)
	req1.Header.Set("Origin", "http://bsm.local:8080")
	if !upgrader.CheckOrigin(req1) {
		t.Errorf("expected same origin to be accepted")
	}

	// 2. Empty origin (e.g. non-browser tool)
	req2 := httptest.NewRequest("GET", "http://bsm.local:8080/api/servers/s1/console/ws", nil)
	if !upgrader.CheckOrigin(req2) {
		t.Errorf("expected empty origin to be accepted")
	}

	// 3. Evil external site (CSWSH attempt)
	req3 := httptest.NewRequest("GET", "http://bsm.local:8080/api/servers/s1/console/ws", nil)
	req3.Header.Set("Origin", "https://evil-attacker-site.com")
	if upgrader.CheckOrigin(req3) {
		t.Errorf("expected cross-site evil origin to be rejected")
	}

	// 4. Local dev origin (Vite port 3000 to backend 8080)
	req4 := httptest.NewRequest("GET", "http://localhost:8080/api/servers/s1/console/ws", nil)
	req4.Header.Set("Origin", "http://localhost:3000")
	if !upgrader.CheckOrigin(req4) {
		t.Errorf("expected localhost dev origin to be accepted")
	}
}


