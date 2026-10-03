package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func TestManagerDB(t *testing.T) {
	ctx := context.Background()
	db, err := OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// 1. Users test
	count, err := db.CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 users, got %d", count)
	}

	u, err := db.CreateUser(ctx, "admin", "hashed_pw", models.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if u.Username != "admin" || u.Role != models.RoleAdmin {
		t.Errorf("unexpected user: %+v", u)
	}

	found, err := db.GetUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	if found.ID != u.ID {
		t.Errorf("expected user ID %d, got %d", u.ID, found.ID)
	}

	// 2. Server test
	srv := &models.Server{
		ID:              "test-srv-1",
		Name:            "Survival World",
		Version:         "1.21.0.03",
		Port:            19132,
		PortV6:          19133,
		Status:          models.ServerStatusStopped,
		Mode:            "survival",
		Difficulty:      "hard",
		AutostartOnBoot: true,
		PortGateEnabled: true,
		PortGateMode:    models.PortGateModeCombined,
		PortGateTimeout: 3600,
		MemoryLimit:     "4G",
		CPULimit:        2.5,
		Seed:            "1234567890",
	}
	if err := db.CreateServer(ctx, srv); err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	gotSrv, err := db.GetServer(ctx, "test-srv-1")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if gotSrv.Name != "Survival World" || !gotSrv.AutostartOnBoot || !gotSrv.PortGateEnabled || gotSrv.Seed != "1234567890" {
		t.Errorf("unexpected server data: %+v", gotSrv)
	}

	// 3. User Server Access
	if err := db.GrantServerAccess(ctx, u.ID, srv.ID); err != nil {
		t.Fatalf("GrantServerAccess failed: %v", err)
	}
	hasAccess, err := db.CheckUserServerAccess(ctx, u.ID, srv.ID)
	if err != nil || !hasAccess {
		t.Fatalf("expected user to have access, got %v (err: %v)", hasAccess, err)
	}

	// 4. Port Gate Key
	exp := time.Now().Add(24 * time.Hour)
	key := &models.PortGateKey{
		ServerID:             &srv.ID,
		Label:                "VIP Access",
		KeyHash:              "hmac_hash_sample",
		KeyPrefix:            "vip-",
		MaxUses:              10,
		LeaseDurationSeconds: 7200,
		ExpiresAt:            &exp,
		IsActive:             true,
	}
	if err := db.CreatePortGateKey(ctx, key); err != nil {
		t.Fatalf("CreatePortGateKey failed: %v", err)
	}
	if key.ID == 0 {
		t.Errorf("expected valid key ID")
	}

	keys, err := db.ListPortGateKeys(ctx, &srv.ID)
	if err != nil {
		t.Fatalf("ListPortGateKeys failed: %v", err)
	}
	if len(keys) != 1 || keys[0].Label != "VIP Access" {
		t.Errorf("unexpected keys list: %+v", keys)
	}

	// 5. Port Gate Lease
	lease := &models.PortGateLease{
		ServerID:         srv.ID,
		KeyID:            &key.ID,
		IPAddress:        "198.51.100.25",
		Gamertag:         "Steve",
		KnockMethod:      "combined",
		SessionTokenHash: "token_hash_abc",
		GrantedAt:        time.Now().UTC(),
		ExpiresAt:        time.Now().UTC().Add(2 * time.Hour),
		Comment:          "VIP Grant",
		Status:           "active",
	}
	if err := db.CreatePortGateLease(ctx, lease); err != nil {
		t.Fatalf("CreatePortGateLease failed: %v", err)
	}

	activeLease, err := db.GetActiveLeaseByIP(ctx, srv.ID, "198.51.100.25")
	if err != nil {
		t.Fatalf("GetActiveLeaseByIP failed: %v", err)
	}
	if activeLease.Gamertag != "Steve" {
		t.Errorf("expected Gamertag 'Steve', got '%s'", activeLease.Gamertag)
	}

	// 6. Settings test
	if err := db.SetSetting(ctx, "jwt_secret", "supersecret"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}
	val, err := db.GetSetting(ctx, "jwt_secret")
	if err != nil || val != "supersecret" {
		t.Errorf("expected 'supersecret', got '%s' (err: %v)", val, err)
	}

	// 7. Global Player Access test
	gp := &models.GlobalPlayer{
		Name:          "GlobalAlex",
		XUID:          "2535400000000001",
		IsAllowlisted: true,
		Permission:    "operator",
	}
	createdGP, err := db.UpsertGlobalPlayer(ctx, gp)
	if err != nil {
		t.Fatalf("UpsertGlobalPlayer failed: %v", err)
	}
	if createdGP.ID == 0 || createdGP.Name != "GlobalAlex" {
		t.Errorf("unexpected created global player: %+v", createdGP)
	}

	byName, err := db.GetGlobalPlayerByName(ctx, "globalalex")
	if err != nil || byName.Permission != "operator" {
		t.Fatalf("GetGlobalPlayerByName failed: %v, player: %+v", err, byName)
	}

	// Upsert update role
	byName.Permission = "member"
	updatedGP, err := db.UpsertGlobalPlayer(ctx, byName)
	if err != nil || updatedGP.Permission != "member" {
		t.Fatalf("Upsert update failed: %v, player: %+v", err, updatedGP)
	}

	listGPs, err := db.ListGlobalPlayers(ctx)
	if err != nil || len(listGPs) != 1 {
		t.Fatalf("ListGlobalPlayers failed: %v, count: %d", err, len(listGPs))
	}

	if err := db.DeleteGlobalPlayerByName(ctx, "globalalex"); err != nil {
		t.Fatalf("DeleteGlobalPlayerByName failed: %v", err)
	}
	countGPs, _ := db.ListGlobalPlayers(ctx)
	if len(countGPs) != 0 {
		t.Errorf("expected 0 global players after delete, got %d", len(countGPs))
	}
}

