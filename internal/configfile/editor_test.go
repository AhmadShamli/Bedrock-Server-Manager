package configfile

import (
	"path/filepath"
	"testing"
)

func TestSafePath(t *testing.T) {
	tempDir := t.TempDir()

	// Valid path
	path, err := SafePath(tempDir, "srv-1", "server.properties")
	if err != nil {
		t.Fatalf("unexpected error on valid SafePath: %v", err)
	}
	expected := filepath.Join(tempDir, "servers", "srv-1", "server.properties")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}

	// Path traversal attempt with ..
	_, err = SafePath(tempDir, "srv-1", "../../etc/passwd")
	if err == nil {
		t.Errorf("expected error on path traversal attempt")
	}

	// Absolute path attempt
	_, err = SafePath(tempDir, "srv-1", "/etc/passwd")
	if err == nil {
		t.Errorf("expected error on absolute path")
	}

	// Backslash traversal attempt on Unix/Windows
	_, err = SafePath(tempDir, "srv-1", `..\something`)
	if err == nil {
		t.Errorf("expected error on backslash traversal")
	}
}

func TestPropertiesReaderWriter(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "server.properties")

	initialProps := map[string]string{
		"server-name":  "My Bedrock Realm",
		"gamemode":     "survival",
		"difficulty":   "hard",
		"allow-cheats": "false",
	}
	keys := []string{"server-name", "gamemode", "difficulty", "allow-cheats"}

	if err := WriteProperties(tempFile, initialProps, keys); err != nil {
		t.Fatalf("WriteProperties failed: %v", err)
	}

	readProps, readKeys, err := ReadProperties(tempFile)
	if err != nil {
		t.Fatalf("ReadProperties failed: %v", err)
	}

	if len(readKeys) != 4 {
		t.Errorf("expected 4 keys, got %d", len(readKeys))
	}
	if readProps["server-name"] != "My Bedrock Realm" {
		t.Errorf("unexpected server-name: %s", readProps["server-name"])
	}
	if readProps["gamemode"] != "survival" {
		t.Errorf("unexpected gamemode: %s", readProps["gamemode"])
	}
}

func TestAllowlistReaderWriter(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "allowlist.json")

	list := []AllowlistEntry{
		{Name: "Steve", XUID: "123456789", IgnoresPlayerLimit: false},
		{Name: "Alex", XUID: "987654321", IgnoresPlayerLimit: true},
	}

	if err := WriteAllowlist(tempFile, list); err != nil {
		t.Fatalf("WriteAllowlist failed: %v", err)
	}

	loaded, err := ReadAllowlist(tempFile)
	if err != nil {
		t.Fatalf("ReadAllowlist failed: %v", err)
	}

	if len(loaded) != 2 || loaded[0].Name != "Steve" || !loaded[1].IgnoresPlayerLimit {
		t.Errorf("unexpected loaded allowlist: %+v", loaded)
	}
}

func TestPermissionsReaderWriter(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "permissions.json")

	list := []PermissionEntry{
		{Permission: "operator", XUID: "123456789"},
	}

	if err := WritePermissions(tempFile, list); err != nil {
		t.Fatalf("WritePermissions failed: %v", err)
	}

	loaded, err := ReadPermissions(tempFile)
	if err != nil {
		t.Fatalf("ReadPermissions failed: %v", err)
	}

	if len(loaded) != 1 || loaded[0].Permission != "operator" {
		t.Errorf("unexpected loaded permissions: %+v", loaded)
	}
}

func TestMergeProperties(t *testing.T) {
	initialProps := map[string]string{
		"server-name": "Mojang Server",
		"gamemode":    "survival",
		"difficulty":  "easy",
	}
	initialKeys := []string{"server-name", "gamemode", "difficulty"}

	updates := map[string]string{
		"gamemode":   "creative",
		"level-seed": "12345",
	}

	mergedProps, mergedKeys := MergeProperties(initialProps, initialKeys, updates)

	if mergedProps["server-name"] != "Mojang Server" {
		t.Errorf("expected server-name preserved, got %s", mergedProps["server-name"])
	}
	if mergedProps["gamemode"] != "creative" {
		t.Errorf("expected gamemode updated to creative, got %s", mergedProps["gamemode"])
	}
	if mergedProps["level-seed"] != "12345" {
		t.Errorf("expected level-seed added, got %s", mergedProps["level-seed"])
	}
	if len(mergedKeys) != 4 {
		t.Errorf("expected 4 keys, got %d", len(mergedKeys))
	}
}

func TestUpdateExistingPropertyFile(t *testing.T) {
	tempDir := t.TempDir()
	nonExistent := filepath.Join(tempDir, "server.properties")

	// 1. Should not create file if it does not exist
	updated, err := UpdateExistingPropertyFile(nonExistent, map[string]string{"level-seed": "999"})
	if err != nil {
		t.Fatalf("unexpected error on non-existent file: %v", err)
	}
	if updated {
		t.Errorf("expected updated=false for non-existent file")
	}

	// 2. Should update file if it exists
	initialProps := map[string]string{
		"server-name": "Existing Server",
		"difficulty":  "normal",
	}
	_ = WriteProperties(nonExistent, initialProps, []string{"server-name", "difficulty"})

	updated, err = UpdateExistingPropertyFile(nonExistent, map[string]string{
		"difficulty": "hard",
		"level-seed": "888",
	})
	if err != nil {
		t.Fatalf("unexpected error on existing file: %v", err)
	}
	if !updated {
		t.Errorf("expected updated=true for existing file")
	}

	readProps, readKeys, _ := ReadProperties(nonExistent)
	if readProps["difficulty"] != "hard" {
		t.Errorf("expected difficulty 'hard', got %s", readProps["difficulty"])
	}
	if readProps["level-seed"] != "888" {
		t.Errorf("expected level-seed '888', got %s", readProps["level-seed"])
	}
	if readProps["server-name"] != "Existing Server" {
		t.Errorf("expected server-name 'Existing Server', got %s", readProps["server-name"])
	}
	if len(readKeys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(readKeys))
	}
}

