package addon

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAddonInstallAndListFlow(t *testing.T) {
	serverDir := t.TempDir()

	// 1. Create in-memory zip of a behavior pack
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	manifestContent := `{
		"format_version": 2,
		"header": {
			"name": "Custom Weapons Pack",
			"description": "Adds awesome weapons",
			"uuid": "11111111-2222-3333-4444-555555555555",
			"version": [1, 2, 3]
		},
		"modules": [
			{
				"type": "data",
				"uuid": "66666666-7777-8888-9999-000000000000"
			}
		]
	}`

	w, _ := zw.Create("manifest.json")
	_, _ = w.Write([]byte(manifestContent))
	w2, _ := zw.Create("items/sword.json")
	_, _ = w2.Write([]byte(`{"item": "sword"}`))
	_ = zw.Close()

	// 2. Install pack
	reader := bytes.NewReader(buf.Bytes())
	pack, err := InstallPack(serverDir, reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("InstallPack failed: %v", err)
	}

	if pack.Type != "behavior" {
		t.Fatalf("expected behavior pack, got %s", pack.Type)
	}
	if pack.Name != "Custom Weapons Pack" {
		t.Fatalf("unexpected pack name: %s", pack.Name)
	}
	if pack.Version != "1.2.3" {
		t.Fatalf("unexpected version: %s", pack.Version)
	}

	// 3. Verify file extracted
	swordFile := filepath.Join(serverDir, "behavior_packs", pack.Folder, "items", "sword.json")
	if _, err := os.Stat(swordFile); err != nil {
		t.Fatalf("expected items/sword.json on disk: %v", err)
	}

	// 4. List packs
	packs, err := ListInstalledPacks(serverDir)
	if err != nil {
		t.Fatalf("ListInstalledPacks failed: %v", err)
	}
	if len(packs) != 1 || packs[0].Folder != pack.Folder {
		t.Fatalf("unexpected list of packs: %+v", packs)
	}

	// 5. Delete pack
	if err := DeletePack(serverDir, pack.Type, pack.Folder); err != nil {
		t.Fatalf("DeletePack failed: %v", err)
	}

	remainingPacks, _ := ListInstalledPacks(serverDir)
	if len(remainingPacks) != 0 {
		t.Fatalf("expected 0 packs remaining, got %d", len(remainingPacks))
	}
}

func TestAddonInstallNestedFolder(t *testing.T) {
	serverDir := t.TempDir()

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	manifestContent := `{
		"format_version": 2,
		"header": {
			"name": "Nested Resource Pack",
			"description": "Nested in folder",
			"uuid": "22222222-3333-4444-5555-666666666666",
			"version": [1, 0, 0]
		},
		"modules": [
			{
				"type": "resources",
				"uuid": "77777777-8888-9999-0000-111111111111"
			}
		]
	}`

	// Pack wrapped in top-level directory "MyPack/"
	w, _ := zw.Create("MyPack/manifest.json")
	_, _ = w.Write([]byte(manifestContent))
	w2, _ := zw.Create("MyPack/textures/item.png")
	_, _ = w2.Write([]byte("fake_png_data"))
	_ = zw.Close()

	reader := bytes.NewReader(buf.Bytes())
	pack, err := InstallPack(serverDir, reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("InstallPack failed: %v", err)
	}

	if pack.Type != "resource" {
		t.Fatalf("expected resource pack, got %s", pack.Type)
	}

	// Verify manifest.json is directly at the pack root on disk (not under MyPack/MyPack/manifest.json)
	manifestOnDisk := filepath.Join(serverDir, "resource_packs", pack.Folder, "manifest.json")
	if _, err := os.Stat(manifestOnDisk); err != nil {
		t.Fatalf("expected manifest.json at pack root: %v", err)
	}

	textureOnDisk := filepath.Join(serverDir, "resource_packs", pack.Folder, "textures", "item.png")
	if _, err := os.Stat(textureOnDisk); err != nil {
		t.Fatalf("expected textures/item.png at pack root: %v", err)
	}
}

func TestDeletePackPathTraversal(t *testing.T) {
	serverDir := t.TempDir()

	// Should reject traversal attempts
	traversalCases := []struct {
		packType string
		folder   string
	}{
		{"behavior", "../worlds"},
		{"behavior", "../../servers"},
		{"resource", ".."},
		{"resource", "/etc/passwd"},
		{"invalid_type", "valid_folder"},
	}

	for _, tc := range traversalCases {
		err := DeletePack(serverDir, tc.packType, tc.folder)
		if err == nil {
			t.Errorf("expected error for traversal case %s / %s, got nil", tc.packType, tc.folder)
		}
	}
}

func TestListIgnoresBuiltInPacks(t *testing.T) {
	serverDir := t.TempDir()

	// Simulate a built-in BDS pack (e.g. "vanilla") - has manifest.json but NO .bsm_installed marker
	vanillaDir := filepath.Join(serverDir, "behavior_packs", "vanilla")
	if err := os.MkdirAll(vanillaDir, 0755); err != nil {
		t.Fatal(err)
	}
	vanillaManifest := `{
		"format_version": 2,
		"header": {
			"name": "Vanilla",
			"description": "Default Minecraft behaviour pack",
			"uuid": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			"version": [1, 0, 0]
		},
		"modules": [{"type": "data", "uuid": "ffffffff-0000-1111-2222-333333333333"}]
	}`
	if err := os.WriteFile(filepath.Join(vanillaDir, "manifest.json"), []byte(vanillaManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Simulate another built-in pack "chemistry"
	chemDir := filepath.Join(serverDir, "resource_packs", "chemistry")
	if err := os.MkdirAll(chemDir, 0755); err != nil {
		t.Fatal(err)
	}
	chemManifest := `{
		"format_version": 2,
		"header": {
			"name": "Chemistry",
			"description": "Education Edition chemistry features",
			"uuid": "11111111-aaaa-bbbb-cccc-dddddddddddd",
			"version": [1, 0, 0]
		},
		"modules": [{"type": "resources", "uuid": "22222222-eeee-ffff-0000-111111111111"}]
	}`
	if err := os.WriteFile(filepath.Join(chemDir, "manifest.json"), []byte(chemManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// ListInstalledPacks should return 0 packs (built-in packs have no marker)
	packs, err := ListInstalledPacks(serverDir)
	if err != nil {
		t.Fatalf("ListInstalledPacks failed: %v", err)
	}
	if len(packs) != 0 {
		t.Fatalf("expected 0 user-installed packs but got %d: %+v", len(packs), packs)
	}

	// Now install a user pack via InstallPack and verify it IS listed
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("manifest.json")
	_, _ = w.Write([]byte(`{
		"format_version": 2,
		"header": {
			"name": "My Custom Pack",
			"description": "User installed",
			"uuid": "99999999-8888-7777-6666-555555555555",
			"version": [2, 0, 1]
		},
		"modules": [{"type": "data", "uuid": "44444444-3333-2222-1111-000000000000"}]
	}`))
	_ = zw.Close()

	reader := bytes.NewReader(buf.Bytes())
	pack, err := InstallPack(serverDir, reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("InstallPack failed: %v", err)
	}

	// Now listing should return exactly 1 (the user-installed pack)
	packs, err = ListInstalledPacks(serverDir)
	if err != nil {
		t.Fatalf("ListInstalledPacks failed: %v", err)
	}
	if len(packs) != 1 {
		t.Fatalf("expected 1 user-installed pack but got %d: %+v", len(packs), packs)
	}
	if packs[0].UUID != pack.UUID {
		t.Fatalf("listed pack UUID %s does not match installed %s", packs[0].UUID, pack.UUID)
	}
}