func TestMetricsDB(t *testing.T) {
	ctx := context.Background()
	db, err := OpenMetricsDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory metrics db: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	samples := []models.MetricRaw{
		{ServerID: "srv-1", Timestamp: now.Add(-10 * time.Minute), CPUPercent: 12.5, RAMBytes: 500000000, PlayerCount: 3},
		{ServerID: "srv-1", Timestamp: now.Add(-9 * time.Minute), CPUPercent: 15.0, RAMBytes: 520000000, PlayerCount: 4},
		{ServerID: "srv-1", Timestamp: now.Add(-8 * time.Minute), CPUPercent: 20.0, RAMBytes: 530000000, PlayerCount: 5},
	}

	if err := db.InsertRawBatch(ctx, samples); err != nil {
		t.Fatalf("InsertRawBatch failed: %v", err)
	}

	raw, err := db.QueryRaw(ctx, "srv-1", now.Add(-15*time.Minute))
	if err != nil {
		t.Fatalf("QueryRaw failed: %v", err)
	}
	if len(raw) != 3 {
		t.Errorf("expected 3 raw metrics, got %d", len(raw))
	}

	// Test 5m rollup
	if err := db.Rollup5Min(ctx, now); err != nil {
		t.Fatalf("Rollup5Min failed: %v", err)
	}

	// Test 1h rollup
	if err := db.Rollup1Hour(ctx, now); err != nil {
		t.Fatalf("Rollup1Hour failed: %v", err)
	}

	// Test pruning (should not prune recent records)
	if err := db.PruneOldMetrics(ctx, 6, 7, 30); err != nil {
		t.Fatalf("PruneOldMetrics failed: %v", err)
	}
	rawAfter, err := db.QueryRaw(ctx, "srv-1", now.Add(-15*time.Minute))
	if err != nil || len(rawAfter) != 3 {
		t.Errorf("expected 3 raw metrics retained, got %d", len(rawAfter))
	}
}

