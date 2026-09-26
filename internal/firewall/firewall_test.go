package firewall

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func TestMockFirewallDriver(t *testing.T) {
	ctx := context.Background()
	driver := NewMockFirewallDriver()

	if driver.Name() != "mock" {
		t.Fatalf("expected name mock, got %s", driver.Name())
	}

	detected, err := driver.Detect(ctx)
	if err != nil || !detected {
		t.Fatalf("detect failed: %v", err)
	}

	if err := driver.AllowPort(ctx, "192.168.1.50", 19132, "test"); err != nil {
		t.Fatalf("allow port failed: %v", err)
	}

	if !driver.HasRule("192.168.1.50", 19132) {
		t.Fatalf("expected rule to exist")
	}

	rules, err := driver.ListActiveRules(ctx)
	if err != nil || len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	if err := driver.RevokePort(ctx, "192.168.1.50", 19132, "test"); err != nil {
		t.Fatalf("revoke port failed: %v", err)
	}

	if driver.HasRule("192.168.1.50", 19132) {
		t.Fatalf("expected rule to be revoked")
	}

	// Test flush
	_ = driver.AllowPort(ctx, "10.0.0.1", 19132, "c1")
	_ = driver.AllowPort(ctx, "10.0.0.2", 19133, "c2")
	if driver.TotalRules() != 2 {
		t.Fatalf("expected 2 rules, got %d", driver.TotalRules())
	}
	_ = driver.FlushRules(ctx)
	if driver.TotalRules() != 0 {
		t.Fatalf("expected 0 rules after flush, got %d", driver.TotalRules())
	}
}

func TestParseUFWStatusLines(t *testing.T) {
	sampleOutput := `
Status: active

     To                         Action      From
     --                         ------      ----
[ 1] 22/tcp                     ALLOW IN    Anywhere
[ 2] 19132/udp                  ALLOW IN    198.51.100.12 # bsm_srv1_101
[ 3] 19133/udp                  ALLOW IN    2001:db8::1 # bsm_srv1_102
[ 4] 80/tcp                     ALLOW IN    Anywhere
`
	rules := parseUFWStatusLines(sampleOutput)
	if len(rules) != 2 {
		t.Fatalf("expected 2 UDP rules, got %d", len(rules))
	}

	if rules[0].Port != 19132 || rules[0].IP != "198.51.100.12" || rules[0].Comment != "bsm_srv1_101" {
		t.Fatalf("unexpected rule 0: %+v", rules[0])
	}
	if rules[1].Port != 19133 || rules[1].IP != "2001:db8::1" || rules[1].Comment != "bsm_srv1_102" {
		t.Fatalf("unexpected rule 1: %+v", rules[1])
	}
}

func TestLeaseAuditor(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	mgrDB, err := database.OpenManagerDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer mgrDB.Close()

	// Create test server
	server := &models.Server{
		ID:              "test-server-audit",
		Name:            "Test Audit",
		Port:            19132,
		PortV6:          19133,
		PortGateEnabled: true,
		PortGateMode:    "passphrase",
		Status:          "running",
	}
	if err := mgrDB.CreateServer(ctx, server); err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	driver := NewMockFirewallDriver()
	auditor := NewLeaseAuditor(mgrDB, driver, 100*time.Millisecond)

	// Add an active lease that already expired 1 minute ago
	now := time.Now().UTC()
	expiredLease := &models.PortGateLease{
		ServerID:         server.ID,
		IPAddress:        "203.0.113.5",
		Gamertag:         "ExpiredPlayer",
		KnockMethod:      "passphrase",
		SessionTokenHash: "dummyhash",
		GrantedAt:        now.Add(-10 * time.Minute),
		ExpiresAt:        now.Add(-1 * time.Minute),
		Status:           "active",
	}
	if err := mgrDB.CreatePortGateLease(ctx, expiredLease); err != nil {
		t.Fatalf("failed to create expired lease: %v", err)
	}

	// Add the rule to the mock firewall
	_ = driver.AllowPort(ctx, "203.0.113.5", 19132, "bsm_test-server-audit_1")
	_ = driver.AllowPort(ctx, "203.0.113.5", 19133, "bsm_test-server-audit_1")
	if !driver.HasRule("203.0.113.5", 19132) {
		t.Fatalf("rule should exist initially")
	}

	// Run audit pass
	revoked, err := auditor.AuditOnce(ctx)
	if err != nil {
		t.Fatalf("AuditOnce failed: %v", err)
	}
	if revoked != 1 {
		t.Fatalf("expected 1 revoked lease, got %d", revoked)
	}

	// Firewall rules should be removed
	if driver.HasRule("203.0.113.5", 19132) || driver.HasRule("203.0.113.5", 19133) {
		t.Fatalf("firewall rules were not revoked")
	}

	// Lease in DB should now have status = 'expired'
	refreshed, err := mgrDB.GetLease(ctx, expiredLease.ID)
	if err != nil {
		t.Fatalf("failed to fetch lease: %v", err)
	}
	if refreshed.Status != "expired" {
		t.Fatalf("expected status 'expired', got '%s'", refreshed.Status)
	}
}

func TestFindExecutable(t *testing.T) {
	// Should resolve sh or ls
	shPath := FindExecutable("sh")
	if shPath == "sh" && !filepath.IsAbs(shPath) {
		t.Logf("FindExecutable resolved 'sh' to '%s'", shPath)
	}

	// Should preserve existing absolute paths
	abs := "/usr/bin/custom-tool"
	if FindExecutable(abs) != abs {
		t.Fatalf("expected '%s', got '%s'", abs, FindExecutable(abs))
	}
}
