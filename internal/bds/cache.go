package bds

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// CacheManager manages cached Mojang Bedrock Dedicated Server binaries.
type CacheManager struct {
	cacheDir string
	client   *http.Client
	mu       sync.Mutex
}

// NewCacheManager initializes a BDS cache manager rooted at dataDir/bds.
func NewCacheManager(dataDir string, client *http.Client) *CacheManager {
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Minute,
		}
	}
	cacheDir := filepath.Join(dataDir, "bds")
	_ = os.MkdirAll(cacheDir, 0750)

	return &CacheManager{
		cacheDir: cacheDir,
		client:   client,
	}
}

// VersionDir returns the directory path for a cached BDS version.
func (c *CacheManager) VersionDir(version string) string {
	return filepath.Join(c.cacheDir, version)
}

// IsVersionCached returns true if the specified version is already downloaded and extracted.
func (c *CacheManager) IsVersionCached(version string) bool {
	vDir := c.VersionDir(version)
	marker := filepath.Join(vDir, ".complete")
	if _, err := os.Stat(marker); err == nil {
		return true
	}
	exePath := filepath.Join(vDir, "bedrock_server.exe")
	if _, err := os.Stat(exePath); err == nil {
		return true
	}
	return false
}

// EnsureVersion checks if the specified version is cached, and downloads/extracts it if needed.
func (c *CacheManager) EnsureVersion(ctx context.Context, ver *ResolvedVersion, eulaAccepted bool) (string, error) {
	if ver == nil || ver.Version == "" {
		return "", fmt.Errorf("invalid version definition")
	}

	targetDir := c.VersionDir(ver.Version)
	if c.IsVersionCached(ver.Version) {
		return targetDir, nil
	}

	if !eulaAccepted {
		return "", ErrEULANotAccepted
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check under lock
	if c.IsVersionCached(ver.Version) {
		return targetDir, nil
	}

	if err := os.MkdirAll(c.cacheDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create BDS cache root: %w", err)
	}

	zipTmp := filepath.Join(c.cacheDir, fmt.Sprintf("download-%s-%d.zip", ver.Version, time.Now().UnixNano()))
	defer os.Remove(zipTmp)

	log.Printf("[BDS] Downloading official Windows BDS %s from %s ...", ver.Version, ver.DownloadURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ver.DownloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", MojangUserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with HTTP %d", resp.StatusCode)
	}

	out, err := os.OpenFile(zipTmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create temp archive: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return "", fmt.Errorf("failed to save download archive: %w", err)
	}
	out.Close()

	// Extract to temporary directory
	extractTmp := filepath.Join(c.cacheDir, fmt.Sprintf("extract-%s-%d", ver.Version, time.Now().UnixNano()))
	if err := os.MkdirAll(extractTmp, 0750); err != nil {
		return "", fmt.Errorf("failed to create temp extraction dir: %w", err)
	}
	defer os.RemoveAll(extractTmp)

	if err := unzipArchive(zipTmp, extractTmp); err != nil {
		return "", fmt.Errorf("failed to extract BDS archive: %w", err)
	}

	// Write .complete marker
	markerFile := filepath.Join(extractTmp, ".complete")
	_ = os.WriteFile(markerFile, []byte(fmt.Sprintf("version=%s\ndownload_url=%s\nextracted_at=%s\n", ver.Version, ver.DownloadURL, time.Now().UTC().Format(time.RFC3339))), 0640)

	// Clean up any existing broken directory at targetDir before renaming
	_ = os.RemoveAll(targetDir)

	if err := os.Rename(extractTmp, targetDir); err != nil {
		return "", fmt.Errorf("failed to finalize BDS cache directory: %w", err)
	}

	log.Printf("[BDS] Successfully cached official BDS version %s at %s", ver.Version, targetDir)
	return targetDir, nil
}

func unzipArchive(srcZip, destDir string) error {
	r, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer r.Close()

	destClean := filepath.Clean(destDir)

	for _, f := range r.File {
		targetPath := filepath.Join(destDir, f.Name)
		targetClean := filepath.Clean(targetPath)

		// Guard against ZipSlip
		if !strings.HasPrefix(targetClean, destClean+string(filepath.Separator)) && targetClean != destClean {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetClean, 0750); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetClean), 0750); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		mode := f.Mode()
		if mode == 0 {
			mode = 0640
		}
		// Ensure binaries have executable bit
		if strings.HasSuffix(strings.ToLower(f.Name), ".exe") {
			mode = 0750
		}

		dst, err := os.OpenFile(targetClean, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			rc.Close()
			return err
		}

		_, copyErr := io.Copy(dst, rc)
		rc.Close()
		dst.Close()
		if copyErr != nil {
			return copyErr
		}
	}

	return nil
}

// GetCachedVersions returns all fully cached BDS version strings.
func (c *CacheManager) GetCachedVersions() ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	versions := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			vName := entry.Name()
			if c.IsVersionCached(vName) {
				versions = append(versions, vName)
			}
		}
	}
	return versions, nil
}

// GetCacheSize computes total disk usage and version count of the BDS cache.
func (c *CacheManager) GetCacheSize() (int64, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var totalBytes int64
	versionCount := 0

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	for _, entry := range entries {
		if entry.IsDir() && c.IsVersionCached(entry.Name()) {
			versionCount++
		}
	}

	err = filepath.Walk(c.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalBytes += info.Size()
		}
		return nil
	})

	return totalBytes, versionCount, err
}

type versionEntry struct {
	version string
	modTime time.Time
}

// PruneCache keeps the newest N versions, protecting versions currently in use or designated for rollback.
func (c *CacheManager) PruneCache(keepN int, protectedVersions []string) error {
	if keepN <= 0 {
		keepN = 3
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	protected := make(map[string]bool)
	for _, v := range protectedVersions {
		if v != "" {
			protected[v] = true
		}
	}

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return err
	}

	var candidates []versionEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		vName := entry.Name()
		if protected[vName] {
			continue // Protected version never purged
		}
		if !c.IsVersionCached(vName) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, versionEntry{
			version: vName,
			modTime: info.ModTime(),
		})
	}

	// Sort candidates newest to oldest
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	if len(candidates) > keepN {
		toDelete := candidates[keepN:]
		for _, v := range toDelete {
			targetDir := filepath.Join(c.cacheDir, v.version)
			log.Printf("[BDS] Pruning old BDS cache version %s at %s", v.version, targetDir)
			_ = os.RemoveAll(targetDir)
		}
	}

	return nil
}

// PurgeUnused deletes all cached BDS versions except those in protectedVersions.
func (c *CacheManager) PurgeUnused(protectedVersions []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	protected := make(map[string]bool)
	for _, v := range protectedVersions {
		if v != "" {
			protected[v] = true
		}
	}

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		vName := entry.Name()
		if protected[vName] {
			continue
		}
		targetDir := filepath.Join(c.cacheDir, vName)
		log.Printf("[BDS] Purging unused BDS cache version %s at %s", vName, targetDir)
		_ = os.RemoveAll(targetDir)
	}

	return nil
}
