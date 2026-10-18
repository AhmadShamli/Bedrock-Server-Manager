package addon

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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

func TestInstallFromURL(t *testing.T) {
	serverDir := t.TempDir()

	// Create test server serving a valid zip pack
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("manifest.json")
	_, _ = w.Write([]byte(`{
		"format_version": 2,
		"header": {
			"name": "URL Remote Pack",
			"description": "Installed from direct link",
			"uuid": "43214321-4321-4321-4321-432143214321",
			"version": [1, 5, 0]
		},
		"modules": [{"type": "data", "uuid": "87658765-8765-8765-8765-876587658765"}]
	}`))
	_ = zw.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(buf.Bytes())
	}))
	defer ts.Close()

	pack, err := InstallFromURL(context.Background(), serverDir, ts.URL+"/pack.mcpack")
	if err != nil {
		t.Fatalf("InstallFromURL failed: %v", err)
	}
	if pack.Name != "URL Remote Pack" {
		t.Fatalf("unexpected pack name: %s", pack.Name)
	}
	if pack.Type != "behavior" {
		t.Fatalf("unexpected pack type: %s", pack.Type)
	}
}

func TestInstallMcaddonNested(t *testing.T) {
	serverDir := t.TempDir()

	// Create inner behavior pack
	bpBuf := new(bytes.Buffer)
	bpZw := zip.NewWriter(bpBuf)
	bpW, _ := bpZw.Create("manifest.json")
	_, _ = bpW.Write([]byte(`{
		"format_version": 2,
		"header": {
			"name": "Nested BP",
			"description": "Nested BP in mcaddon",
			"uuid": "bbbbbbbb-1111-2222-3333-444444444444",
			"version": [1, 0, 0]
		},
		"modules": [{"type": "data", "uuid": "cccccccc-1111-2222-3333-444444444444"}]
	}`))
	_ = bpZw.Close()

	// Outer mcaddon containing the inner .mcpack
	outerBuf := new(bytes.Buffer)
	outerZw := zip.NewWriter(outerBuf)
	innerF, _ := outerZw.Create("addon_bp.mcpack")
	_, _ = innerF.Write(bpBuf.Bytes())
	_ = outerZw.Close()

	pack, err := InstallPack(serverDir, bytes.NewReader(outerBuf.Bytes()), int64(outerBuf.Len()))
	if err != nil {
		t.Fatalf("InstallPack nested mcaddon failed: %v", err)
	}
	if pack.Name != "Nested BP" {
		t.Fatalf("unexpected pack name: %s", pack.Name)
	}
}

func TestPackActivationFlow(t *testing.T) {
	serverDir := t.TempDir()

	// 1. Install a behavior pack
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("manifest.json")
	_, _ = w.Write([]byte(`{
		"format_version": 2,
		"header": {
			"name": "Activation Test Pack",
			"description": "Test activation",
			"uuid": "99999999-8888-7777-6666-555555555555",
			"version": [2, 0, 1]
		},
		"modules": [{"type": "data", "uuid": "88888888-7777-6666-5555-444444444444"}]
	}`))
	_ = zw.Close()

	pack, err := InstallPack(serverDir, bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("InstallPack failed: %v", err)
	}

	// Initially inactive
	packs, err := ListInstalledPacks(serverDir)
	if err != nil || len(packs) != 1 {
		t.Fatalf("ListInstalledPacks failed: %v", err)
	}
	if packs[0].Active {
		t.Errorf("expected pack to be inactive initially")
	}

	// Activate pack
	updated, err := SetPackActive(serverDir, pack.Type, pack.Folder, true)
	if err != nil {
		t.Fatalf("SetPackActive true failed: %v", err)
	}
	if !updated.Active {
		t.Errorf("expected updated pack to be active")
	}

	// Verify world_behavior_packs.json written
	levelName := GetActiveLevelName(serverDir)
	refs, err := GetWorldPackRefs(serverDir, levelName, "behavior")
	if err != nil || len(refs) != 1 {
		t.Fatalf("expected 1 ref in world_behavior_packs.json, got: %v", refs)
	}
	if refs[0].PackID != "99999999-8888-7777-6666-555555555555" {
		t.Errorf("unexpected pack_id: %s", refs[0].PackID)
	}
	if len(refs[0].Version) != 3 || refs[0].Version[0] != 2 {
		t.Errorf("unexpected version: %+v", refs[0].Version)
	}

	// Verify ListInstalledPacks shows active: true
	packs, _ = ListInstalledPacks(serverDir)
	if !packs[0].Active {
		t.Errorf("expected ListInstalledPacks to report active: true")
	}

	// Deactivate pack
	updated, err = SetPackActive(serverDir, pack.Type, pack.Folder, false)
	if err != nil {
		t.Fatalf("SetPackActive false failed: %v", err)
	}
	if updated.Active {
		t.Errorf("expected updated pack to be inactive")
	}

	refs, _ = GetWorldPackRefs(serverDir, levelName, "behavior")
	if len(refs) != 0 {
		t.Errorf("expected 0 refs after deactivation, got %d", len(refs))
	}
}

