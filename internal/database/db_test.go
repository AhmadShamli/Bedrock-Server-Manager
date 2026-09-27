package database

import (
	"context"
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