func TestPortGateAllowRules(t *testing.T) {
	db, err := OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// 1. Create a global allow rule (applies to all servers)
	globalRule := &models.PortGateAllowRule{
		ServerID:   nil, // Global
		IPOrSubnet: "10.0.0.0/16",
		Comment:    "Internal Corporate Subnet",
	}
	if err := db.CreatePortGateAllowRule(ctx, globalRule); err != nil {
		t.Fatalf("CreatePortGateAllowRule (global) failed: %v", err)
	}
	if globalRule.ID <= 0 {
		t.Fatalf("expected positive rule ID, got %d", globalRule.ID)
	}

	// 2. Create a server-specific single IP rule
	serverID := "srv-survival"
	_ = db.CreateServer(ctx, &models.Server{
		ID:   serverID,
		Name: "Survival World",
		Port: 19132,
		Mode: "survival",
	})
	srvRule := &models.PortGateAllowRule{
		ServerID:   &serverID,
		IPOrSubnet: "192.168.1.55",
		Comment:    "Admin Static IP",
	}
	if err := db.CreatePortGateAllowRule(ctx, srvRule); err != nil {
		t.Fatalf("CreatePortGateAllowRule (srv) failed: %v", err)
	}

	// 3. List rules for srv-survival: should return both global rule and srvRule
	rules, err := db.ListPortGateAllowRules(ctx, &serverID)
	if err != nil {
		t.Fatalf("ListPortGateAllowRules failed: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules for srv-survival, got %d", len(rules))
	}

	// 4. List rules globally: should return all 2 rules
	allRules, err := db.ListPortGateAllowRules(ctx, nil)
	if err != nil || len(allRules) != 2 {
		t.Fatalf("expected 2 global rules, got %d (err: %v)", len(allRules), err)
	}

	// 5. Test IsIPAllowed with CIDR match in 10.0.0.0/16 (matches global)
	rule, allowed, err := db.IsIPAllowed(ctx, "srv-survival", "10.0.5.21")
	if err != nil || !allowed || rule == nil {
		t.Fatalf("expected 10.0.5.21 to be allowed via 10.0.0.0/16, got allowed=%v, err=%v", allowed, err)
	}

	// 6. Test IsIPAllowed with another server ID (should still match global rule)
	rule2, allowed2, err := db.IsIPAllowed(ctx, "srv-other", "10.0.99.1")
	if err != nil || !allowed2 || rule2 == nil {
		t.Fatalf("expected 10.0.99.1 to be allowed for srv-other via global rule, got allowed=%v", allowed2)
	}

	// 7. Test single IP match on srv-survival
	_, allowed3, _ := db.IsIPAllowed(ctx, "srv-survival", "192.168.1.55")
	if !allowed3 {
		t.Fatalf("expected 192.168.1.55 to be allowed on srv-survival")
	}

	// 8. Test single IP on srv-other (should NOT match)
	_, allowed4, _ := db.IsIPAllowed(ctx, "srv-other", "192.168.1.55")
	if allowed4 {
		t.Fatalf("expected 192.168.1.55 to be rejected on srv-other")
	}

	// 9. Unmatched IP
	_, allowed5, _ := db.IsIPAllowed(ctx, "srv-survival", "8.8.8.8")
	if allowed5 {
		t.Fatalf("expected 8.8.8.8 to be rejected")
	}

	// 10. Delete rule
	if err := db.DeletePortGateAllowRule(ctx, srvRule.ID); err != nil {
		t.Fatalf("DeletePortGateAllowRule failed: %v", err)
	}
	rulesAfter, _ := db.ListPortGateAllowRules(ctx, &serverID)
	if len(rulesAfter) != 1 {
		t.Errorf("expected 1 rule after delete, got %d", len(rulesAfter))
	}
}

