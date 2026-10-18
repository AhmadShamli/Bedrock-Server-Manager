package firewall

import (
	"context"
	"testing"
)

func TestWindowsFirewallDriver(t *testing.T) {
	driver := NewWindowsFirewallDriver()
	if driver.Name() != "windows" {
		t.Fatalf("expected name windows, got %s", driver.Name())
	}

	ctx := context.Background()

	// 1. Add IP to port 19132
	_ = driver.AllowPort(ctx, "1.2.3.4", 19132, "test")
	_ = driver.AllowPort(ctx, "5.6.7.8", 19132, "test")

	rules, err := driver.ListActiveRules(ctx)
	if err != nil {
		t.Fatalf("failed to list rules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 active rules, got %d", len(rules))
	}

	// 2. Revoke one IP
	_ = driver.RevokePort(ctx, "1.2.3.4", 19132, "test")
	rules, err = driver.ListActiveRules(ctx)
	if err != nil {
		t.Fatalf("failed to list rules: %v", err)
	}
	if len(rules) != 1 || rules[0].IP != "5.6.7.8" {
		t.Fatalf("expected only 5.6.7.8, got %v", rules)
	}

	// 3. Revoke last IP (Zero-lease state)
	_ = driver.RevokePort(ctx, "5.6.7.8", 19132, "test")
	rules, err = driver.ListActiveRules(ctx)
	if err != nil {
		t.Fatalf("failed to list rules: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected 0 active rules, got %d", len(rules))
	}

	// 4. Flush
	_ = driver.AllowPort(ctx, "9.9.9.9", 19133, "test")
	_ = driver.FlushRules(ctx)
	rules, _ = driver.ListActiveRules(ctx)
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules after flush, got %d", len(rules))
	}
}
