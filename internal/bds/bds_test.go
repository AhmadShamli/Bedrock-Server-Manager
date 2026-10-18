package bds

import (
	"archive/zip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type testSettingStore struct {
	settings map[string]string
}

func (s *testSettingStore) GetSetting(ctx context.Context, key string) (string, error) {
	if val, ok := s.settings[key]; ok {
		return val, nil
	}
	return "", fmt.Errorf("key not found")
}

func (s *testSettingStore) SetSetting(ctx context.Context, key, value string) error {
	s.settings[key] = value
	return nil
}

func TestEULAFlow(t *testing.T) {
	store := &testSettingStore{settings: make(map[string]string)}
	ctx := context.Background()

	accepted, err := IsEULAAccepted(ctx, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accepted {
		t.Fatalf("expected EULA not accepted initially")
	}

	if err := AcceptEULA(ctx, store, "admin"); err != nil {
		t.Fatalf("failed to accept EULA: %v", err)
	}

	accepted, err = IsEULAAccepted(ctx, store)
	if err != nil || !accepted {
		t.Fatalf("expected EULA accepted, got %v (err: %v)", accepted, err)
	}
	if store.settings[SettingEULAAcceptedBy] != "admin" {
		t.Fatalf("expected accepted by admin, got %s", store.settings[SettingEULAAcceptedBy])
	}
}

func createTestZip(t *testing.T, zipPath string, files map[string]string) {
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create test zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range files {
		wf, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := wf.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip content: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
}

func TestCacheManagerAndInstaller(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a dummy zip file
	dummyZip := filepath.Join(tmpDir, "server.zip")
	createTestZip(t, dummyZip, map[string]string{
		"bedrock_server.exe":            "binary content",
		"server.properties":             "server-name=Default Server\n",
		"definitions/behavior/test.json": "{}",
		"resource_packs/pack1/manifest.json": "{}",
	})

	zipBytes, err := os.ReadFile(dummyZip)
	if err != nil {
		t.Fatalf("failed to read test zip: %v", err)
	}

	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipBytes)
	}))
	defer server.Close()

	cm := NewCacheManager(tmpDir, server.Client())

	ctx := context.Background()
	ver := &ResolvedVersion{
		Version:     "1.21.50.07",
		DownloadURL: server.URL,
	}

	// 1. Ensure without EULA should fail
	_, err = cm.EnsureVersion(ctx, ver, false)
	if err != ErrEULANotAccepted {
		t.Fatalf("expected ErrEULANotAccepted, got %v", err)
	}

	// 2. Ensure with EULA should download and extract
	vDir, err := cm.EnsureVersion(ctx, ver, true)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !cm.IsVersionCached("1.21.50.07") {
		t.Fatalf("expected version to be cached")
	}

	// Verify files exist in cache
	if _, err := os.Stat(filepath.Join(vDir, "bedrock_server.exe")); err != nil {
		t.Fatalf("expected bedrock_server.exe in cache: %v", err)
	}

	// 3. Install into a server dir
	srvDir := filepath.Join(tmpDir, "server1")
	if err := InstallServerFiles(srvDir, vDir, false); err != nil {
		t.Fatalf("failed to install server files: %v", err)
	}

	// Mutate server.properties to simulate user edit
	customProps := filepath.Join(srvDir, "server.properties")
	_ = os.WriteFile(customProps, []byte("server-name=My Custom Realm\n"), 0640)

	// Create custom world
	worldDir := filepath.Join(srvDir, "worlds", "Bedrock level")
	_ = os.MkdirAll(worldDir, 0750)
	_ = os.WriteFile(filepath.Join(worldDir, "level.dat"), []byte("worlddata"), 0640)

	// 4. Overlay update: user configs and worlds must be preserved
	if err := InstallServerFiles(srvDir, vDir, true); err != nil {
		t.Fatalf("failed to overlay update server files: %v", err)
	}

	content, err := os.ReadFile(customProps)
	if err != nil || string(content) != "server-name=My Custom Realm\n" {
		t.Fatalf("expected custom server.properties preserved, got %s", string(content))
	}
	if _, err := os.Stat(filepath.Join(worldDir, "level.dat")); err != nil {
		t.Fatalf("expected world level.dat preserved: %v", err)
	}

	// 5. Test pruning
	// Create another fake version
	v2Dir := filepath.Join(tmpDir, "bds", "1.20.0.01")
	_ = os.MkdirAll(v2Dir, 0750)
	_ = os.WriteFile(filepath.Join(v2Dir, ".complete"), []byte("ok"), 0640)

	// Prune keeping 1 version, protecting 1.21.50.07
	if err := cm.PruneCache(1, []string{"1.21.50.07"}); err != nil {
		t.Fatalf("prune failed: %v", err)
	}

	// 6. Test PurgeUnused
	if err := cm.PurgeUnused([]string{"1.21.50.07"}); err != nil {
		t.Fatalf("purge unused failed: %v", err)
	}
	if !cm.IsVersionCached("1.21.50.07") {
		t.Fatalf("expected 1.21.50.07 to remain cached")
	}
}