func TestPortGateBans(t *testing.T) {
	ctx := context.Background()
	db, err := OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	serverID := "srv-survival"
	_ = db.CreateServer(ctx, &models.Server{
		ID:   serverID,
		Name: "Survival World",
	})

	// 1. Create a global ban on a CIDR subnet
	globalBan := &models.PortGateBanRule{
		ServerID:   nil,
		IPOrSubnet: "198.51.100.0/24",
		Reason:     "Malicious subnet scanner",
		BannedBy:   "admin",
	}
	if err := db.CreatePortGateBan(ctx, globalBan); err != nil {
		t.Fatalf("CreatePortGateBan (global) failed: %v", err)
	}
	if globalBan.ID == 0 {
		t.Errorf("expected valid ban ID")
	}

	// 2. Create an instance-specific ban for srv-survival
	srvBan := &models.PortGateBanRule{
		ServerID:   &serverID,
		IPOrSubnet: "203.0.113.50",
		Reason:     "Griefer IP",
		BannedBy:   "admin",
	}
	if err := db.CreatePortGateBan(ctx, srvBan); err != nil {
		t.Fatalf("CreatePortGateBan (srv) failed: %v", err)
	}

	// 3. List bans for srv-survival (should return global + instance = 2)
	bans, err := db.ListPortGateBans(ctx, &serverID)
	if err != nil {
		t.Fatalf("ListPortGateBans failed: %v", err)
	}
	if len(bans) != 2 {
		t.Fatalf("expected 2 bans for srv-survival, got %d", len(bans))
	}

	// 4. List bans globally (nil serverID): returns all 2
	allBans, err := db.ListPortGateBans(ctx, nil)
	if err != nil || len(allBans) != 2 {
		t.Fatalf("expected 2 all bans, got %d", len(allBans))
	}

	// 5. Check IsIPBanned on global CIDR
	banned, reason, err := db.IsIPBanned(ctx, "srv-survival", "198.51.100.42")
	if err != nil || !banned || reason != "Malicious subnet scanner" {
		t.Fatalf("expected 198.51.100.42 to be banned, got banned=%v, reason=%s", banned, reason)
	}

	// Global CIDR also blocks on other servers
	banned2, _, _ := db.IsIPBanned(ctx, "srv-creative", "198.51.100.99")
	if !banned2 {
		t.Fatalf("expected global CIDR to ban on other server too")
	}

	// 6. Check single IP ban on srv-survival
	banned3, reason3, _ := db.IsIPBanned(ctx, "srv-survival", "203.0.113.50")
	if !banned3 || reason3 != "Griefer IP" {
		t.Fatalf("expected 203.0.113.50 to be banned on srv-survival, got %v", banned3)
	}

	// Single IP ban does NOT affect srv-creative
	banned4, _, _ := db.IsIPBanned(ctx, "srv-creative", "203.0.113.50")
	if banned4 {
		t.Fatalf("expected 203.0.113.50 to NOT be banned on srv-creative")
	}

	// Unbanned IP
	banned5, _, _ := db.IsIPBanned(ctx, "srv-survival", "8.8.8.8")
	if banned5 {
		t.Fatalf("expected 8.8.8.8 to NOT be banned")
	}

	// 7. Test RevokeMatchingLeases
	now := time.Now().UTC()
	lease1 := &models.PortGateLease{
		ServerID:    serverID,
		IPAddress:   "203.0.113.50",
		Gamertag:    "BadPlayer",
		KnockMethod: "passphrase",
		GrantedAt:   now,
		ExpiresAt:   now.Add(2 * time.Hour),
		Status:      "active",
	}
	if err := db.CreatePortGateLease(ctx, lease1); err != nil {
		t.Fatalf("CreatePortGateLease failed: %v", err)
	}

	revoked, err := db.RevokeMatchingLeases(ctx, &serverID, "203.0.113.50")
	if err != nil {
		t.Fatalf("RevokeMatchingLeases failed: %v", err)
	}
	if len(revoked) != 1 || revoked[0].ID != lease1.ID {
		t.Fatalf("expected lease %d to be revoked, got %v", lease1.ID, revoked)
	}

	// Verify lease is now revoked in db
	checkLease, err := db.GetLease(ctx, lease1.ID)
	if err != nil || checkLease.Status != "revoked" {
		t.Fatalf("expected lease status revoked, got %s", checkLease.Status)
	}

	// 8. Delete ban (unban)
	if err := db.DeletePortGateBan(ctx, srvBan.ID); err != nil {
		t.Fatalf("DeletePortGateBan failed: %v", err)
	}
	bannedAfter, _, _ := db.IsIPBanned(ctx, "srv-survival", "203.0.113.50")
	if bannedAfter {
		t.Fatalf("expected IP to no longer be banned after deletion")
	}
}