func TestPackReorderingFlow(t *testing.T) {
	serverDir := t.TempDir()
	levelName := GetActiveLevelName(serverDir)

	initialRefs := []WorldPackRef{
		{PackID: "uuid-1", Version: []int{1, 0, 0}},
		{PackID: "uuid-2", Version: []int{1, 0, 0}},
		{PackID: "uuid-3", Version: []int{1, 0, 0}},
	}
	_ = SaveWorldPackRefs(serverDir, levelName, "resource", initialRefs)

	// Reorder to uuid-3, uuid-1, uuid-2
	err := SetPackOrder(serverDir, "resource", []string{"uuid-3", "uuid-1", "uuid-2"})
	if err != nil {
		t.Fatalf("SetPackOrder failed: %v", err)
	}

	refs, err := GetWorldPackRefs(serverDir, levelName, "resource")
	if err != nil || len(refs) != 3 {
		t.Fatalf("unexpected refs: %+v", refs)
	}
	if refs[0].PackID != "uuid-3" || refs[1].PackID != "uuid-1" || refs[2].PackID != "uuid-2" {
		t.Errorf("reordering failed: %+v", refs)
	}
}

func TestAddonConfigFlow(t *testing.T) {
	serverDir := t.TempDir()

	// Default config
	cfg := GetAddonConfig(serverDir)
	if cfg.ActiveWorld != "Bedrock level" || cfg.TexturePackRequired {
		t.Errorf("unexpected initial config: %+v", cfg)
	}

	// Update texturepack-required
	err := UpdateAddonConfig(serverDir, true)
	if err != nil {
		t.Fatalf("UpdateAddonConfig failed: %v", err)
	}

	cfg = GetAddonConfig(serverDir)
	if !cfg.TexturePackRequired {
		t.Errorf("expected TexturePackRequired to be true")
	}
}

