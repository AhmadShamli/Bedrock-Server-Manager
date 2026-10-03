package addon

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// bsmInstalledMarker is a hidden file placed inside packs installed through BSM.
// This distinguishes user-installed addons from the many built-in BDS packs
// (vanilla, chemistry, experimental_*, etc.) that ship in behavior_packs/ and
// resource_packs/ by default.
const bsmInstalledMarker = ".bsm_installed"

// PackManifest represents the minimal Bedrock pack manifest.json.
type PackManifest struct {
	FormatVersion int `json:"format_version"`
	Header        struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		UUID        string `json:"uuid"`
		Version     []int  `json:"version"`
	} `json:"header"`
	Modules []struct {
		Type string `json:"type"`
		UUID string `json:"uuid"`
	} `json:"modules"`
}

// InstalledPack describes an installed addon pack.
type InstalledPack struct {
	Type        string `json:"type"` // "behavior" or "resource"
	Folder      string `json:"folder"`
	Name        string `json:"name"`
	Description string `json:"description"`
	UUID        string `json:"uuid"`
	Version     string `json:"version"`
}

// ListInstalledPacks inspects behavior_packs and resource_packs folders for a server.
func ListInstalledPacks(serverDir string) ([]InstalledPack, error) {
	packs := make([]InstalledPack, 0)

	scanDir := func(dirName, packType string) {
		fullPath := filepath.Join(serverDir, dirName)
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			packDir := filepath.Join(fullPath, entry.Name())

			// Only list packs installed through BSM (skip built-in BDS packs)
			if _, err := os.Stat(filepath.Join(packDir, bsmInstalledMarker)); err != nil {
				continue
			}

			manifestPath := filepath.Join(packDir, "manifest.json")
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				continue
			}

			var mf PackManifest
			if err := json.Unmarshal(data, &mf); err == nil {
				verStr := "1.0.0"
				if len(mf.Header.Version) >= 3 {
					verStr = fmt.Sprintf("%d.%d.%d", mf.Header.Version[0], mf.Header.Version[1], mf.Header.Version[2])
				}
				name := mf.Header.Name
				if name == "" {
					name = entry.Name()
				}
				packs = append(packs, InstalledPack{
					Type:        packType,
					Folder:      entry.Name(),
					Name:        name,
					Description: mf.Header.Description,
					UUID:        mf.Header.UUID,
					Version:     verStr,
				})
			}
		}
	}

	scanDir("behavior_packs", "behavior")
	scanDir("resource_packs", "resource")

	return packs, nil
}