func TestPlanManagementAndQuotas(t *testing.T) {
	ctx := context.Background()
	db, err := OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// 1. Verify default plan was seeded on migration
	defPlan, err := db.GetDefaultPlan(ctx)
	if err != nil {
		t.Fatalf("GetDefaultPlan failed: %v", err)
	}
	if !defPlan.IsDefault {
		t.Errorf("expected default plan to have IsDefault=true")
	}
	if defPlan.MaxServers != 1 {
		t.Errorf("expected default plan max servers 1, got %d", defPlan.MaxServers)
	}

	// 2. Create custom plan
	proPlan := &models.Plan{
		Name:                "Pro Plan",
		Description:         "Advanced tier with 3 servers",
		IsDefault:           false,
		BillingInterval:     "monthly",
		MaxServers:          3,
		MaxMemory:           "4G",
		MaxCPU:              4.0,
		MaxBackupsPerServer: 5,
		MaxDiskMB:           10240,
		MaxPlayerSlots:      20,
		MaxCollaborators:    2,
		AllowCustomPort:     true,
		AllowCustomSeed:     true,
		AllowAddons:         true,
		AllowPortGateKeys:   true,
		AllowTasks:          true,
	}
	if err := db.CreatePlan(ctx, proPlan); err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}
	if proPlan.ID == 0 {
		t.Fatalf("expected non-zero ID for proPlan")
	}

	// 3. Create user with RoleUser (should auto-assign default plan)
	user1, err := db.CreateUser(ctx, "player1", "hash1", models.RoleUser)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if user1.PlanID == nil || *user1.PlanID != defPlan.ID {
		t.Errorf("expected user1 to have default plan ID %d, got %v", defPlan.ID, user1.PlanID)
	}
	if user1.PlanName != defPlan.Name {
		t.Errorf("expected user1 plan name '%s', got '%s'", defPlan.Name, user1.PlanName)
	}

	// 4. Update user to Pro Plan
	exp := time.Now().Add(30 * 24 * time.Hour)
	if err := db.UpdateUserPlan(ctx, user1.ID, &proPlan.ID, "active", &exp); err != nil {
		t.Fatalf("UpdateUserPlan failed: %v", err)
	}
	fetchedUser, err := db.GetUserByID(ctx, user1.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if fetchedUser.PlanID == nil || *fetchedUser.PlanID != proPlan.ID {
		t.Errorf("expected pro plan ID, got %v", fetchedUser.PlanID)
	}
	if fetchedUser.PlanName != "Pro Plan" {
		t.Errorf("expected plan name 'Pro Plan', got '%s'", fetchedUser.PlanName)
	}

	// 5. Server ownership and quota checking
	s1 := &models.Server{
		ID:          "user-srv-1",
		Name:        "User Server 1",
		Port:        19140,
		PortV6:      19141,
		Status:      models.ServerStatusStopped,
		OwnerUserID: &user1.ID,
	}
	if err := db.CreateServer(ctx, s1); err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	sCount, err := db.CountServersByOwner(ctx, user1.ID)
	if err != nil {
		t.Fatalf("CountServersByOwner failed: %v", err)
	}
	if sCount != 1 {
		t.Errorf("expected 1 server owned, got %d", sCount)
	}

	ownedList, err := db.ListServersByOwner(ctx, user1.ID)
	if err != nil {
		t.Fatalf("ListServersByOwner failed: %v", err)
	}
	if len(ownedList) != 1 || ownedList[0].ID != "user-srv-1" {
		t.Errorf("unexpected owned servers list: %+v", ownedList)
	}

	// 6. Plan list with user count
	plans, err := db.ListPlans(ctx)
	if err != nil {
		t.Fatalf("ListPlans failed: %v", err)
	}
	if len(plans) < 2 {
		t.Errorf("expected at least 2 plans, got %d", len(plans))
	}
	for _, p := range plans {
		if p.ID == proPlan.ID && p.UserCount != 1 {
			t.Errorf("expected pro plan user count 1, got %d", p.UserCount)
		}
	}

	// 7. Prevent deleting plan with assigned users
	uCount, err := db.CountUsersByPlanID(ctx, proPlan.ID)
	if err != nil {
		t.Fatalf("CountUsersByPlanID failed: %v", err)
	}
	if uCount != 1 {
		t.Errorf("expected 1 user on proPlan, got %d", uCount)
	}
}

