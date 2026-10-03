package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/addon"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/allocator"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/configfile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/firewall"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ipresolver"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/player"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/preset"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/version"
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

	// 10. Test Permanent Allowlist Rule (Always Allowed CIDR / IP)
	allowReq := map[string]interface{}{
		"ip_or_subnet": "10.50.0.0/16",
		"is_global":    true,
		"comment":      "Permanent Office Subnet",
	}
	aBody, _ := json.Marshal(allowReq)
	req = httptest.NewRequest("POST", "/api/servers/bsm-world/portgate/allowlist", bytes.NewReader(aBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create allow rule failed: %d, body: %s", w.Code, w.Body.String())
	}

	// Verify firewall rule created for the subnet
	if !mockFw.HasRule("10.50.0.0/16", 19132) {
		t.Fatalf("expected firewall rule for 10.50.0.0/16:19132")
	}

	// 11. Visitor from IP in 10.50.0.0/16 checks status: should be always_allowed = true
	req = httptest.NewRequest("GET", "/api/knock/bsm-world/status", nil)
	req.RemoteAddr = "10.50.12.34:55123"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("knock status failed: %d", w.Code)
	}
	var statusRes map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&statusRes)
	if statusRes["always_allowed"] != true || statusRes["active"] != true {
		t.Fatalf("expected always_allowed: true, got %+v", statusRes)
	}

	// 12. Visitor from IP in 10.50.0.0/16 knocks: should succeed without passphrase/gamertag
	req = httptest.NewRequest("POST", "/api/knock/bsm-world", bytes.NewReader([]byte("{}")))
	req.RemoteAddr = "10.50.12.34:55123"
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("knock for always-allowed IP failed: %d, body: %s", w.Code, w.Body.String())
	}
	var knockPermRes map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&knockPermRes)
	if knockPermRes["always_allowed"] != true {
		t.Fatalf("expected knockPermRes always_allowed: true, got %+v", knockPermRes)
	}

	// 13. Stop Server
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

	// 2.5. Verify GET /api/global-players on empty list returns '[]' and NOT 'null'
	reqEmpty := httptest.NewRequest("GET", "/api/global-players", nil)
	reqEmpty.Header.Set("Authorization", "Bearer "+token)
	wEmpty := httptest.NewRecorder()
	router.ServeHTTP(wEmpty, reqEmpty)
	if wEmpty.Code != http.StatusOK {
		t.Fatalf("list global players on empty failed: %d", wEmpty.Code)
	}
	if strings.TrimSpace(wEmpty.Body.String()) != "[]" {
		t.Fatalf("expected '[]' on empty global players list, got '%s'", wEmpty.Body.String())
	}

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

	// 8. Test SetRole via POST /api/global-players/{id}/role
	roleBody := []byte(`{"role":"member"}`)
	req = httptest.NewRequest("POST", fmt.Sprintf("/api/global-players/%d/role", gps[0].ID), bytes.NewReader(roleBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("set-role failed: %d, body: %s", w.Code, w.Body.String())
	}
	updatedGp, _ := db.GetGlobalPlayer(ctx, gps[0].ID)
	if updatedGp == nil || updatedGp.Permission != "member" {
		t.Fatalf("expected permission to be 'member', got: %+v", updatedGp)
	}

	// 9. Test GET /api/active-players
	req = httptest.NewRequest("GET", "/api/active-players", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get active-players failed: %d", w.Code)
	}

	// 10. Test Direct Global Ban via POST /api/banned-players
	banBody := []byte(`{"gamertag":"UniversalPlayer","xuid":"999999999","reason":"Griefing globally","scope":"global"}`)
	req = httptest.NewRequest("POST", "/api/banned-players", bytes.NewReader(banBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("direct global ban failed: %d, body: %s", w.Code, w.Body.String())
	}

	// Verify player was removed from global_players
	bannedGp, _ := db.GetGlobalPlayerByName(ctx, "UniversalPlayer")
	if bannedGp != nil {
		t.Fatalf("expected UniversalPlayer to be removed from global players on global ban")
	}
	// Verify player is in banned_players
	isBanned, _, bErr := db.IsPlayerBanned(ctx, "any-server", "UniversalPlayer", "999999999")
	if bErr != nil || !isBanned {
		t.Fatalf("expected player to be banned globally")
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

	// Update seed
	seedBody := []byte(`{"seed":"custom-bedrock-seed-999"}`)
	req = httptest.NewRequest("PUT", "/api/servers/srv-upd-test", bytes.NewReader(seedBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for seed update, got %d", w.Code)
	}
	var seedUpdated models.Server
	_ = json.Unmarshal(w.Body.Bytes(), &seedUpdated)
	if seedUpdated.Seed != "custom-bedrock-seed-999" {
		t.Errorf("expected seed 'custom-bedrock-seed-999', got '%s'", seedUpdated.Seed)
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

func TestAPIServerMetrics(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Create admin user & token
	pwHash, _ := auth.HashPassword("TestPass1234!", 8)
	user, err := db.CreateUser(ctx, "admin_metrics", pwHash, models.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	token, _ := auth.GenerateJWT(jwtSecret, user.ID, user.Username, user.Role, time.Hour)

	// 2. Create server
	srv := &models.Server{
		ID:          "srv-metrics-1",
		Name:        "Metrics Test Server",
		Port:        19180,
		PortV6:      19181,
		Status:      models.ServerStatusRunning,
		MemoryLimit: "4G",
		CPULimit:    3.0,
	}
	if err := db.CreateServer(ctx, srv); err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	// 3. Query metrics endpoint
	req := httptest.NewRequest("GET", "/api/servers/srv-metrics-1/metrics?range=15m", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for metrics endpoint, got %d: %s", w.Code, w.Body.String())
	}

	var resp MetricsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode MetricsResponse: %v", err)
	}

	if resp.ServerID != "srv-metrics-1" {
		t.Errorf("expected ServerID 'srv-metrics-1', got '%s'", resp.ServerID)
	}
	if resp.CPULimit != 3.0 {
		t.Errorf("expected CPULimit 3.0, got %f", resp.CPULimit)
	}
	if resp.MaxPlayers <= 0 {
		t.Errorf("expected MaxPlayers > 0, got %d", resp.MaxPlayers)
	}
	if resp.Range != "15m" {
		t.Errorf("expected range '15m', got '%s'", resp.Range)
	}
}

func TestPortGateBansAndCentralizedEndpoints(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()

	ctx := t.Context()
	u, _ := db.CreateUser(ctx, "admin", "hash", models.RoleAdmin)
	token, _ := auth.GenerateJWT(jwtSecret, u.ID, u.Username, u.Role, 1*time.Hour)

	// 1. Create server with port gate enabled
	srv := &models.Server{
		ID:              "srv-pg-test",
		Name:            "Port Gate Test Server",
		Port:            19132,
		PortV6:          19133,
		Status:          models.ServerStatusRunning,
		PortGateEnabled: true,
		PortGateMode:    models.PortGateModePassphrase,
	}
	if err := db.CreateServer(ctx, srv); err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	// 2. Ban IP globally via POST /api/portgate/bans
	banReqBody := []byte(`{
		"ip_or_subnet": "198.51.100.77",
		"is_global": true,
		"reason": "Test Ban Policy Violation"
	}`)
	req := httptest.NewRequest("POST", "/api/portgate/bans", bytes.NewReader(banReqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for ban creation, got %d: %s", w.Code, w.Body.String())
	}

	var createdBan models.PortGateBanRule
	if err := json.Unmarshal(w.Body.Bytes(), &createdBan); err != nil || createdBan.ID == 0 {
		t.Fatalf("failed to decode created ban: %v", err)
	}

	// 3. List bans via GET /api/portgate/bans
	req = httptest.NewRequest("GET", "/api/portgate/bans", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list bans, got %d", w.Code)
	}
	var bans []models.PortGateBanRule
	_ = json.Unmarshal(w.Body.Bytes(), &bans)
	if len(bans) != 1 || bans[0].IPOrSubnet != "198.51.100.77" {
		t.Fatalf("expected 1 ban with 198.51.100.77, got %+v", bans)
	}

	// 4. Knock Portal Config for banned IP: should return is_banned: true
	req = httptest.NewRequest("GET", "/api/knock/srv-pg-test/config", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.77")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for knock config, got %d", w.Code)
	}
	var conf KnockConfigResponse
	_ = json.Unmarshal(w.Body.Bytes(), &conf)
	if !conf.IsBanned || conf.BanReason != "Test Ban Policy Violation" {
		t.Fatalf("expected IsBanned=true, got %+v", conf)
	}

	// 5. Knock attempt from banned IP: should be 403 Forbidden
	knockBody := []byte(`{"passphrase": "any-secret"}`)
	req = httptest.NewRequest("POST", "/api/knock/srv-pg-test/knock", bytes.NewReader(knockBody))
	req.Header.Set("X-Forwarded-For", "198.51.100.77")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for banned knock, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Status check from banned IP: should be 403 Forbidden
	req = httptest.NewRequest("GET", "/api/knock/srv-pg-test/status", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.77")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for status check, got %d", w.Code)
	}

	// 7. Centralized list leases: GET /api/portgate/leases
	req = httptest.NewRequest("GET", "/api/portgate/leases", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/portgate/leases, got %d", w.Code)
	}

	// 8. Delete ban via DELETE /api/portgate/bans/{banId}
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/portgate/bans/%d", createdBan.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for unban, got %d: %s", w.Code, w.Body.String())
	}

	// 9. Verify caller is no longer banned in config
	req = httptest.NewRequest("GET", "/api/knock/srv-pg-test/config", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.77")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var confAfter KnockConfigResponse
	_ = json.Unmarshal(w.Body.Bytes(), &confAfter)
	if confAfter.IsBanned {
		t.Fatalf("expected IsBanned=false after unban")
	}
}

func TestCustomGameServerAddress(t *testing.T) {
	router, db, _, mockFw, jwtSecret, pepper := setupTestRouter(t)
	defer db.Close()

	ctx := t.Context()
	u, _ := db.CreateUser(ctx, "admin", "hash", models.RoleAdmin)
	token, _ := auth.GenerateJWT(jwtSecret, u.ID, u.Username, u.Role, 1*time.Hour)

	// 1. Create server with custom game server address
	srv := &models.Server{
		ID:                "custom-addr-srv",
		Name:              "Custom Realm",
		Version:           "1.21.0.03",
		Port:              19132,
		PortV6:            19133,
		Mode:              "survival",
		Difficulty:        "normal",
		PortGateEnabled:   true,
		PortGateMode:      "passphrase",
		PortGateTimeout:   3600,
		GameServerAddress: "play.customrealm.net",
		Status:            "stopped",
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

	// 2. Verify Knock Config returns custom game server address
	req = httptest.NewRequest("GET", "/api/knock/custom-addr-srv/config", nil)
	req.Host = "panel.webadmin.io:8080"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("knock config failed: %d, body: %s", w.Code, w.Body.String())
	}
	var conf KnockConfigResponse
	_ = json.Unmarshal(w.Body.Bytes(), &conf)
	if conf.GameServerAddress != "play.customrealm.net" {
		t.Fatalf("expected game_server_address 'play.customrealm.net', got '%s'", conf.GameServerAddress)
	}

	// 3. Create access key
	keyHash := auth.HashAccessKey(pepper, "custom-secret")
	srvID := "custom-addr-srv"
	_ = db.CreatePortGateKey(ctx, &models.PortGateKey{
		ServerID:  &srvID,
		Label:     "Secret Door",
		KeyHash:   keyHash,
		KeyPrefix: "custom-",
		IsActive:  true,
	})

	// 4. Perform Knock with custom address
	knockReq := KnockRequest{
		Passphrase: "custom-secret",
	}
	kBody, _ := json.Marshal(knockReq)
	req = httptest.NewRequest("POST", "/api/knock/custom-addr-srv", bytes.NewReader(kBody))
	req.Host = "panel.webadmin.io:8080"
	req.RemoteAddr = "192.0.2.45:33221"
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("knock failed: %d, body: %s", w.Code, w.Body.String())
	}
	var knockRes map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &knockRes)
	if knockRes["game_server_address"] != "play.customrealm.net" {
		t.Fatalf("expected knock game_server_address 'play.customrealm.net', got '%v'", knockRes["game_server_address"])
	}
	directURL, _ := knockRes["direct_launch_url"].(string)
	if !strings.Contains(directURL, "minecraft://connect?serverUrl=play.customrealm.net&serverPort=19132") {
		t.Fatalf("expected direct connect url to contain 'minecraft://connect?serverUrl=play.customrealm.net&serverPort=19132', got '%s'", directURL)
	}
	addServerURL, _ := knockRes["add_server_url"].(string)
	if !strings.Contains(addServerURL, "minecraft://?addExternalServer=") || !strings.Contains(addServerURL, "play.customrealm.net:19132") {
		t.Fatalf("expected add_server_url with addExternalServer, got '%s'", addServerURL)
	}

	// 5. Check Status endpoint returns custom address
	req = httptest.NewRequest("GET", "/api/knock/custom-addr-srv/status", nil)
	req.Host = "panel.webadmin.io:8080"
	req.RemoteAddr = "192.0.2.45:33221"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status failed: %d", w.Code)
	}
	var statusRes map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &statusRes)
	if statusRes["game_server_address"] != "play.customrealm.net" {
		t.Fatalf("expected status game_server_address 'play.customrealm.net', got '%v'", statusRes["game_server_address"])
	}

	// 6. Update GameServerAddress via PUT
	newAddress := "mc.updatedaddress.com"
	updatePayload := map[string]interface{}{
		"game_server_address": newAddress,
	}
	uBody, _ := json.Marshal(updatePayload)
	req = httptest.NewRequest("PUT", "/api/servers/custom-addr-srv", bytes.NewReader(uBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update server failed: %d, body: %s", w.Code, w.Body.String())
	}

	// Verify updated address in config
	req = httptest.NewRequest("GET", "/api/knock/custom-addr-srv/config", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	_ = json.Unmarshal(w.Body.Bytes(), &conf)
	if conf.GameServerAddress != "mc.updatedaddress.com" {
		t.Fatalf("expected updated game_server_address 'mc.updatedaddress.com', got '%s'", conf.GameServerAddress)
	}

	_ = mockFw
}

func TestBroadcastAndTellrawFormatting(t *testing.T) {
	router, db, mockEng, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()

	ctx := t.Context()
	u, _ := db.CreateUser(ctx, "admin", "hash", models.RoleAdmin)
	token, _ := auth.GenerateJWT(jwtSecret, u.ID, u.Username, u.Role, 1*time.Hour)

	// Create and start server
	srv := &models.Server{
		ID:         "srv-broadcast-test",
		Name:       "Broadcast Realm",
		Port:       19132,
		PortV6:     19133,
		Mode:       "survival",
		Difficulty: "normal",
		Status:     "stopped",
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

	// 1. Global Broadcast: must use tellraw without "[" say syntax error
	bPayload := map[string]string{
		"message": "Welcome all players!",
	}
	bBody, _ := json.Marshal(bPayload)
	req = httptest.NewRequest("POST", "/api/servers/srv-broadcast-test/broadcast", bytes.NewReader(bBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("broadcast failed: %d, body: %s", w.Code, w.Body.String())
	}

	logs := mockEng.GetRecentLogs("srv-broadcast-test")
	foundTellrawGlobal := false
	for _, l := range logs {
		if strings.Contains(l, `tellraw @a {"rawtext":[{"text":"§6[BSM Broadcast] §fWelcome all players!"}]}`) {
			foundTellrawGlobal = true
			break
		}
	}
	if !foundTellrawGlobal {
		t.Fatalf("expected tellraw @a command in mock engine logs, got: %v", logs)
	}

	// 2. Targeted Broadcast: must use tellraw with quoted player name
	targetPayload := map[string]string{
		"message": "Private admin notification",
		"target":  "Diamond Miner",
	}
	tBody, _ := json.Marshal(targetPayload)
	req = httptest.NewRequest("POST", "/api/servers/srv-broadcast-test/broadcast", bytes.NewReader(tBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("targeted broadcast failed: %d", w.Code)
	}

	logs = mockEng.GetRecentLogs("srv-broadcast-test")
	foundTellrawTarget := false
	for _, l := range logs {
		if strings.Contains(l, `tellraw "Diamond Miner" {"rawtext":[{"text":"§d[BSM Admin -> You] §fPrivate admin notification"}]}`) {
			foundTellrawTarget = true
			break
		}
	}
	if !foundTellrawTarget {
		t.Fatalf("expected targeted tellraw command in logs, got: %v", logs)
	}

	// 3. Raw console command starting with `say [` should be normalized to tellraw
	cmdPayload := map[string]string{
		"command": "say [BSM Broadcast]: hellp",
	}
	cBody, _ := json.Marshal(cmdPayload)
	req = httptest.NewRequest("POST", "/api/servers/srv-broadcast-test/command", bytes.NewReader(cBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("command failed: %d", w.Code)
	}

	logs = mockEng.GetRecentLogs("srv-broadcast-test")
	foundNormalizedCmd := false
	for _, l := range logs {
		if strings.Contains(l, `tellraw @a {"rawtext":[{"text":"[BSM Broadcast]: hellp"}]}`) {
			foundNormalizedCmd = true
			break
		}
	}
	if !foundNormalizedCmd {
		t.Fatalf("expected normalized say [ command in logs, got: %v", logs)
	}
}

func TestVersionAndHealthEndpoints(t *testing.T) {
	router, _, _, _, _, _ := setupTestRouter(t)

	// Test /api/health
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/health, got %d", w.Code)
	}
	var healthRes map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &healthRes); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if healthRes["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", healthRes["status"])
	}
	if healthRes["version"] != version.Version {
		t.Errorf("expected version %s in /api/health, got %v", version.Version, healthRes["version"])
	}

	// Test /api/version
	req = httptest.NewRequest("GET", "/api/version", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/version, got %d", w.Code)
	}
	var verRes map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &verRes); err != nil {
		t.Fatalf("failed to decode version response: %v", err)
	}
	if verRes["version"] != version.Version {
		t.Errorf("expected version %s in /api/version, got %v", version.Version, verRes["version"])
	}
	if verRes["app_name"] != "Bedrock Server Manager (BSM)" {
		t.Errorf("expected app_name Bedrock Server Manager (BSM), got %v", verRes["app_name"])
	}
}

func TestPopularSeedsEndpoint(t *testing.T) {
	router, _, _, _, jwtSecret, _ := setupTestRouter(t)

	// Auth token
	token, _ := auth.GenerateJWT(jwtSecret, 1, "admin", "admin", time.Hour)

	req := httptest.NewRequest("GET", "/api/presets/seeds", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/presets/seeds, got %d", w.Code)
	}

	var seeds []preset.SeedPreset
	if err := json.Unmarshal(w.Body.Bytes(), &seeds); err != nil {
		t.Fatalf("failed to decode seeds json: %v", err)
	}

	if len(seeds) < 50 {
		t.Fatalf("expected at least 50 seeds, got %d", len(seeds))
	}

	foundCherry := false
	for _, s := range seeds {
		if s.ID == "cherry-caldera" {
			foundCherry = true
			if s.Seed != "-8219986470354173872" {
				t.Errorf("expected cherry caldera seed '-8219986470354173872', got '%s'", s.Seed)
			}
			if len(s.Biomes) == 0 || len(s.Features) == 0 {
				t.Errorf("expected biomes and features to be populated for cherry caldera")
			}
			break
		}
	}

	if !foundCherry {
		t.Errorf("expected cherry-caldera seed in popular seeds list")
	}
}

func TestOnlinePlayersAndOpDeop(t *testing.T) {
	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}
	defer db.Close()

	jwtSecret := []byte("test-jwt-secret-key-32bytes-long!")
	pepper := "test-pepper-1234"
	rateLimiter := auth.NewRateLimiter(5, 5*time.Minute, 10, 5*time.Minute, 15*time.Minute)
	resolver := ipresolver.NewResolver("direct", nil)
	mockEngine := engine.NewMockEngine()
	mockFw := firewall.NewMockFirewallDriver()
	pa := allocator.NewPortAllocator()
	pm := player.NewManager(nil, nil)

	r := NewRouter(RouterOptions{
		DB:            db,
		IPResolver:    resolver,
		RateLimiter:   rateLimiter,
		Engine:        mockEngine,
		Firewall:      mockFw,
		PortAllocator: pa,
		PlayerManager: pm,
		DataDir:       t.TempDir(),
		JWTSecret:     jwtSecret,
		Pepper:        pepper,
	})

	token, _ := auth.GenerateJWT(jwtSecret, 1, "admin", "admin", time.Hour)

	// Create running server
	srv := &models.Server{
		ID:        "srv-players-1",
		Name:      "Player Test Server",
		Port:      19132,
		Status:    models.ServerStatusRunning,
		CreatedAt: time.Now().UTC(),
	}
	if err := db.CreateServer(context.Background(), srv); err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	// 1. Initial /stats should have 0 players
	req := httptest.NewRequest("GET", "/api/servers/srv-players-1/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from stats, got %d", w.Code)
	}
	var stats map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &stats)
	if count, ok := stats["player_count"].(float64); !ok || int(count) != 0 {
		t.Fatalf("expected 0 player_count initially, got %v", stats["player_count"])
	}

	// 2. Connect player Steve
	pm.ProcessLine("srv-players-1", "Player connected: Steve, xuid: 123456789")

	// 3. /stats should now return player_count = 1
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/servers/srv-players-1/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	stats = nil
	_ = json.Unmarshal(w.Body.Bytes(), &stats)
	if count, ok := stats["player_count"].(float64); !ok || int(count) != 1 {
		t.Fatalf("expected 1 player_count after connect, got %v", stats["player_count"])
	}

	// 4. GET /players - verify Steve is online and not OP
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/servers/srv-players-1/players", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from get players, got %d", w.Code)
	}
	var plRes map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &plRes)
	playersList, ok := plRes["online_players"].([]interface{})
	if !ok || len(playersList) != 1 {
		t.Fatalf("expected 1 online player, got %+v", plRes)
	}
	playerObj := playersList[0].(map[string]interface{})
	if playerObj["gamertag"] != "Steve" || playerObj["is_op"] == true {
		t.Fatalf("expected Steve is_op=false, got %+v", playerObj)
	}

	// 5. POST /players/op - OP player Steve
	opBody := `{"gamertag":"Steve"}`
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/servers/srv-players-1/players/op", strings.NewReader(opBody))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from op, got %d: %s", w.Code, w.Body.String())
	}

	// 6. GET /players - verify Steve is now is_op = true and permission = operator
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/servers/srv-players-1/players", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	plRes = nil
	_ = json.Unmarshal(w.Body.Bytes(), &plRes)
	playersList = plRes["online_players"].([]interface{})
	playerObj = playersList[0].(map[string]interface{})
	if playerObj["is_op"] != true || playerObj["permission"] != "operator" {
		t.Fatalf("expected Steve is_op=true permission=operator, got %+v", playerObj)
	}

	// 7. POST /players/deop - DeOP player Steve
	deopBody := `{"gamertag":"Steve"}`
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/servers/srv-players-1/players/deop", strings.NewReader(deopBody))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from deop, got %d: %s", w.Code, w.Body.String())
	}

	// 8. GET /players - verify Steve is now is_op = false
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/servers/srv-players-1/players", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	plRes = nil
	_ = json.Unmarshal(w.Body.Bytes(), &plRes)
	playersList = plRes["online_players"].([]interface{})
	playerObj = playersList[0].(map[string]interface{})
	if playerObj["is_op"] == true || playerObj["permission"] == "operator" {
		t.Fatalf("expected Steve is_op=false after deop, got %+v", playerObj)
	}

	// Verify all_players contains Steve and is_online is true
	allPlayers, ok := plRes["all_players"].([]interface{})
	if !ok || len(allPlayers) != 1 {
		t.Fatalf("expected 1 in all_players, got %+v", plRes["all_players"])
	}
	allObj := allPlayers[0].(map[string]interface{})
	if allObj["gamertag"] != "Steve" || allObj["is_online"] != true {
		t.Fatalf("expected Steve in all_players with is_online=true, got %+v", allObj)
	}

	// 9. Simulate disconnect: Steve disconnects
	pm.ProcessLine("srv-players-1", "Player disconnected: Steve, xuid: 2535412345678901")
	_ = db.UpdatePlayerLastSeen(context.Background(), "srv-players-1", "Steve")

	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/servers/srv-players-1/players", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	plRes = nil
	_ = json.Unmarshal(w.Body.Bytes(), &plRes)
	if count, ok := plRes["online_count"].(float64); !ok || int(count) != 0 {
		t.Fatalf("expected 0 online_count, got %v", plRes["online_count"])
	}
	// Steve must STILL appear in all_players history list as is_online=false!
	allPlayers = plRes["all_players"].([]interface{})
	if len(allPlayers) != 1 {
		t.Fatalf("expected Steve to still be retained in all_players after disconnect, got %d", len(allPlayers))
	}
	allObj = allPlayers[0].(map[string]interface{})
	if allObj["gamertag"] != "Steve" || allObj["is_online"] != false {
		t.Fatalf("expected Steve in all_players history with is_online=false, got %+v", allObj)
	}
}

