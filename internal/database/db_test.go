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
	}
	if err := db.CreateServer(ctx, srv); err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}

	gotSrv, err := db.GetServer(ctx, "test-srv-1")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if gotSrv.Name != "Survival World" || !gotSrv.AutostartOnBoot || !gotSrv.PortGateEnabled {
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