func TestAddonInstallMultiFolderMCAddonAndActivation(t *testing.T) {
	serverDir := t.TempDir()

	rpUUID := "33333333-4444-5555-6666-777777777777"
	bpUUID := "11111111-2222-3333-4444-555555555555"

	// Create a .mcaddon archive with separate BP and RP folders
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// 1. Behavior Pack with dependency on RP
	bpManifest := `{
		"format_version": 2,
		"header": {
			"name": "Super Dragon BP",
			"description": "Dragon behaviors",
			"uuid": "` + bpUUID + `",
			"version": [1, 0, 0]
		},
		"modules": [
			{
				"type": "data",
				"uuid": "66666666-7777-8888-9999-000000000000"
			}
		],
		"dependencies": [
			{
				"uuid": "` + rpUUID + `",
				"version": [1, 0, 0]
			}
		]
	}`
	wBP, _ := zw.Create("dragon_bp/manifest.json")
	_, _ = wBP.Write([]byte(bpManifest))
	wBPEntity, _ := zw.Create("dragon_bp/entities/dragon.json")
	_, _ = wBPEntity.Write([]byte(`{"entity": "dragon"}`))

	// 2. Resource Pack
	rpManifest := `{
		"format_version": 2,
		"header": {
			"name": "Super Dragon RP",
			"description": "Dragon textures",
			"uuid": "` + rpUUID + `",
			"version": [1, 0, 0]
		},
		"modules": [
			{
				"type": "resources",
				"uuid": "77777777-8888-9999-0000-111111111111"
			}
		]
	}`
	wRP, _ := zw.Create("dragon_rp/manifest.json")
	_, _ = wRP.Write([]byte(rpManifest))
	wRPTexture, _ := zw.Create("dragon_rp/textures/dragon.png")
	_, _ = wRPTexture.Write([]byte(`fake-image-bytes`))

	_ = zw.Close()

	// Install archive
	packs, err := InstallAllPacks(serverDir, bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("InstallAllPacks failed: %v", err)
	}
	if len(packs) != 2 {
		t.Fatalf("expected 2 packs installed, got %d", len(packs))
	}

	// Verify both packs are listed
	installed, err := ListInstalledPacks(serverDir)
	if err != nil || len(installed) != 2 {
		t.Fatalf("expected 2 installed packs, got %d", len(installed))
	}

	// Verify folders on disk
	bpFile := filepath.Join(serverDir, "behavior_packs", "Super_Dragon_BP", "entities", "dragon.json")
	if _, err := os.Stat(bpFile); err != nil {
		t.Fatalf("expected behavior pack file on disk: %v", err)
	}
	rpFile := filepath.Join(serverDir, "resource_packs", "Super_Dragon_RP", "textures", "dragon.png")
	if _, err := os.Stat(rpFile); err != nil {
		t.Fatalf("expected resource pack file on disk: %v", err)
	}

	// Now activate the behavior pack
	_, err = SetPackActive(serverDir, "behavior", "Super_Dragon_BP", true)
	if err != nil {
		t.Fatalf("SetPackActive on BP failed: %v", err)
	}

	// Verify that companion RP was auto-activated!
	levelName := GetActiveLevelName(serverDir)
	rpRefs, err := GetWorldPackRefs(serverDir, levelName, "resource")
	if err != nil || len(rpRefs) != 1 {
		t.Fatalf("expected companion RP to be auto-activated, got refs: %+v", rpRefs)
	}
	if rpRefs[0].PackID != rpUUID {
		t.Errorf("expected companion RP UUID %s, got %s", rpUUID, rpRefs[0].PackID)
	}

	// Verify that texturepack-required was auto-enabled for Android clients
	cfg := GetAddonConfig(serverDir)
	if !cfg.TexturePackRequired {
		t.Errorf("expected texturepack-required to be automatically enabled")
	}

	// Verify active packs are mirrored into worlds/Bedrock level/{behavior_packs,resource_packs}
	worldBPManifest := filepath.Join(serverDir, "worlds", levelName, "behavior_packs", "Super_Dragon_BP", "manifest.json")
	if _, err := os.Stat(worldBPManifest); err != nil {
		t.Errorf("expected active BP mirrored into world folder %s: %v", worldBPManifest, err)
	}
	worldRPManifest := filepath.Join(serverDir, "worlds", levelName, "resource_packs", "Super_Dragon_RP", "manifest.json")
	if _, err := os.Stat(worldRPManifest); err != nil {
		t.Errorf("expected active RP mirrored into world folder %s: %v", worldRPManifest, err)
	}
	worldRPHistory := filepath.Join(serverDir, "worlds", levelName, "world_resource_pack_history.json")
	if _, err := os.Stat(worldRPHistory); err != nil {
		t.Errorf("expected world_resource_pack_history.json to be written: %v", err)
	}
}

