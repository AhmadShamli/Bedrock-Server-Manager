package clone

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

func TestCopyConfigs(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}
	defer db.Close()

	eng := engine.NewMockEngine()

	// 1. Create Source Server
	srcServer := &models.Server{
		ID:     "srv-source",
		Name:   "Source Server",
		Port:   19132,
		PortV6: 19133,
		Status: models.ServerStatusRunning,
	}
	_ = db.CreateServer(ctx, srcServer)

	// Create Target Server 1 (running)
	tgtServer1 := &models.Server{
		ID:     "srv-target-1",
		Name:   "Target Server One",
		Port:   19142,
		PortV6: 19143,
		Status: models.ServerStatusRunning,
	}
	_ = db.CreateServer(ctx, tgtServer1)

	// Create Target Server 2 (stopped)
	tgtServer2 := &models.Server{
		ID:     "srv-target-2",
		Name:   "Target Server Two",
		Port:   19152,
		PortV6: 19153,
		Status: models.ServerStatusStopped,
	}
	_ = db.CreateServer(ctx, tgtServer2)

	// Prepare source files
	srcDir := filepath.Join(tempDir, "servers", "srv-source")
	_ = os.MkdirAll(srcDir, 0755)

	srcAllowlist := []configfile.AllowlistEntry{
		{Name: "Alice", XUID: "1111", IgnoresPlayerLimit: true},
		{Name: "Bob", XUID: "2222", IgnoresPlayerLimit: false},
	}
	_ = configfile.WriteAllowlist(filepath.Join(srcDir, "allowlist.json"), srcAllowlist)

	srcPerms := []configfile.PermissionEntry{
		{Permission: "operator", XUID: "1111"},
	}
	_ = configfile.WritePermissions(filepath.Join(srcDir, "permissions.json"), srcPerms)

	srcProps := map[string]string{
		"server-name": "Source Server",
		"server-port": "19132",
		"gamemode":    "survival",
		"difficulty":  "hard",
	}
	_ = configfile.WriteProperties(filepath.Join(srcDir, "server.properties"), srcProps, []string{"server-name", "server-port", "gamemode", "difficulty"})

	// Prepare target 1 existing files (has Charlie in allowlist)
	tgt1Dir := filepath.Join(tempDir, "servers", "srv-target-1")
	_ = os.MkdirAll(tgt1Dir, 0755)
	tgt1Allowlist := []configfile.AllowlistEntry{
		{Name: "Charlie", XUID: "3333", IgnoresPlayerLimit: false},
	}
	_ = configfile.WriteAllowlist(filepath.Join(tgt1Dir, "allowlist.json"), tgt1Allowlist)
	tgt1Props := map[string]string{
		"server-name": "Target Server One",
		"server-port": "19142",
		"gamemode":    "creative",
		"level-name":  "Target1World",
	}
	_ = configfile.WriteProperties(filepath.Join(tgt1Dir, "server.properties"), tgt1Props, []string{"server-name", "server-port", "gamemode", "level-name"})

	// Test Case 1: Merge Mode to Target 1
	optsMerge := CopyConfigOptions{
		TargetServerIDs: []string{"srv-target-1"},
		CopyAllowlist:   true,
		CopyPermissions: true,
		CopyProperties:  true,
		Mode:            "merge",
	}

	resMerge, err := CopyConfigs(ctx, "srv-source", optsMerge, tempDir, db, eng)
	if err != nil {
		t.Fatalf("CopyConfigs merge failed: %v", err)
	}

	if len(resMerge.Results) != 1 || !resMerge.Results[0].Success {
		t.Fatalf("expected successful copy to target 1: %+v", resMerge.Results)
	}

	// Verify Target 1 merged allowlist contains Charlie, Alice, Bob
	mergedAllowlist, err := configfile.ReadAllowlist(filepath.Join(tgt1Dir, "allowlist.json"))
	if err != nil {
		t.Fatalf("ReadAllowlist target 1 failed: %v", err)
	}
	if len(mergedAllowlist) != 3 {
		t.Fatalf("expected 3 entries in merged allowlist, got %d: %+v", len(mergedAllowlist), mergedAllowlist)
	}

	// Verify Target 1 properties: gamemode/difficulty updated, but server-port, server-name, and level-name preserved!
	mergedProps, _, err := configfile.ReadProperties(filepath.Join(tgt1Dir, "server.properties"))
	if err != nil {
		t.Fatalf("ReadProperties target 1 failed: %v", err)
	}
	if mergedProps["server-port"] != "19142" {
		t.Errorf("expected target port 19142 to be preserved, got %s", mergedProps["server-port"])
	}
	if mergedProps["server-name"] != "Target Server One" {
		t.Errorf("expected target name 'Target Server One' to be preserved, got %s", mergedProps["server-name"])
	}
	if mergedProps["level-name"] != "Target1World" {
		t.Errorf("expected target level-name 'Target1World' to be preserved, got %s", mergedProps["level-name"])
	}
	if mergedProps["difficulty"] != "hard" {
		t.Errorf("expected difficulty 'hard' copied from source, got %s", mergedProps["difficulty"])
	}

	// Test Case 2: Replace Mode to Target 2 (no prior files)
	optsReplace := CopyConfigOptions{
		TargetServerIDs: []string{"srv-target-2"},
		CopyAllowlist:   true,
		CopyPermissions: true,
		CopyProperties:  false,
		Mode:            "replace",
	}

	resReplace, err := CopyConfigs(ctx, "srv-source", optsReplace, tempDir, db, eng)
	if err != nil {
		t.Fatalf("CopyConfigs replace failed: %v", err)
	}
	if len(resReplace.Results) != 1 || !resReplace.Results[0].Success {
		t.Fatalf("expected successful copy to target 2: %+v", resReplace.Results)
	}

	tgt2Dir := filepath.Join(tempDir, "servers", "srv-target-2")
	tgt2Allowlist, err := configfile.ReadAllowlist(filepath.Join(tgt2Dir, "allowlist.json"))
	if err != nil {
		t.Fatalf("ReadAllowlist target 2 failed: %v", err)
	}
	if len(tgt2Allowlist) != 2 {
		t.Fatalf("expected exactly 2 entries in replaced allowlist, got %d", len(tgt2Allowlist))
	}

	// Test Case 3: Error validation (empty targets, self target, invalid source)
	_, err = CopyConfigs(ctx, "srv-source", CopyConfigOptions{}, tempDir, db, eng)
	if err == nil {
		t.Errorf("expected error for empty targets")
	}

	resSelf, err := CopyConfigs(ctx, "srv-source", CopyConfigOptions{
		TargetServerIDs: []string{"srv-source", "srv-target-1"},
		CopyAllowlist:   true,
	}, tempDir, db, eng)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resSelf.Results[0].Success || resSelf.Results[0].Error != "cannot copy configuration to source server itself" {
		t.Errorf("expected self copy to fail with specific error, got %+v", resSelf.Results[0])
	}
	if !resSelf.Results[1].Success {
		t.Errorf("expected second target to succeed")
	}
}
