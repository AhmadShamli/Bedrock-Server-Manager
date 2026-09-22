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