func TestPlayerBanWorkflow(t *testing.T) {
	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}
	defer db.Close()

	jwtSecret := []byte("test-jwt-secret-key-32bytes-long!")
	pepper := "test-pepper-1234"
	rateLimiter := auth.NewRateLimiter(5, 5*time.Minute, 10, 5*time.Minute, 15*time.Minute)
	resolver := ipresolver.NewResolver("direct", nil)
	mockEngine := engine.NewMockEngine()
	mockFw := firewall.NewMockFirewallDriver()
	pa := allocator.NewPortAllocator()
	pm := player.NewManager(nil, nil)

	r := NewRouter(RouterOptions{
		DB:            db,
		IPResolver:    resolver,
		RateLimiter:   rateLimiter,
		Engine:        mockEngine,
		Firewall:      mockFw,
		PortAllocator: pa,
		PlayerManager: pm,
		DataDir:       t.TempDir(),
		JWTSecret:     jwtSecret,
		Pepper:        pepper,
	})

	token, _ := auth.GenerateJWT(jwtSecret, 1, "admin", "admin", time.Hour)

	// Create running server
	srv := &models.Server{
		ID:        "srv-ban-1",
		Name:      "Ban Test Server",
		Port:      19132,
		Status:    models.ServerStatusRunning,
		CreatedAt: time.Now().UTC(),
	}
	if err := db.CreateServer(context.Background(), srv); err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	// 1. Ban player Alex on instance
	banBody := `{"gamertag":"Alex","xuid":"987654321","reason":"Griefing spawn","scope":"instance","ban_ip":true,"ip_address":"198.51.100.22"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/servers/srv-ban-1/players/ban", strings.NewReader(banBody))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from ban, got %d: %s", w.Code, w.Body.String())
	}

	// Verify ban is recorded
	isBanned, reason, err := db.IsPlayerBanned(context.Background(), "srv-ban-1", "Alex", "987654321")
	if err != nil || !isBanned || reason != "Griefing spawn" {
		t.Fatalf("expected Alex to be banned on srv-ban-1, got isBanned=%v, reason=%s", isBanned, reason)
	}

	// Verify IP ban was created in port gate bans
	isIPBanned, _, _ := db.IsIPBanned(context.Background(), "srv-ban-1", "198.51.100.22")
	if !isIPBanned {
		t.Fatalf("expected 198.51.100.22 to be banned on port gate")
	}

	// Verify list bans endpoint
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/servers/srv-ban-1/players/bans", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from list bans, got %d", w.Code)
	}
	var bansList []models.BannedPlayer
	_ = json.Unmarshal(w.Body.Bytes(), &bansList)
	if len(bansList) != 1 || bansList[0].Gamertag != "Alex" {
		t.Fatalf("expected 1 ban for Alex, got %+v", bansList)
	}

	// 2. Unban Alex
	unbanBody := `{"gamertag":"Alex"}`
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/servers/srv-ban-1/players/unban", strings.NewReader(unbanBody))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from unban, got %d", w.Code)
	}

	isBannedAfter, _, _ := db.IsPlayerBanned(context.Background(), "srv-ban-1", "Alex", "")
	if isBannedAfter {
		t.Fatalf("expected Alex to not be banned after unban")
	}
}

func TestServerPropertiesUnzipAndMergeFlow(t *testing.T) {
	router, db, mockEngine, _, jwtSecret, _ := setupTestRouter(t)
	token, _ := auth.GenerateJWT(jwtSecret, 1, "admin", "admin", time.Hour)
	dataDir := "data_test"
	defer os.RemoveAll(dataDir)

	// 1. Create server instance with a seed (simulating wizard deploy)
	createPayload := `{
		"id": "wizard-srv",
		"name": "Wizard Deployed Realm",
		"port": 19180,
		"portv6": 19181,
		"mode": "survival",
		"difficulty": "normal",
		"seed": "777888999"
	}`
	req := httptest.NewRequest("POST", "/api/servers", strings.NewReader(createPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created from POST /api/servers, got %d: %s", w.Code, w.Body.String())
	}

	// Verify server.properties was NOT created prematurely on disk, leaving directory clean for itzg
	propPath := filepath.Join(dataDir, "servers", "wizard-srv", "server.properties")
	if _, err := os.Stat(propPath); !os.IsNotExist(err) {
		t.Fatalf("expected server.properties to NOT exist prior to itzg boot, but it was found")
	}

	// 2. GET /properties before first boot returns uninitialized=true and default properties for pre-boot configuration
	req = httptest.NewRequest("GET", "/api/servers/wizard-srv/properties", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /properties, got %d: %s", w.Code, w.Body.String())
	}

	var res struct {
		Properties    map[string]string `json:"properties"`
		Keys          []string          `json:"keys"`
		Uninitialized bool              `json:"uninitialized"`
		Pending       bool              `json:"pending"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal properties response: %v", err)
	}
	if !res.Uninitialized {
		t.Errorf("expected uninitialized=true before first boot")
	}
	if len(res.Keys) != len(configfile.DefaultServerPropertyKeys) {
		t.Errorf("expected %d default keys before first boot, got %d", len(configfile.DefaultServerPropertyKeys), len(res.Keys))
	}
	if res.Properties["server-name"] != "Wizard Deployed Realm" {
		t.Errorf("expected server-name 'Wizard Deployed Realm', got '%s'", res.Properties["server-name"])
	}

	// 3. User customizes properties before first boot (e.g. view-distance=48, max-players=25)
	res.Properties["view-distance"] = "48"
	res.Properties["max-players"] = "25"
	updateBody, _ := json.Marshal(map[string]interface{}{
		"properties": res.Properties,
		"keys":       res.Keys,
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest("PUT", "/api/servers/wizard-srv/properties", bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from PUT /properties, got %d: %s", w.Code, w.Body.String())
	}

	// server.properties must STILL NOT exist on disk, avoiding blocking itzg
	if _, err := os.Stat(propPath); !os.IsNotExist(err) {
		t.Fatalf("expected server.properties to not exist yet after pending save")
	}

	// .pending_properties.json must exist on disk
	serverDir := filepath.Join(dataDir, "servers", "wizard-srv")
	pendingProps, _, hasPending, err := configfile.ReadPendingProperties(serverDir)
	if err != nil || !hasPending {
		t.Fatalf("expected pending properties file to exist, err: %v", err)
	}
	if pendingProps["view-distance"] != "48" {
		t.Errorf("expected pending view-distance '48', got '%s'", pendingProps["view-distance"])
	}

	// 4. Simulate itzg booting and unzipping official Mojang BDS properties on disk (with default view-distance=32)
	bdsTemplate := "server-name=Dedicated Server\ngamemode=survival\ndifficulty=easy\nview-distance=32\nmax-players=10\nallow-cheats=false\nlevel-seed=777888999\n"
	if err := os.WriteFile(propPath, []byte(bdsTemplate), 0644); err != nil {
		t.Fatalf("failed to simulate itzg unzipping server.properties: %v", err)
	}

	// 5. Trigger applyPendingPropertiesOnStart (which unblocks on server.properties existing)
	serverHandler := &ServerHandler{
		db:      db,
		engine:  mockEngine,
		dataDir: dataDir,
	}
	serverHandler.applyPendingPropertiesOnStart("wizard-srv")

	// Verify server.properties on disk has now merged the pre-boot customized properties!
	diskProps, _, err := configfile.ReadProperties(propPath)
	if err != nil {
		t.Fatalf("failed to read updated server.properties: %v", err)
	}
	if diskProps["view-distance"] != "48" {
		t.Errorf("expected view-distance updated to '48', got '%s'", diskProps["view-distance"])
	}
	if diskProps["max-players"] != "25" {
		t.Errorf("expected max-players updated to '25', got '%s'", diskProps["max-players"])
	}
	if diskProps["gamemode"] != "survival" {
		t.Errorf("expected gamemode 'survival' preserved, got '%s'", diskProps["gamemode"])
	}

	// Verify pending file was removed
	if _, _, stillPending, _ := configfile.ReadPendingProperties(serverDir); stillPending {
		t.Errorf("expected pending properties file to be deleted after apply")
	}

	// 6. Next GET /properties returns uninitialized=false, pending=false
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/servers/wizard-srv/properties", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /properties after boot, got %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Uninitialized {
		t.Errorf("expected uninitialized=false after first boot")
	}
	if res.Pending {
		t.Errorf("expected pending=false after first boot")
	}
	if res.Properties["view-distance"] != "48" {
		t.Errorf("expected view-distance '48', got '%s'", res.Properties["view-distance"])
	}
}