func TestTolerantManifestAndForcePropertiesSync(t *testing.T) {
	serverDir := t.TempDir()

	// Create a zip with comments, trailing commas, string version, and "script" module
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	scriptManifest := "\xef\xbb\xbf" + `{
		// Comment inside manifest.json
		"format_version": 2,
		"header": {
			"name": "Scripted RPG BP",
			"description": "Uses script module and string version",
			"uuid": "12345678-1111-2222-3333-444444444444",
			"version": "1.4.2",
		},
		"modules": [
			{
				"type": "script",
				"uuid": "87654321-1111-2222-3333-444444444444",
				"version": "1.4.2",
			},
		],
	}`
	w, _ := zw.Create("manifest.json")
	_, _ = w.Write([]byte(scriptManifest))
	_ = zw.Close()

	packs, err := InstallAllPacks(serverDir, bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("InstallAllPacks with comments/script/string-version failed: %v", err)
	}
	if len(packs) != 1 {
		t.Fatalf("expected 1 pack, got %d", len(packs))
	}
	if packs[0].Type != "behavior" {
		t.Errorf("expected script module pack to be detected as behavior, got %s", packs[0].Type)
	}
	if packs[0].Version != "1.4.2" {
		t.Errorf("expected version 1.4.2, got %s", packs[0].Version)
	}

	// Test UpdateAddonConfig(texturepack-required=true) auto-activates installed packs when none are active
	if err := UpdateAddonConfig(serverDir, true); err != nil {
		t.Fatalf("UpdateAddonConfig failed: %v", err)
	}
	listed, err := ListInstalledPacks(serverDir)
	if err != nil || len(listed) != 1 || !listed[0].Active {
		t.Fatalf("expected installed pack to be auto-activated when enabling texturepack-required, got %+v", listed)
	}
}

