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
