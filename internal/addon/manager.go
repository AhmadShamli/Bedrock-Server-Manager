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

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/configfile"
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

// WorldPackRef represents an entry in world_behavior_packs.json or world_resource_packs.json.
type WorldPackRef struct {
	PackID  string `json:"pack_id"`
	Version []int  `json:"version"`
}

// InstalledPack describes an installed addon pack.
type InstalledPack struct {
	Type        string `json:"type"` // "behavior" or "resource"
	Folder      string `json:"folder"`
	Name        string `json:"name"`
	Description string `json:"description"`
	UUID        string `json:"uuid"`
	Version     string `json:"version"`
	VersionInts []int  `json:"version_ints,omitempty"`
	Active      bool   `json:"active"`
}

// AddonConfig contains related addon/pack configurations for a server.
type AddonConfig struct {
	ActiveWorld         string `json:"active_world"`
	TexturePackRequired bool   `json:"texturepack_required"`
}

// GetActiveLevelName determines the active world folder name from server.properties or defaults to "Bedrock level".
func GetActiveLevelName(serverDir string) string {
	propPath := filepath.Join(serverDir, "server.properties")
	if _, err := os.Stat(propPath); err == nil {
		props, _, _ := configfile.ReadProperties(propPath)
		if val, ok := props["level-name"]; ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}

	pendingProps, _, ok, _ := configfile.ReadPendingProperties(serverDir)
	if ok {
		if val, ok := pendingProps["level-name"]; ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}

	return "Bedrock level"
}

// GetWorldPackPath returns the path to world_behavior_packs.json or world_resource_packs.json.
func GetWorldPackPath(serverDir, levelName, packType string) string {
	return filepath.Join(serverDir, "worlds", levelName, fmt.Sprintf("world_%s_packs.json", packType))
}

// GetWorldPackRefs reads the active pack list for a world and pack type.
func GetWorldPackRefs(serverDir, levelName, packType string) ([]WorldPackRef, error) {
	filePath := GetWorldPackPath(serverDir, levelName, packType)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []WorldPackRef{}, nil
		}
		return nil, err
	}

	var refs []WorldPackRef
	if err := json.Unmarshal(data, &refs); err != nil {
		return []WorldPackRef{}, nil
	}
	return refs, nil
}

// SaveWorldPackRefs writes the active pack list for a world and pack type.
func SaveWorldPackRefs(serverDir, levelName, packType string, refs []WorldPackRef) error {
	worldDir := filepath.Join(serverDir, "worlds", levelName)
	if err := os.MkdirAll(worldDir, 0755); err != nil {
		return fmt.Errorf("failed to create world directory: %w", err)
	}

	filePath := GetWorldPackPath(serverDir, levelName, packType)
	data, err := json.MarshalIndent(refs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode pack refs: %w", err)
	}

	return os.WriteFile(filePath, append(data, '\n'), 0644)
}