func TestInstallWorldArchiveDirect(t *testing.T) {
	serverDir := t.TempDir()

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	w1, _ := zw.Create("level.dat")
	_, _ = w1.Write([]byte("fake-level-data"))
	w2, _ := zw.Create("levelname.txt")
	_, _ = w2.Write([]byte("Castle Siege\n"))
	w3, _ := zw.Create("db/CURRENT")
	_, _ = w3.Write([]byte("fake-db-current"))
	w4, _ := zw.Create("world_icon.jpeg")
	_, _ = w4.Write([]byte("fake-icon"))
	_ = zw.Close()

	reader := bytes.NewReader(buf.Bytes())
	pack, err := InstallPack(serverDir, reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("InstallPack for world failed: %v", err)
	}

	if pack.Type != "world" {
		t.Fatalf("expected pack type 'world', got %s", pack.Type)
	}
	if pack.Name != "Castle Siege" {
		t.Fatalf("expected world name 'Castle Siege', got %s", pack.Name)
	}

	// Verify world files exist under worlds/Castle Siege
	targetDir := filepath.Join(serverDir, "worlds", "Castle Siege")
	if _, err := os.Stat(filepath.Join(targetDir, "level.dat")); err != nil {
		t.Fatalf("expected level.dat in world directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "db", "CURRENT")); err != nil {
		t.Fatalf("expected db/CURRENT in world directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "world_icon.jpeg")); err != nil {
		t.Fatalf("expected world_icon.jpeg in world directory: %v", err)
	}

	// Verify active level name in server properties is updated
	activeLevel := GetActiveLevelName(serverDir)
	if activeLevel != "Castle Siege" {
		t.Fatalf("expected active level name 'Castle Siege', got '%s'", activeLevel)
	}
}

func TestInstallWorldArchiveWithBundledPacks(t *testing.T) {
	serverDir := t.TempDir()

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	w1, _ := zw.Create("level.dat")
	_, _ = w1.Write([]byte("fake-level-data"))
	w2, _ := zw.Create("levelname.txt")
	_, _ = w2.Write([]byte("Parkour World\n"))
	w3, _ := zw.Create("db/CURRENT")
	_, _ = w3.Write([]byte("fake-db-current"))

	bpManifest := `{
		"format_version": 2,
		"header": {
			"name": "Parkour BP",
			"description": "Parkour custom scripts",
			"uuid": "aaaaaaaa-1111-2222-3333-444444444444",
			"version": [1, 0, 0]
		},
		"modules": [{"type": "data", "uuid": "bbbbbbbb-1111-2222-3333-444444444444"}]
	}`
	w4, _ := zw.Create("behavior_packs/parkour_bp/manifest.json")
	_, _ = w4.Write([]byte(bpManifest))

	rpManifest := `{
		"format_version": 2,
		"header": {
			"name": "Parkour RP",
			"description": "Parkour custom textures",
			"uuid": "cccccccc-1111-2222-3333-444444444444",
			"version": [1, 0, 0]
		},
		"modules": [{"type": "resources", "uuid": "dddddddd-1111-2222-3333-444444444444"}]
	}`
	w5, _ := zw.Create("resource_packs/parkour_rp/manifest.json")
	_, _ = w5.Write([]byte(rpManifest))
	_ = zw.Close()

	reader := bytes.NewReader(buf.Bytes())
	packs, err := InstallAllPacks(serverDir, reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("InstallAllPacks with bundled packs failed: %v", err)
	}

	if len(packs) < 3 {
		t.Fatalf("expected at least 3 installed packs (world, BP, RP), got %d: %+v", len(packs), packs)
	}

	if packs[0].Type != "world" || packs[0].Name != "Parkour World" {
		t.Errorf("expected first pack to be world 'Parkour World', got %+v", packs[0])
	}

	// Verify BP is installed to <serverDir>/behavior_packs with .bsm_installed
	bpServerDir := filepath.Join(serverDir, "behavior_packs", "Parkour_BP")
	if _, err := os.Stat(filepath.Join(bpServerDir, bsmInstalledMarker)); err != nil {
		t.Fatalf("expected .bsm_installed marker in server behavior_packs: %v", err)
	}

	// Verify RP is installed to <serverDir>/resource_packs with .bsm_installed
	rpServerDir := filepath.Join(serverDir, "resource_packs", "Parkour_RP")
	if _, err := os.Stat(filepath.Join(rpServerDir, bsmInstalledMarker)); err != nil {
		t.Fatalf("expected .bsm_installed marker in server resource_packs: %v", err)
	}

	// Verify active refs in worlds/Parkour World
	bpRefs, err := GetWorldPackRefs(serverDir, "Parkour World", "behavior")
	if err != nil || len(bpRefs) != 1 || bpRefs[0].PackID != "aaaaaaaa-1111-2222-3333-444444444444" {
		t.Fatalf("expected BP ref in world_behavior_packs.json: %+v, err: %v", bpRefs, err)
	}
	rpRefs, err := GetWorldPackRefs(serverDir, "Parkour World", "resource")
	if err != nil || len(rpRefs) != 1 || rpRefs[0].PackID != "cccccccc-1111-2222-3333-444444444444" {
		t.Fatalf("expected RP ref in world_resource_packs.json: %+v, err: %v", rpRefs, err)
	}

	// Verify texturepack-required was enabled
	cfg := GetAddonConfig(serverDir)
	if !cfg.TexturePackRequired {
		t.Errorf("expected texturepack_required to be true when map contains resource packs")
	}
	if cfg.ActiveWorld != "Parkour World" {
		t.Errorf("expected active world to be 'Parkour World', got '%s'", cfg.ActiveWorld)
	}
}

func TestInstallWorldArchiveNestedMCWorld(t *testing.T) {
	serverDir := t.TempDir()

	// 1. Create inner .mcworld
	innerBuf := new(bytes.Buffer)
	innerZw := zip.NewWriter(innerBuf)
	iw1, _ := innerZw.Create("level.dat")
	_, _ = iw1.Write([]byte("fake-inner-level"))
	iw2, _ := innerZw.Create("levelname.txt")
	_, _ = iw2.Write([]byte("Skyblock Extreme\n"))
	iw3, _ := innerZw.Create("db/CURRENT")
	_, _ = iw3.Write([]byte("fake-db"))
	_ = innerZw.Close()

	// 2. Wrap inside outer .zip
	outerBuf := new(bytes.Buffer)
	outerZw := zip.NewWriter(outerBuf)
	ow, _ := outerZw.Create("skyblock_extreme.mcworld")
	_, _ = ow.Write(innerBuf.Bytes())
	_ = outerZw.Close()

	reader := bytes.NewReader(outerBuf.Bytes())
	pack, err := InstallPack(serverDir, reader, int64(outerBuf.Len()))
	if err != nil {
		t.Fatalf("InstallPack for nested .mcworld failed: %v", err)
	}

	if pack.Type != "world" {
		t.Fatalf("expected pack type 'world', got %s", pack.Type)
	}
	if pack.Name != "Skyblock Extreme" {
		t.Fatalf("expected world name 'Skyblock Extreme', got %s", pack.Name)
	}

	// Verify extracted to disk
	levelPath := filepath.Join(serverDir, "worlds", "Skyblock Extreme", "level.dat")
	if _, err := os.Stat(levelPath); err != nil {
		t.Fatalf("expected level.dat at worlds/Skyblock Extreme: %v", err)
	}
}

func TestInstallWorldArchiveTemplate(t *testing.T) {
	serverDir := t.TempDir()

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	templateManifest := `{
		"format_version": 2,
		"header": {
			"name": "Dungeon Crawler",
			"description": "World template adventure",
			"uuid": "99999999-8888-7777-6666-555555555555",
			"version": [1, 0, 0]
		},
		"modules": [
			{
				"type": "world_template",
				"uuid": "11112222-3333-4444-5555-666677778888",
				"version": [1, 0, 0]
			}
		]
	}`
	w1, _ := zw.Create("manifest.json")
	_, _ = w1.Write([]byte(templateManifest))
	w2, _ := zw.Create("level.dat")
	_, _ = w2.Write([]byte("fake-template-level"))
	w3, _ := zw.Create("db/CURRENT")
	_, _ = w3.Write([]byte("fake-template-db"))
	_ = zw.Close()

	reader := bytes.NewReader(buf.Bytes())
	pack, err := InstallPack(serverDir, reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("InstallPack world template failed: %v", err)
	}

	if pack.Type != "world" {
		t.Fatalf("expected world_template to be installed as 'world', got '%s'", pack.Type)
	}
	if pack.Name != "Dungeon Crawler" {
		t.Fatalf("expected name 'Dungeon Crawler', got '%s'", pack.Name)
	}

	// Ensure NOT placed in resource_packs/
	rpDir := filepath.Join(serverDir, "resource_packs", "Dungeon_Crawler")
	if _, err := os.Stat(rpDir); err == nil {
		t.Fatalf("world_template should NOT be extracted to resource_packs")
	}

	// Verify placed in worlds/Dungeon Crawler
	worldLevel := filepath.Join(serverDir, "worlds", "Dungeon Crawler", "level.dat")
	if _, err := os.Stat(worldLevel); err != nil {
		t.Fatalf("expected level.dat in worlds/Dungeon Crawler: %v", err)
	}
}

func TestInstallWorldArchiveTraversal(t *testing.T) {
	serverDir := t.TempDir()

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	w1, _ := zw.Create("../../etc/passwd")
	_, _ = w1.Write([]byte("root:x:0:0"))
	w2, _ := zw.Create("level.dat")
	_, _ = w2.Write([]byte("fake-level"))
	_ = zw.Close()

	reader := bytes.NewReader(buf.Bytes())
	_, _ = InstallPack(serverDir, reader, int64(buf.Len()))

	// Ensure file was not written outside worlds
	badFile := filepath.Join(serverDir, "..", "etc", "passwd")
	if _, err := os.Stat(badFile); err == nil {
		t.Fatalf("path traversal file was written: %s", badFile)
	}
}


