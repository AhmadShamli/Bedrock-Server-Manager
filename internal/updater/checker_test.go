package updater

import (
	"context"
	"testing"
)

func TestCheckerFallback(t *testing.T) {
	c := NewChecker()
	ctx := context.Background()

	// Should not crash and should return safe fallback if offline / rate limited
	info, err := c.CheckUpdate(ctx, "latest")
	if err != nil {
		t.Fatalf("CheckUpdate failed: %v", err)
	}

	if info == nil || info.CurrentVersion != "latest" {
		t.Errorf("unexpected info: %+v", info)
	}
}