// InstallPack unzips a .mcpack or .zip into the appropriate pack directory.
func InstallPack(serverDir string, r io.ReaderAt, size int64) (*InstalledPack, error) {
	zipReader, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("invalid archive: %w", err)
	}

	// 1. Find and parse manifest.json to identify pack type and identity
	var manifestData []byte
	var manifestPrefix string
	for _, f := range zipReader.File {
		if strings.EqualFold(filepath.Base(f.Name), "manifest.json") {
			rc, err := f.Open()
			if err == nil {
				manifestData, _ = io.ReadAll(rc)
				rc.Close()
				cleaned := filepath.Clean(f.Name)
				dir := filepath.Dir(cleaned)
				if dir != "." && dir != "/" && dir != "" {
					manifestPrefix = dir + "/"
				}
				break
			}
		}
	}

	if len(manifestData) == 0 {
		var firstInstalled *InstalledPack
		for _, f := range zipReader.File {
			lowerName := strings.ToLower(f.Name)
			if strings.HasSuffix(lowerName, ".mcpack") || strings.HasSuffix(lowerName, ".zip") {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				packBytes, err := io.ReadAll(rc)
				rc.Close()
				if err != nil || len(packBytes) == 0 {
					continue
				}
				p, err := InstallPack(serverDir, bytes.NewReader(packBytes), int64(len(packBytes)))
				if err == nil && firstInstalled == nil {
					firstInstalled = p
				}
			}
		}
		if firstInstalled != nil {
			return firstInstalled, nil
		}
		return nil, fmt.Errorf("archive does not contain a valid manifest.json or .mcpack files")
	}

	var mf PackManifest
	if err := json.Unmarshal(manifestData, &mf); err != nil {
		return nil, fmt.Errorf("malformed manifest.json: %w", err)
	}

	packType := "resource"
	for _, m := range mf.Modules {
		if m.Type == "data" {
			packType = "behavior"
			break
		}
	}

	folderName := mf.Header.Name
	if folderName == "" {
		folderName = mf.Header.UUID
	}
	// Sanitize folder name
	folderName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, folderName)

	var targetDir string
	if packType == "behavior" {
		targetDir = filepath.Join(serverDir, "behavior_packs", folderName)
	} else {
		targetDir = filepath.Join(serverDir, "resource_packs", folderName)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, err
	}

	// 2. Extract contents
	for _, f := range zipReader.File {
		cleaned := filepath.Clean(f.Name)
		if strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") {
			continue
		}

		// Strip enclosing subfolder prefix if present so manifest.json is at targetDir root
		if manifestPrefix != "" {
			if strings.HasPrefix(f.Name, manifestPrefix) {
				cleaned = filepath.Clean(strings.TrimPrefix(f.Name, manifestPrefix))
			} else {
				continue
			}
		}
		if cleaned == "" || cleaned == "." {
			continue
		}

		destPath := filepath.Join(targetDir, cleaned)
		targetDirClean := filepath.Clean(targetDir) + string(filepath.Separator)
		if !strings.HasPrefix(filepath.Clean(destPath), targetDirClean) {
			continue // Zip Slip path traversal attempt blocked
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(destPath, f.Mode())
			continue
		}

		_ = os.MkdirAll(filepath.Dir(destPath), 0755)
		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			continue
		}
		_, _ = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
	}

	// Write marker file so ListInstalledPacks knows this is a user-installed pack
	_ = os.WriteFile(filepath.Join(targetDir, bsmInstalledMarker), []byte("installed-by-bsm\n"), 0644)

	verStr := "1.0.0"
	if len(mf.Header.Version) >= 3 {
		verStr = fmt.Sprintf("%d.%d.%d", mf.Header.Version[0], mf.Header.Version[1], mf.Header.Version[2])
	}

	return &InstalledPack{
		Type:        packType,
		Folder:      folderName,
		Name:        mf.Header.Name,
		Description: mf.Header.Description,
		UUID:        mf.Header.UUID,
		Version:     verStr,
	}, nil
}

// DeletePack removes an installed pack folder.
func DeletePack(serverDir, packType, folder string) error {
	if packType != "behavior" && packType != "resource" {
		return fmt.Errorf("invalid pack type: must be 'behavior' or 'resource'")
	}

	cleanFolder := filepath.Clean(folder)
	if cleanFolder == "." || cleanFolder == ".." || strings.Contains(cleanFolder, "/") || strings.Contains(cleanFolder, "\\") {
		return fmt.Errorf("invalid folder name: directory traversal attempt")
	}

	baseDir := filepath.Join(serverDir, packType+"_packs")
	targetDir := filepath.Join(baseDir, cleanFolder)
	baseDirClean := filepath.Clean(baseDir) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(targetDir), baseDirClean) {
		return fmt.Errorf("path traversal attempt detected")
	}

	return os.RemoveAll(targetDir)
}

// InstallFromURL downloads a pack archive from a direct URL and installs it.
func InstallFromURL(ctx context.Context, serverDir, downloadURL string) (*InstalledPack, error) {
	parsed, err := url.Parse(downloadURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid URL: must be http or https")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Bedrock-Server-Manager/1.0")

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server responded with status %d", resp.StatusCode)
	}

	tempFile, err := os.CreateTemp("", "pack_url_*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Max 100MB limit
	size, err := io.Copy(tempFile, io.LimitReader(resp.Body, 100<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read download: %w", err)
	}
	if size == 0 {
		return nil, fmt.Errorf("downloaded file is empty")
	}

	return InstallPack(serverDir, tempFile, size)
}