func TestDashboardSummary(t *testing.T) {
	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	dataDir := t.TempDir()
	eng := engine.NewMockEngine()
	pa := allocator.NewPortAllocator()
	pm := player.NewManager(nil, nil)
	resolver := ipresolver.NewResolver("direct", nil)
	jwtSecret := []byte("test-dashboard-secret-32bytes-long!")

	router := NewRouter(RouterOptions{
		DB:            db,
		IPResolver:    resolver,
		Engine:        eng,
		PortAllocator: pa,
		PlayerManager: pm,
		DataDir:       dataDir,
		JWTSecret:     jwtSecret,
		Pepper:        "test-pepper",
	})

	// Create admin user & login
	_, _ = db.CreateUser(context.Background(), "dashadmin", "$2a$10$abcdefghijklmnopqrstuuNOPQRSTUVWXYZ1234567890abcdefghij", "admin")
	token, _ := auth.GenerateJWT(jwtSecret, 1, "dashadmin", "admin", time.Hour)

	// Create running server
	s1 := &models.Server{
		ID:          "srv-dash-1",
		Name:        "Dash Running Server",
		Version:     "1.20.0",
		Port:        19132,
		PortV6:      19133,
		Status:      models.ServerStatusRunning,
		MemoryLimit: "2G",
		CPULimit:    2.0,
	}
	_ = db.CreateServer(context.Background(), s1)
	_ = eng.StartServer(context.Background(), s1)

	// Create stopped server
	s2 := &models.Server{
		ID:          "srv-dash-2",
		Name:        "Dash Stopped Server",
		Version:     "1.20.0",
		Port:        19134,
		PortV6:      19135,
		Status:      models.ServerStatusStopped,
		MemoryLimit: "4G",
		CPULimit:    4.0,
	}
	_ = db.CreateServer(context.Background(), s2)

	// Online player on s1
	pm.ProcessLine("srv-dash-1", "Player connected: DashPlayer, xuid: 123456789")

	// Create a backup
	_ = db.CreateBackup(context.Background(), &models.Backup{
		ServerID:  "srv-dash-1",
		Filename:  "backup-dash-1.tar.gz",
		SizeBytes: 1048576,
		Type:      "manual",
		Status:    "completed",
		CreatedAt: time.Now().UTC(),
	})

	// Create an active port gate lease
	_ = db.CreatePortGateLease(context.Background(), &models.PortGateLease{
		ServerID:    "srv-dash-1",
		IPAddress:   "192.168.1.100",
		Gamertag:    "DashPlayer",
		GrantedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(2 * time.Hour),
		KnockMethod: "passphrase",
		Status:      "active",
	})

	// Fetch GET /api/dashboard/summary
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/dashboard/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/dashboard/summary, got %d: %s", w.Code, w.Body.String())
	}

	var summary DashboardSummaryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to unmarshal dashboard summary: %v", err)
	}

	if summary.TotalServers != 2 {
		t.Errorf("expected 2 total servers, got %d", summary.TotalServers)
	}
	if summary.RunningServers != 1 {
		t.Errorf("expected 1 running server, got %d", summary.RunningServers)
	}
	if summary.StoppedServers != 1 {
		t.Errorf("expected 1 stopped server, got %d", summary.StoppedServers)
	}
	if summary.TotalOnlinePlayers != 1 {
		t.Errorf("expected 1 online player, got %d", summary.TotalOnlinePlayers)
	}
	if len(summary.ActivePlayers) != 1 || summary.ActivePlayers[0].Gamertag != "DashPlayer" {
		t.Errorf("expected DashPlayer in active players, got %+v", summary.ActivePlayers)
	}
	if summary.TotalBackupsCount != 1 {
		t.Errorf("expected 1 backup, got %d", summary.TotalBackupsCount)
	}
	if summary.ActiveLeasesCount != 1 {
		t.Errorf("expected 1 active lease, got %d", summary.ActiveLeasesCount)
	}
	if summary.TotalAllocatedCores != 6.0 {
		t.Errorf("expected 6.0 allocated cores, got %f", summary.TotalAllocatedCores)
	}
	if summary.TotalUsedRAM != 1024*1024*450 {
		t.Errorf("expected 450MB total used ram, got %d", summary.TotalUsedRAM)
	}
	if summary.TotalUsedCPUPercent != 14.5 {
		t.Errorf("expected 14.5 total used cpu percent, got %f", summary.TotalUsedCPUPercent)
	}
	if summary.TotalUsedCPUCores != 0.145 {
		t.Errorf("expected 0.145 used cpu cores, got %f", summary.TotalUsedCPUCores)
	}
	if summary.HostSystem.Version == "" || summary.HostSystem.OS == "" {
		t.Errorf("expected valid host system telemetry, got %+v", summary.HostSystem)
	}
}