// ListInstalledPacks inspects behavior_packs and resource_packs folders for a server,
// identifying whether each pack is active in the currently active world.
func ListInstalledPacks(serverDir string) ([]InstalledPack, error) {
	levelName := GetActiveLevelName(serverDir)
	activeBehaviorRefs, _ := GetWorldPackRefs(serverDir, levelName, "behavior")
	activeResourceRefs, _ := GetWorldPackRefs(serverDir, levelName, "resource")

	activeBehaviorUUIDs := make(map[string]bool)
	for _, r := range activeBehaviorRefs {
		activeBehaviorUUIDs[strings.ToLower(r.PackID)] = true
	}
	activeResourceUUIDs := make(map[string]bool)
	for _, r := range activeResourceRefs {
		activeResourceUUIDs[strings.ToLower(r.PackID)] = true
	}

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
				verInts := mf.Header.Version
				if len(verInts) >= 3 {
					verStr = fmt.Sprintf("%d.%d.%d", verInts[0], verInts[1], verInts[2])
				}
				name := mf.Header.Name
				if name == "" {
					name = entry.Name()
				}

				isActive := false
				if packType == "behavior" {
					isActive = activeBehaviorUUIDs[strings.ToLower(mf.Header.UUID)]
				} else {
					isActive = activeResourceUUIDs[strings.ToLower(mf.Header.UUID)]
				}

				packs = append(packs, InstalledPack{
					Type:        packType,
					Folder:      entry.Name(),
					Name:        name,
					Description: mf.Header.Description,
					UUID:        mf.Header.UUID,
					Version:     verStr,
					VersionInts: verInts,
					Active:      isActive,
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

// DeletePack removes an installed pack folder and deactivates it from world pack files.
func DeletePack(serverDir, packType, folder string) error {
	if packType != "behavior" && packType != "resource" {
		return fmt.Errorf("invalid pack type: must be 'behavior' or 'resource'")
	}

	cleanFolder := filepath.Clean(folder)
	if cleanFolder == "." || cleanFolder == ".." || strings.Contains(cleanFolder, "/") || strings.Contains(cleanFolder, "\\") {
		return fmt.Errorf("invalid folder name: directory traversal attempt")
	}

	// Deactivate from world pack configuration first
	_, _ = SetPackActive(serverDir, packType, cleanFolder, false)

	baseDir := filepath.Join(serverDir, packType+"_packs")
	targetDir := filepath.Join(baseDir, cleanFolder)
	baseDirClean := filepath.Clean(baseDir) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(targetDir), baseDirClean) {
		return fmt.Errorf("path traversal attempt detected")
	}

	return os.RemoveAll(targetDir)
}

// SetPackActive activates or deactivates an installed pack in the active world's configuration file.
func SetPackActive(serverDir, packType, folder string, active bool) (*InstalledPack, error) {
	if packType != "behavior" && packType != "resource" {
		return nil, fmt.Errorf("invalid pack type: must be 'behavior' or 'resource'")
	}

	cleanFolder := filepath.Clean(folder)
	if cleanFolder == "." || cleanFolder == ".." || strings.Contains(cleanFolder, "/") || strings.Contains(cleanFolder, "\\") {
		return nil, fmt.Errorf("invalid folder name: directory traversal attempt")
	}

	manifestPath := filepath.Join(serverDir, packType+"_packs", cleanFolder, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pack manifest: %w", err)
	}

	var mf PackManifest
	if err := json.Unmarshal(data, &mf); err != nil {
		return nil, fmt.Errorf("invalid pack manifest: %w", err)
	}

	packUUID := mf.Header.UUID
	if packUUID == "" {
		return nil, fmt.Errorf("pack manifest missing header.uuid")
	}

	verInts := mf.Header.Version
	if len(verInts) == 0 {
		verInts = []int{1, 0, 0}
	} else if len(verInts) < 3 {
		for len(verInts) < 3 {
			verInts = append(verInts, 0)
		}
	}

	levelName := GetActiveLevelName(serverDir)
	refs, err := GetWorldPackRefs(serverDir, levelName, packType)
	if err != nil {
		return nil, err
	}

	newRefs := make([]WorldPackRef, 0, len(refs)+1)
	found := false
	for _, r := range refs {
		if strings.EqualFold(r.PackID, packUUID) {
			found = true
			if active {
				// Keep with current/updated version
				newRefs = append(newRefs, WorldPackRef{
					PackID:  packUUID,
					Version: verInts,
				})
			}
			// If not active, skip (deactivate)
		} else {
			newRefs = append(newRefs, r)
		}
	}

	if active && !found {
		newRefs = append(newRefs, WorldPackRef{
			PackID:  packUUID,
			Version: verInts,
		})
	}

	if err := SaveWorldPackRefs(serverDir, levelName, packType, newRefs); err != nil {
		return nil, err
	}

	verStr := fmt.Sprintf("%d.%d.%d", verInts[0], verInts[1], verInts[2])
	name := mf.Header.Name
	if name == "" {
		name = cleanFolder
	}

	return &InstalledPack{
		Type:        packType,
		Folder:      cleanFolder,
		Name:        name,
		Description: mf.Header.Description,
		UUID:        packUUID,
		Version:     verStr,
		VersionInts: verInts,
		Active:      active,
	}, nil
}

// SetPackOrder reorders the active pack priority list for a world.
func SetPackOrder(serverDir, packType string, orderedUUIDs []string) error {
	if packType != "behavior" && packType != "resource" {
		return fmt.Errorf("invalid pack type: must be 'behavior' or 'resource'")
	}

	levelName := GetActiveLevelName(serverDir)
	existingRefs, err := GetWorldPackRefs(serverDir, levelName, packType)
	if err != nil {
		return err
	}

	refMap := make(map[string]WorldPackRef)
	for _, r := range existingRefs {
		refMap[strings.ToLower(r.PackID)] = r
	}

	newRefs := make([]WorldPackRef, 0, len(existingRefs))
	seen := make(map[string]bool)

	// Append in the requested order
	for _, uuid := range orderedUUIDs {
		lower := strings.ToLower(uuid)
		if ref, ok := refMap[lower]; ok && !seen[lower] {
			newRefs = append(newRefs, ref)
			seen[lower] = true
		}
	}

	// Append any remaining active packs that were not explicitly ordered
	for _, r := range existingRefs {
		lower := strings.ToLower(r.PackID)
		if !seen[lower] {
			newRefs = append(newRefs, r)
			seen[lower] = true
		}
	}

	return SaveWorldPackRefs(serverDir, levelName, packType, newRefs)
}

// GetAddonConfig reads addon-related settings for the server.
func GetAddonConfig(serverDir string) AddonConfig {
	activeWorld := GetActiveLevelName(serverDir)
	textureReq := false

	propPath := filepath.Join(serverDir, "server.properties")
	if _, err := os.Stat(propPath); err == nil {
		props, _, _ := configfile.ReadProperties(propPath)
		if val, ok := props["texturepack-required"]; ok {
			textureReq = strings.EqualFold(strings.TrimSpace(val), "true")
		}
	} else {
		pendingProps, _, ok, _ := configfile.ReadPendingProperties(serverDir)
		if ok {
			if val, ok := pendingProps["texturepack-required"]; ok {
				textureReq = strings.EqualFold(strings.TrimSpace(val), "true")
			}
		}
	}

	return AddonConfig{
		ActiveWorld:         activeWorld,
		TexturePackRequired: textureReq,
	}
}

// UpdateAddonConfig updates addon-related settings in server.properties or pending properties.
func UpdateAddonConfig(serverDir string, texturePackRequired bool) error {
	valStr := "false"
	if texturePackRequired {
		valStr = "true"
	}
	updates := map[string]string{
		"texturepack-required": valStr,
	}

	propPath := filepath.Join(serverDir, "server.properties")
	updated, err := configfile.UpdateExistingPropertyFile(propPath, updates)
	if err != nil {
		return fmt.Errorf("failed to update server.properties: %w", err)
	}

	if !updated {
		// Server not booted yet, queue into pending properties
		existingProps, existingKeys, ok, err := configfile.ReadPendingProperties(serverDir)
		if err != nil {
			return err
		}
		if !ok || existingProps == nil {
			existingProps = make(map[string]string)
			existingKeys = []string{}
		}
		mergedProps, mergedKeys := configfile.MergeProperties(existingProps, existingKeys, updates)
		if err := configfile.WritePendingProperties(serverDir, mergedProps, mergedKeys); err != nil {
			return fmt.Errorf("failed to save pending properties: %w", err)
		}
	}

	return nil
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