func TestLegacyDatabaseMigration(t *testing.T) {
	ctx := context.Background()
	// Open a raw SQLite db without the new 1.7.0 columns
	rawDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open raw sqlite: %v", err)
	}
	defer rawDB.Close()

	// Simulate old schema (v1.6.1)
	oldSchema := `
	CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'admin',
		created_at TEXT NOT NULL
	);
	CREATE TABLE servers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		version TEXT NOT NULL DEFAULT 'latest',
		port INTEGER NOT NULL DEFAULT 19132,
		portv6 INTEGER NOT NULL DEFAULT 19133,
		status TEXT NOT NULL DEFAULT 'stopped',
		mode TEXT NOT NULL DEFAULT 'survival',
		difficulty TEXT NOT NULL DEFAULT 'normal',
		autostart_on_boot INTEGER NOT NULL DEFAULT 0,
		port_gate_enabled INTEGER NOT NULL DEFAULT 0,
		port_gate_mode TEXT NOT NULL DEFAULT 'gamertag',
		port_gate_timeout INTEGER NOT NULL DEFAULT 7200,
		memory_limit TEXT NOT NULL DEFAULT '2G',
		cpu_limit REAL NOT NULL DEFAULT 2.0,
		container_id TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	`
	if _, err := rawDB.ExecContext(ctx, oldSchema); err != nil {
		t.Fatalf("failed to create old schema: %v", err)
	}

	// Insert an existing user and server
	_, err = rawDB.ExecContext(ctx, "INSERT INTO users (username, password_hash, role, created_at) VALUES ('legacy_admin', 'hash', 'admin', '2026-01-01T00:00:00Z')")
	if err != nil {
		t.Fatalf("failed to insert legacy user: %v", err)
	}

	// Now run ManagerDB.Migrate
	mdb := &ManagerDB{DB: rawDB}
	if err := mdb.Migrate(ctx); err != nil {
		t.Fatalf("migration on legacy database failed: %v", err)
	}

	// Verify columns exist and indices work
	u, err := mdb.GetUserByUsername(ctx, "legacy_admin")
	if err != nil {
		t.Fatalf("GetUserByUsername failed after migration: %v", err)
	}
	if u.PlanStatus != "active" {
		t.Errorf("expected plan_status active, got %s", u.PlanStatus)
	}

	// Verify default plan was created
	defPlan, err := mdb.GetDefaultPlan(ctx)
	if err != nil {
		t.Fatalf("GetDefaultPlan failed: %v", err)
	}
	if defPlan == nil || !defPlan.IsDefault {
		t.Errorf("expected default plan to exist after migration")
	}
}

func TestServerNetworkModePersistence(t *testing.T) {
	ctx := context.Background()
	db, err := OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}
	defer db.Close()

	// 1. Create server with host network mode
	srvHost := &models.Server{
		ID:          "srv-host",
		Name:        "Host Net Server",
		Port:        19132,
		PortV6:      19133,
		NetworkMode: "host",
	}
	if err := db.CreateServer(ctx, srvHost); err != nil {
		t.Fatalf("CreateServer (host) failed: %v", err)
	}

	got, err := db.GetServer(ctx, "srv-host")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if got.NetworkMode != "host" {
		t.Errorf("expected NetworkMode host, got %s", got.NetworkMode)
	}

	// 2. Create server with empty network mode (should default to bridge)
	srvDefault := &models.Server{
		ID:     "srv-default",
		Name:   "Default Net Server",
		Port:   19134,
		PortV6: 19135,
	}
	if err := db.CreateServer(ctx, srvDefault); err != nil {
		t.Fatalf("CreateServer (default) failed: %v", err)
	}

	gotDefault, err := db.GetServer(ctx, "srv-default")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if gotDefault.NetworkMode != "bridge" {
		t.Errorf("expected default NetworkMode bridge, got %s", gotDefault.NetworkMode)
	}

	// 3. Update network mode
	gotDefault.NetworkMode = "host"
	if err := db.UpdateServer(ctx, gotDefault); err != nil {
		t.Fatalf("UpdateServer failed: %v", err)
	}

	updated, err := db.GetServer(ctx, "srv-default")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if updated.NetworkMode != "host" {
		t.Errorf("expected updated NetworkMode host, got %s", updated.NetworkMode)
	}
}