func TestPlansAPIAndNormalUserQuotas(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	ctx := context.Background()

	adminUser, err := db.CreateUser(ctx, "sysadmin", "hash", models.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	adminToken, _ := auth.GenerateJWT(jwtSecret, adminUser.ID, adminUser.Username, adminUser.Role, time.Hour)

	// 1. List plans as admin (verify Default Plan exists)
	req := httptest.NewRequest("GET", "/api/plans", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from GET /api/plans, got %d: %s", w.Code, w.Body.String())
	}
	var plans []models.Plan
	_ = json.Unmarshal(w.Body.Bytes(), &plans)
	if len(plans) == 0 {
		t.Fatalf("expected at least 1 plan")
	}

	// 2. Create custom plan
	newPlanPayload := `{
		"name": "Standard Tier",
		"description": "2 servers, 2GB each",
		"max_servers": 2,
		"max_memory": "2G",
		"max_cpu": 2.0,
		"max_collaborators": 1,
		"allow_custom_port": false,
		"allow_custom_seed": true
	}`
	req = httptest.NewRequest("POST", "/api/plans", strings.NewReader(newPlanPayload))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 from POST /api/plans, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Test self-registration: blocked when disabled
	regPayload := `{"username": "gamer1", "password": "password123", "email": "gamer1@example.com"}`
	req = httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(regPayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when registration is disabled, got %d: %s", w.Code, w.Body.String())
	}

	// Enable registration via settings
	_ = db.SetSetting(ctx, "allow_registration", "true")

	// Test self-registration: success when enabled
	req = httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(regPayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 from registration, got %d: %s", w.Code, w.Body.String())
	}
	var regResp struct {
		Token string      `json:"token"`
		User  models.User `json:"user"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &regResp)
	userToken := regResp.Token
	userID := regResp.User.ID

	// 4. Check user plan status via GET /api/user/plan
	req = httptest.NewRequest("GET", "/api/user/plan", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from GET /api/user/plan, got %d: %s", w.Code, w.Body.String())
	}

	// 5. Normal user creates first server within quota (Default Plan max_servers: 1)
	srv1Payload := `{
		"id": "gamer-srv-1",
		"name": "Gamer Realm 1",
		"port": 19132,
		"portv6": 19133,
		"memory_limit": "2G",
		"cpu_limit": 1.5
	}`
	req = httptest.NewRequest("POST", "/api/servers", strings.NewReader(srv1Payload))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 from first server creation, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Normal user attempts to create second server -> Quota Exceeded!
	srv2Payload := `{
		"id": "gamer-srv-2",
		"name": "Gamer Realm 2",
		"port": 19134,
		"portv6": 19135,
		"memory_limit": "2G"
	}`
	req = httptest.NewRequest("POST", "/api/servers", strings.NewReader(srv2Payload))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for quota exceeded, got %d: %s", w.Code, w.Body.String())
	}

	// 7. Safe command testing: safe command allowed
	safeCmdPayload := `{"command": "say Hello Minecraft"}`
	req = httptest.NewRequest("POST", "/api/servers/gamer-srv-1/command", strings.NewReader(safeCmdPayload))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from safe command, got %d: %s", w.Code, w.Body.String())
	}

	// Dangerous command blocked for normal user
	dangerCmdPayload := `{"command": "op hacker"}`
	req = httptest.NewRequest("POST", "/api/servers/gamer-srv-1/command", strings.NewReader(dangerCmdPayload))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for dangerous command, got %d: %s", w.Code, w.Body.String())
	}

	// 8. User deletes their own server -> Quota reclaimed
	req = httptest.NewRequest("DELETE", "/api/servers/gamer-srv-1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from DELETE /api/servers/gamer-srv-1, got %d: %s", w.Code, w.Body.String())
	}

	countAfter, _ := db.CountServersByOwner(ctx, userID)
	if countAfter != 0 {
		t.Fatalf("expected 0 servers after deletion, got %d", countAfter)
	}

	// Now gamer can deploy again
	req = httptest.NewRequest("POST", "/api/servers", strings.NewReader(srv1Payload))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 after reclaiming quota, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPIServerDeploymentNetworkMode(t *testing.T) {
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer db.Close()

	ctx := t.Context()
	adminUser, _ := db.CreateUser(ctx, "admin_net", "hash", models.RoleAdmin)
	adminToken, _ := auth.GenerateJWT(jwtSecret, adminUser.ID, adminUser.Username, adminUser.Role, 1*time.Hour)

	normalUser, _ := db.CreateUser(ctx, "normal_net", "hash", models.RoleUser)
	normalToken, _ := auth.GenerateJWT(jwtSecret, normalUser.ID, normalUser.Username, normalUser.Role, 1*time.Hour)

	// 1. List Networks
	req := httptest.NewRequest("GET", "/api/servers/networks", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/servers/networks, got %d", w.Code)
	}
	var netResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &netResp)
	nets, ok := netResp["networks"].([]interface{})
	if !ok || len(nets) < 2 {
		t.Fatalf("expected at least bridge and host in networks, got: %v", netResp)
	}

	// 2. Admin creates server with network_mode = "host"
	adminPayload := `{
		"id": "admin-host-srv",
		"name": "Admin Host Server",
		"port": 19132,
		"portv6": 19133,
		"network_mode": "host"
	}`
	req = httptest.NewRequest("POST", "/api/servers", strings.NewReader(adminPayload))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create host server, got %d: %s", w.Code, w.Body.String())
	}

	srv, err := db.GetServer(ctx, "admin-host-srv")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if srv.NetworkMode != "host" {
		t.Errorf("expected server network_mode to be 'host', got '%s'", srv.NetworkMode)
	}

	// 3. Normal user server deployment defaults to "host"
	normalPayload := `{
		"id": "normal-net-srv",
		"name": "Normal User Server",
		"port": 19134,
		"portv6": 19135
	}`
	req = httptest.NewRequest("POST", "/api/servers", strings.NewReader(normalPayload))
	req.Header.Set("Authorization", "Bearer "+normalToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for normal user server, got %d: %s", w.Code, w.Body.String())
	}

	userSrv, err := db.GetServer(ctx, "normal-net-srv")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if userSrv.NetworkMode != "host" {
		t.Errorf("expected normal user server network_mode to be 'host', got '%s'", userSrv.NetworkMode)
	}

	// 4. Admin updates network_mode from "host" to "bridge"
	updatePayload := `{"network_mode": "bridge"}`
	req = httptest.NewRequest("PUT", "/api/servers/admin-host-srv", strings.NewReader(updatePayload))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from PUT /api/servers/admin-host-srv, got %d: %s", w.Code, w.Body.String())
	}

	updatedSrv, err := db.GetServer(ctx, "admin-host-srv")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if updatedSrv.NetworkMode != "bridge" {
		t.Errorf("expected updated server network_mode to be 'bridge', got '%s'", updatedSrv.NetworkMode)
	}
}

func TestAddonURLAndMarketplaceAPIs(t *testing.T) {
	ctx := context.Background()
	router, db, _, _, jwtSecret, _ := setupTestRouter(t)
	defer os.RemoveAll("data_test")

	// 1. Setup admin and normal user
	adminToken, _ := auth.GenerateJWT(jwtSecret, 1, "admin", models.RoleAdmin, 1*time.Hour)
	userToken, _ := auth.GenerateJWT(jwtSecret, 2, "normaluser", models.RoleUser, 1*time.Hour)

	// Create test server assigned to admin
	_ = db.CreateServer(ctx, &models.Server{
		ID:        "addon-srv-1",
		Name:      "Addon Server",
		Port:      19132,
		Status:    "offline",
		Version:   "1.21.0.0",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	_ = db.GrantServerAccess(ctx, 2, "addon-srv-1")
	_ = os.MkdirAll(filepath.Join("data_test", "servers", "addon-srv-1"), 0755)

	// 2. Normal user attempts to update marketplace config -> 403
	req := httptest.NewRequest("POST", "/api/addons/marketplace/config", strings.NewReader(`{"curseforge_api_key":"cf-secret-123"}`))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for normal user updating marketplace config, got %d", w.Code)
	}

	// 3. Admin updates marketplace config -> 200
	req = httptest.NewRequest("POST", "/api/addons/marketplace/config", strings.NewReader(`{"curseforge_api_key":"cf-secret-123"}`))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin updating marketplace config, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Check marketplace config endpoint -> curseforge_configured: true
	req = httptest.NewRequest("GET", "/api/addons/marketplace/config", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from GET /addons/marketplace/config, got %d", w.Code)
	}
	var cfgRes map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &cfgRes)
	if cfgRes["curseforge_configured"] != true {
		t.Fatalf("expected curseforge_configured = true, got %+v", cfgRes)
	}

	// 5. Test install from URL (Normal user attempt -> 403)
	req = httptest.NewRequest("POST", "/api/servers/addon-srv-1/addons/url", strings.NewReader(`{"url":"https://example.com/pack.mcpack"}`))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin install from URL, got %d", w.Code)
	}

	// 6. Test install from URL (Admin attempt with mock pack server)
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	fw, _ := zw.Create("manifest.json")
	_, _ = fw.Write([]byte(`{
		"format_version": 2,
		"header": {
			"name": "Admin URL Pack",
			"description": "Installed via API URL endpoint",
			"uuid": "abcdef01-2345-6789-abcd-ef0123456789",
			"version": [1, 0, 0]
		},
		"modules": [{"type": "data", "uuid": "feeeeeee-2345-6789-abcd-ef0123456789"}]
	}`))
	_ = zw.Close()

	mockFileServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/zip")
		_, _ = rw.Write(buf.Bytes())
	}))
	defer mockFileServer.Close()

	req = httptest.NewRequest("POST", "/api/servers/addon-srv-1/addons/url", strings.NewReader(fmt.Sprintf(`{"url":"%s/pack.mcpack"}`, mockFileServer.URL)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for admin install from URL, got %d: %s", w.Code, w.Body.String())
	}

	// 7. Verify pack is listed
	req = httptest.NewRequest("GET", "/api/servers/addon-srv-1/addons", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from GET /addons, got %d", w.Code)
	}
	var packs []addon.InstalledPack
	_ = json.Unmarshal(w.Body.Bytes(), &packs)
	if len(packs) != 1 || packs[0].Name != "Admin URL Pack" {
		t.Fatalf("unexpected packs after URL install: %+v", packs)
	}
}



