package player

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/configfile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func TestSyncServerWithGlobal(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}
	defer db.Close()

	mockEng := engine.NewMockEngine()

	// 1. Create registered server
	server := &models.Server{
		ID:     "srv-multi-test",
		Name:   "Multi Level Server",
		Port:   19132,
		PortV6: 19133,
		Status: models.ServerStatusRunning,
	}
	_ = db.CreateServer(ctx, server)

	// 2. Setup existing local server files with 1 local player
	serverDir := filepath.Join(tempDir, "servers", "srv-multi-test")
	_ = os.MkdirAll(serverDir, 0755)

	localAllowlist := []configfile.AllowlistEntry{
		{Name: "LocalSteve", XUID: "1000", IgnoresPlayerLimit: false},
	}
	_ = configfile.WriteAllowlist(filepath.Join(serverDir, "allowlist.json"), localAllowlist)

	localPerms := []configfile.PermissionEntry{
		{Permission: "member", XUID: "1000"},
	}
	_ = configfile.WritePermissions(filepath.Join(serverDir, "permissions.json"), localPerms)

	// 3. Populate global players in DB
	gp1 := &models.GlobalPlayer{
		Name:          "GlobalAlex",
		XUID:          "2000",
		IsAllowlisted: true,
		Permission:    "operator",
	}
	gp2 := &models.GlobalPlayer{
		Name:          "GlobalWatcher",
		XUID:          "3000",
		IsAllowlisted: true,
		Permission:    "visitor",
	}
	_, _ = db.UpsertGlobalPlayer(ctx, gp1)
	_, _ = db.UpsertGlobalPlayer(ctx, gp2)

	// 4. Run Sync
	report, err := SyncServerWithGlobal(ctx, tempDir, "srv-multi-test", db, mockEng)
	if err != nil {
		t.Fatalf("SyncServerWithGlobal failed: %v", err)
	}

	if len(report.AllowlistAdded) != 2 {
		t.Errorf("expected 2 players added to allowlist, got %d: %+v", len(report.AllowlistAdded), report.AllowlistAdded)
	}

	// 5. Verify allowlist on disk: contains LocalSteve, GlobalAlex, GlobalWatcher
	diskAllowlist, err := configfile.ReadAllowlist(filepath.Join(serverDir, "allowlist.json"))
	if err != nil {
		t.Fatalf("ReadAllowlist failed: %v", err)
	}
	if len(diskAllowlist) != 3 {
		t.Errorf("expected 3 entries on disk allowlist, got %d", len(diskAllowlist))
	}

	// 6. Verify permissions on disk: contains LocalSteve (member), GlobalAlex (operator), GlobalWatcher (visitor)
	diskPerms, err := configfile.ReadPermissions(filepath.Join(serverDir, "permissions.json"))
	if err != nil {
		t.Fatalf("ReadPermissions failed: %v", err)
	}
	if len(diskPerms) != 3 {
		t.Errorf("expected 3 entries in permissions.json, got %d", len(diskPerms))
	}

	var foundOp bool
	for _, p := range diskPerms {
		if p.XUID == "2000" && p.Permission == "operator" {
			foundOp = true
		}
	}
	if !foundOp {
		t.Errorf("expected GlobalAlex (xuid 2000) to have operator permission")
	}

	// 7. Second Sync should be idempotent (no additions)
	report2, err := SyncServerWithGlobal(ctx, tempDir, "srv-multi-test", db, mockEng)
	if err != nil {
		t.Fatalf("Second sync failed: %v", err)
	}
	if len(report2.AllowlistAdded) != 0 || len(report2.PermissionsUpdated) != 0 {
		t.Errorf("expected 0 updates on second sync, got: %+v", report2)
	}
}