func TestResolverWithMockServiceAPI(t *testing.T) {
	mockResponse := `{
		"result": {
			"links": [
				{
					"downloadType": "serverBedrockWindows",
					"downloadUrl": "https://www.minecraft.net/bedrockdedicatedserver/bin-win/bedrock-server-1.26.52.3.zip"
				},
				{
					"downloadType": "serverBedrockPreviewWindows",
					"downloadUrl": "https://www.minecraft.net/bedrockdedicatedserver/bin-win-preview/bedrock-server-1.26.70.20.zip"
				}
			]
		}
	}`

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer mockServer.Close()

	resolver := NewResolver(mockServer.Client())
	ctx := context.Background()

	// Direct call to fetchFromServiceAPI
	relVer, prevVer, err := resolver.fetchFromServiceAPI(ctx, mockServer.URL)
	if err != nil {
		t.Fatalf("fetchFromServiceAPI failed: %v", err)
	}
	if relVer == nil || relVer.Version != "1.26.52.3" {
		t.Fatalf("expected release version 1.26.52.3, got %+v", relVer)
	}
	if prevVer == nil || prevVer.Version != "1.26.70.20" {
		t.Fatalf("expected preview version 1.26.70.20, got %+v", prevVer)
	}
	if !prevVer.IsPreview {
		t.Fatalf("expected preview version to have IsPreview=true")
	}
}

func TestResolverPinnedVersions(t *testing.T) {
	resolver := NewResolver(nil)
	ctx := context.Background()

	// Pinned release version
	res, err := resolver.ResolveVersion(ctx, "1.26.52.3", false)
	if err != nil {
		t.Fatalf("failed to resolve pinned version: %v", err)
	}
	if res.Version != "1.26.52.3" || res.IsPreview {
		t.Fatalf("unexpected pinned result: %+v", res)
	}
	expectedURL := "https://www.minecraft.net/bedrockdedicatedserver/bin-win/bedrock-server-1.26.52.3.zip"
	if res.DownloadURL != expectedURL {
		t.Fatalf("expected download URL %s, got %s", expectedURL, res.DownloadURL)
	}

	// Pinned preview version
	prevRes, err := resolver.ResolveVersion(ctx, "preview:1.26.70.20", false)
	if err != nil {
		t.Fatalf("failed to resolve pinned preview: %v", err)
	}
	if prevRes.Version != "1.26.70.20" || !prevRes.IsPreview {
		t.Fatalf("unexpected pinned preview result: %+v", prevRes)
	}
	expectedPrevURL := "https://www.minecraft.net/bedrockdedicatedserver/bin-win-preview/bedrock-server-1.26.70.20.zip"
	if prevRes.DownloadURL != expectedPrevURL {
		t.Fatalf("expected preview URL %s, got %s", expectedPrevURL, prevRes.DownloadURL)
	}
}

func TestResolverLiveEndpoint(t *testing.T) {
	resolver := NewResolver(nil)
	ctx := context.Background()

	// Resolve latest
	rel, err := resolver.ResolveVersion(ctx, "latest", true)
	if err != nil {
		t.Fatalf("failed to resolve latest live: %v", err)
	}
	if rel == nil || rel.Version == "" || rel.DownloadURL == "" {
		t.Fatalf("invalid live release resolution: %+v", rel)
	}
	t.Logf("Live Latest BDS: version=%s, url=%s", rel.Version, rel.DownloadURL)

	// Resolve preview
	prev, err := resolver.ResolveVersion(ctx, "preview", false)
	if err != nil {
		t.Fatalf("failed to resolve preview live: %v", err)
	}
	if prev == nil || prev.Version == "" || prev.DownloadURL == "" || !prev.IsPreview {
		t.Fatalf("invalid live preview resolution: %+v", prev)
	}
	t.Logf("Live Preview BDS: version=%s, url=%s", prev.Version, prev.DownloadURL)
}
