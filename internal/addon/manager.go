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
	"strconv"
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
	Dependencies []struct {
		UUID       string      `json:"uuid"`
		ModuleName string      `json:"module_name,omitempty"`
		Version    interface{} `json:"version,omitempty"`
	} `json:"dependencies,omitempty"`
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

// parseVersionSlice converts a Bedrock manifest version ([1,0,0] or "1.0.0") into a 3-int slice.
func parseVersionSlice(raw interface{}) []int {
	switch v := raw.(type) {
	case []int:
		res := append([]int(nil), v...)
		for len(res) < 3 {
			res = append(res, 0)
		}
		return res[:3]
	case []interface{}:
		res := make([]int, 0, 3)
		for _, item := range v {
			switch n := item.(type) {
			case float64:
				res = append(res, int(n))
			case int:
				res = append(res, n)
			case string:
				iv, _ := strconv.Atoi(strings.TrimSpace(n))
				res = append(res, iv)
			}
		}
		if len(res) == 0 {
			return []int{1, 0, 0}
		}
		for len(res) < 3 {
			res = append(res, 0)
		}
		return res[:3]
	case string:
		clean := strings.TrimSpace(strings.TrimPrefix(v, "v"))
		if idx := strings.IndexAny(clean, "-+"); idx != -1 {
			clean = clean[:idx]
		}
		parts := strings.Split(clean, ".")
		res := make([]int, 0, 3)
		for _, p := range parts {
			iv, err := strconv.Atoi(strings.TrimSpace(p))
			if err != nil {
				iv = 0
			}
			res = append(res, iv)
		}
		if len(res) == 0 {
			return []int{1, 0, 0}
		}
		for len(res) < 3 {
			res = append(res, 0)
		}
		return res[:3]
	default:
		return []int{1, 0, 0}
	}
}

// sanitizeJSONBytes removes UTF-8 BOM, // and /* */ comments, and trailing commas outside strings.
func sanitizeJSONBytes(data []byte) []byte {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var out bytes.Buffer
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(data) {
			if data[i+1] == '/' {
				i += 2
				for i < len(data) && data[i] != '\n' {
					i++
				}
				if i < len(data) {
					out.WriteByte('\n')
				}
				continue
			}
			if data[i+1] == '*' {
				i += 2
				for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
					i++
				}
				i++
				continue
			}
		}
		out.WriteByte(c)
	}

	cleaned := out.Bytes()
	var finalOut bytes.Buffer
	inString = false
	escaped = false
	for i := 0; i < len(cleaned); i++ {
		c := cleaned[i]
		if inString {
			finalOut.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			finalOut.WriteByte(c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(cleaned) && (cleaned[j] == ' ' || cleaned[j] == '\t' || cleaned[j] == '\r' || cleaned[j] == '\n') {
				j++
			}
			if j < len(cleaned) && (cleaned[j] == '}' || cleaned[j] == ']') {
				continue
			}
		}
		finalOut.WriteByte(c)
	}
	return finalOut.Bytes()
}

// parseManifest tolerantly parses a Bedrock manifest.json.
func parseManifest(data []byte) (*PackManifest, bool, error) {
	clean := sanitizeJSONBytes(data)
	var raw struct {
		FormatVersion int `json:"format_version"`
		Header        struct {
			Name        string      `json:"name"`
			Description string      `json:"description"`
			UUID        string      `json:"uuid"`
			Version     interface{} `json:"version"`
		} `json:"header"`
		Modules []struct {
			Type string `json:"type"`
			UUID string `json:"uuid"`
		} `json:"modules"`
		Dependencies []struct {
			UUID       string      `json:"uuid"`
			ModuleName string      `json:"module_name,omitempty"`
			Version    interface{} `json:"version,omitempty"`
		} `json:"dependencies,omitempty"`
	}
	if err := json.Unmarshal(clean, &raw); err != nil {
		return nil, false, err
	}

	mf := &PackManifest{
		FormatVersion: raw.FormatVersion,
	}
	mf.Header.Name = strings.TrimSpace(raw.Header.Name)
	mf.Header.Description = strings.TrimSpace(raw.Header.Description)
	mf.Header.UUID = strings.TrimSpace(raw.Header.UUID)
	mf.Header.Version = parseVersionSlice(raw.Header.Version)

	isWorldTemplate := false
	for _, m := range raw.Modules {
		if strings.EqualFold(strings.TrimSpace(m.Type), "world_template") {
			isWorldTemplate = true
		}
		mf.Modules = append(mf.Modules, struct {
			Type string `json:"type"`
			UUID string `json:"uuid"`
		}{
			Type: strings.ToLower(strings.TrimSpace(m.Type)),
			UUID: strings.TrimSpace(m.UUID),
		})
	}
	for _, d := range raw.Dependencies {
		mf.Dependencies = append(mf.Dependencies, struct {
			UUID       string      `json:"uuid"`
			ModuleName string      `json:"module_name,omitempty"`
			Version    interface{} `json:"version,omitempty"`
		}{
			UUID:       strings.TrimSpace(d.UUID),
			ModuleName: strings.TrimSpace(d.ModuleName),
			Version:    d.Version,
		})
	}
	return mf, isWorldTemplate, nil
}

// detectPackType determines whether a parsed manifest is a "behavior" or "resource" pack.
func detectPackType(mf *PackManifest) string {
	for _, mod := range mf.Modules {
		t := strings.ToLower(strings.TrimSpace(mod.Type))
		if t == "data" || t == "script" || t == "javascript" || t == "server_data" {
			return "behavior"
		}
	}
	return "resource"
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

// getTargetWorldDirs returns the active world directory plus any other existing world directories under worlds/.
func getTargetWorldDirs(serverDir, levelName string) []string {
	primary := filepath.Join(serverDir, "worlds", levelName)
	dirs := []string{primary}
	seen := map[string]bool{filepath.Clean(primary): true}

	worldsRoot := filepath.Join(serverDir, "worlds")
	if entries, err := os.ReadDir(worldsRoot); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				d := filepath.Join(worldsRoot, e.Name())
				clean := filepath.Clean(d)
				if !seen[clean] {
					seen[clean] = true
					dirs = append(dirs, d)
				}
			}
		}
	}
	return dirs
}

// invalidateKnownPacksCache removes valid_known_packs.json so BDS regenerates it on start.
func invalidateKnownPacksCache(serverDir string) {
	_ = os.Remove(filepath.Join(serverDir, "valid_known_packs.json"))
}

// copyDirRecursive copies all files and subdirectories from src to dst with 0755/0644 permissions.
func copyDirRecursive(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// syncPackFolderToWorlds mirrors or removes a pack directory inside worlds/<world>/<type>_packs/<folder>.
func syncPackFolderToWorlds(serverDir, levelName, packType, folder string, active bool) {
	srcDir := filepath.Join(serverDir, packType+"_packs", folder)
	for _, worldDir := range getTargetWorldDirs(serverDir, levelName) {
		worldPackDir := filepath.Join(worldDir, packType+"_packs", folder)
		if active {
			if st, err := os.Stat(srcDir); err == nil && st.IsDir() {
				_ = copyDirRecursive(srcDir, worldPackDir)
			}
		} else {
			_ = os.RemoveAll(worldPackDir)
		}
	}
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
	if err := json.Unmarshal(sanitizeJSONBytes(data), &refs); err != nil {
		return []WorldPackRef{}, nil
	}
	return refs, nil
}

// SaveWorldPackRefs writes the active pack list for a world and pack type,
// and also writes world_<type>_pack_history.json across all world directories.
func SaveWorldPackRefs(serverDir, levelName, packType string, refs []WorldPackRef) error {
	if refs == nil {
		refs = []WorldPackRef{}
	}

	data, err := json.MarshalIndent(refs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode pack refs: %w", err)
	}

	type historyEntry struct {
		CanBeRedownloaded bool      `json:"can_be_redownloaded"`
		Name              string    `json:"name,omitempty"`
		UUID              string    `json:"uuid"`
		Version           []int     `json:"version"`
	}
	historyPacks := make([]historyEntry, 0, len(refs))
	for _, r := range refs {
		ver := r.Version
		if len(ver) < 3 {
			ver = parseVersionSlice(ver)
		}
		historyPacks = append(historyPacks, historyEntry{
			CanBeRedownloaded: true,
			UUID:              r.PackID,
			Version:           ver,
		})
	}
	historyData, _ := json.MarshalIndent(map[string]interface{}{
		"packs": historyPacks,
	}, "", "  ")

	worldDirs := getTargetWorldDirs(serverDir, levelName)
	for i, worldDir := range worldDirs {
		if err := os.MkdirAll(worldDir, 0755); err != nil {
			if i == 0 {
				return fmt.Errorf("failed to create world directory: %w", err)
			}
			continue
		}
		packFile := filepath.Join(worldDir, fmt.Sprintf("world_%s_packs.json", packType))
		if err := os.WriteFile(packFile, append(data, '\n'), 0644); err != nil && i == 0 {
			return err
		}
		historyFile := filepath.Join(worldDir, fmt.Sprintf("world_%s_pack_history.json", packType))
		_ = os.WriteFile(historyFile, append(historyData, '\n'), 0644)
	}

	invalidateKnownPacksCache(serverDir)
	return nil
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

			if mf, _, err := parseManifest(data); err == nil && mf.Header.UUID != "" {
				verInts := mf.Header.Version
				verStr := fmt.Sprintf("%d.%d.%d", verInts[0], verInts[1], verInts[2])
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

// SanitizeWorldName cleans world names for safe directory and server.properties use.
func SanitizeWorldName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '=' || r == '#' || r == '?' || r == '*' || r == '<' || r == '>' || r == '|' || r == '"' || r < 32 {
			return '_'
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "._-")
	if name == "" {
		return "Bedrock level"
	}
	return name
}

func sanitizePackFolderName(name string) string {
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	name = strings.Trim(name, "_-")
	if name == "" {
		return "pack"
	}
	return name
}

// UpdateActiveLevelNameProp sets level-name in server.properties or pending properties.
func UpdateActiveLevelNameProp(serverDir, levelName string) error {
	updates := map[string]string{
		"level-name": levelName,
	}

	propPath := filepath.Join(serverDir, "server.properties")
	updated, err := configfile.UpdateExistingPropertyFile(propPath, updates)
	if err != nil {
		return fmt.Errorf("failed to update server.properties: %w", err)
	}

	if !updated {
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

// isWorldArchive checks whether a zip contains world data (.mcworld, level.dat, or world_template).
func isWorldArchive(zipReader *zip.Reader) bool {
	for _, f := range zipReader.File {
		normName := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(normName, "__MACOSX/") || strings.HasPrefix(filepath.Base(normName), "._") {
			continue
		}
		base := strings.ToLower(filepath.Base(normName))
		if base == "level.dat" || base == "levelname.txt" || base == "world_icon.jpeg" || base == "world_icon.png" {
			return true
		}
		if strings.HasSuffix(base, ".mcworld") || strings.HasSuffix(base, ".mctemplate") {
			return true
		}
		if strings.Contains(strings.ToLower(normName), "/db/") || strings.HasPrefix(strings.ToLower(normName), "db/") {
			return true
		}
		if base == "manifest.json" {
			rc, err := f.Open()
			if err == nil {
				data, _ := io.ReadAll(io.LimitReader(rc, 64*1024))
				rc.Close()
				if _, isWT, err := parseManifest(data); err == nil && isWT {
					return true
				}
			}
		}
	}
	return false
}

// InstallWorldArchive unzips a Bedrock map archive (.mcworld, world template, or world zip),
// extracts it to worlds/<cleanWorldName>, installs any bundled behavior/resource packs,
// and configures server.properties level-name so the server boots into the new world.
func InstallWorldArchive(serverDir string, r io.ReaderAt, size int64, defaultName string) ([]InstalledPack, error) {
	zipReader, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("invalid world archive: %w", err)
	}

	// 1. Check for nested .mcworld or .mctemplate archives inside
	for _, f := range zipReader.File {
		normName := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(normName, "__MACOSX/") || strings.HasPrefix(filepath.Base(normName), "._") {
			continue
		}
		lowerName := strings.ToLower(normName)
		if strings.HasSuffix(lowerName, ".mcworld") || strings.HasSuffix(lowerName, ".mctemplate") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			innerBytes, err := io.ReadAll(rc)
			rc.Close()
			if err == nil && len(innerBytes) > 0 {
				innerName := defaultName
				if innerName == "" {
					innerBase := filepath.Base(normName)
					innerName = strings.TrimSuffix(innerBase, filepath.Ext(innerBase))
				}
				packs, err := InstallWorldArchive(serverDir, bytes.NewReader(innerBytes), int64(len(innerBytes)), innerName)
				if err == nil && len(packs) > 0 {
					return packs, nil
				}
			}
		}
	}

	// 2. Identify world root prefix inside the archive
	var worldPrefix string
	var foundWorldData bool
	var templateManifest *PackManifest

	// Check manifest.json for world_template info
	for _, f := range zipReader.File {
		normName := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(normName, "__MACOSX/") || strings.HasPrefix(filepath.Base(normName), "._") {
			continue
		}
		if strings.EqualFold(filepath.Base(normName), "manifest.json") {
			rc, err := f.Open()
			if err == nil {
				data, _ := io.ReadAll(rc)
				rc.Close()
				if mf, isWT, err := parseManifest(data); err == nil && isWT {
					templateManifest = mf
					dir := filepath.Dir(normName)
					if dir == "." || dir == "/" || dir == "" {
						worldPrefix = ""
					} else {
						worldPrefix = strings.TrimSuffix(dir, "/") + "/"
					}
					foundWorldData = true
					break
				}
			}
		}
	}

	// Check level.dat
	for _, f := range zipReader.File {
		normName := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(normName, "__MACOSX/") || strings.HasPrefix(filepath.Base(normName), "._") {
			continue
		}
		if strings.EqualFold(filepath.Base(normName), "level.dat") {
			dir := filepath.Dir(normName)
			if dir == "." || dir == "/" || dir == "" {
				worldPrefix = ""
			} else {
				worldPrefix = strings.TrimSuffix(dir, "/") + "/"
			}
			foundWorldData = true
			break
		}
	}

	// Check db/, levelname.txt, or world_icon.jpeg
	if !foundWorldData {
		for _, f := range zipReader.File {
			normName := strings.ReplaceAll(f.Name, "\\", "/")
			if strings.HasPrefix(normName, "__MACOSX/") || strings.HasPrefix(filepath.Base(normName), "._") {
				continue
			}
			base := strings.ToLower(filepath.Base(normName))
			if base == "levelname.txt" || base == "world_icon.jpeg" || base == "world_icon.png" || strings.Contains(strings.ToLower(normName), "/db/") || strings.HasPrefix(strings.ToLower(normName), "db/") {
				dir := filepath.Dir(normName)
				if strings.Contains(strings.ToLower(normName), "/db/") || strings.HasPrefix(strings.ToLower(normName), "db/") {
					idx := strings.Index(strings.ToLower(normName), "db/")
					dir = normName[:idx]
				}
				if dir == "." || dir == "/" || dir == "" {
					worldPrefix = ""
				} else {
					worldPrefix = strings.TrimSuffix(dir, "/") + "/"
				}
				foundWorldData = true
				break
			}
		}
	}

	if !foundWorldData {
		return nil, fmt.Errorf("archive does not contain valid Bedrock world data (level.dat or world template)")
	}

	// 3. Determine world name
	worldName := ""

	// Check levelname.txt in the archive first
	for _, f := range zipReader.File {
		normName := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(normName, "__MACOSX/") || strings.HasPrefix(filepath.Base(normName), "._") {
			continue
		}
		expectedName := worldPrefix + "levelname.txt"
		if worldPrefix == "" && strings.EqualFold(filepath.Base(normName), "levelname.txt") {
			expectedName = normName
		}
		if strings.EqualFold(normName, expectedName) {
			rc, err := f.Open()
			if err == nil {
				txtData, _ := io.ReadAll(io.LimitReader(rc, 1024))
				rc.Close()
				cleanTxt := strings.TrimSpace(string(txtData))
				if cleanTxt != "" {
					worldName = cleanTxt
					break
				}
			}
		}
	}

	// If no levelname.txt, try template manifest header name
	if worldName == "" && templateManifest != nil && templateManifest.Header.Name != "" {
		worldName = templateManifest.Header.Name
	}

	// If no manifest header name, try defaultName
	if worldName == "" && defaultName != "" {
		worldName = defaultName
	}

	// If no defaultName, try enclosing folder name
	if worldName == "" && worldPrefix != "" {
		trimmedPrefix := strings.Trim(worldPrefix, "/")
		baseDir := filepath.Base(trimmedPrefix)
		if !strings.EqualFold(baseDir, "worlds") && !strings.EqualFold(baseDir, "world") && !strings.EqualFold(baseDir, "world_template") && baseDir != "." && baseDir != "" {
			worldName = baseDir
		}
	}

	if worldName == "" {
		worldName = "Bedrock level"
	}
	cleanWorldName := SanitizeWorldName(worldName)

	// 4. Target world directory
	worldsDir := filepath.Join(serverDir, "worlds")
	targetWorldDir := filepath.Join(worldsDir, cleanWorldName)
	worldsDirClean := filepath.Clean(worldsDir) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(targetWorldDir)+string(filepath.Separator), worldsDirClean) {
		return nil, fmt.Errorf("invalid world name: path traversal attempt detected")
	}

	// Remove any existing world files at target to prevent stale LevelDB database corruption
	_ = os.RemoveAll(targetWorldDir)
	if err := os.MkdirAll(targetWorldDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create world directory: %w", err)
	}

	// 5. Extract all files into targetWorldDir
	for _, f := range zipReader.File {
		normName := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(normName, "__MACOSX/") || strings.HasPrefix(filepath.Base(normName), "._") {
			continue
		}

		var rel string
		if worldPrefix != "" && strings.HasPrefix(normName, worldPrefix) {
			rel = strings.TrimPrefix(normName, worldPrefix)
		} else if strings.HasPrefix(normName, "behavior_packs/") || strings.HasPrefix(normName, "resource_packs/") {
			rel = normName
		} else if worldPrefix == "" {
			rel = normName
		} else {
			continue
		}

		cleanedRel := filepath.Clean(rel)
		if cleanedRel == "." || cleanedRel == "" || strings.HasPrefix(cleanedRel, "..") || filepath.IsAbs(cleanedRel) {
			continue
		}

		destPath := filepath.Join(targetWorldDir, cleanedRel)
		targetWorldClean := filepath.Clean(targetWorldDir) + string(filepath.Separator)
		if !strings.HasPrefix(filepath.Clean(destPath)+string(filepath.Separator), targetWorldClean) && filepath.Clean(destPath) != filepath.Clean(targetWorldDir) {
			continue
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(destPath, 0755)
			continue
		}

		_ = os.MkdirAll(filepath.Dir(destPath), 0755)
		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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

	// Ensure levelname.txt exists
	levelNameTxtPath := filepath.Join(targetWorldDir, "levelname.txt")
	if _, err := os.Stat(levelNameTxtPath); os.IsNotExist(err) {
		_ = os.WriteFile(levelNameTxtPath, []byte(cleanWorldName+"\n"), 0644)
	}

	// 6. Scan targetWorldDir for bundled behavior_packs and resource_packs
	var bundledPacks []InstalledPack
	hasResource := false
	for _, packType := range []string{"behavior", "resource"} {
		worldPacksDir := filepath.Join(targetWorldDir, packType+"_packs")
		entries, err := os.ReadDir(worldPacksDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			packSrcDir := filepath.Join(worldPacksDir, entry.Name())
			mfPath := filepath.Join(packSrcDir, "manifest.json")
			mfData, err := os.ReadFile(mfPath)
			if err != nil {
				continue
			}
			mf, _, err := parseManifest(mfData)
			if err != nil || mf.Header.UUID == "" {
				continue
			}
			detectedType := detectPackType(mf)
			folderName := sanitizePackFolderName(mf.Header.Name)
			if folderName == "" {
				folderName = sanitizePackFolderName(entry.Name())
			}

			serverPackDir := filepath.Join(serverDir, detectedType+"_packs", folderName)
			_ = copyDirRecursive(packSrcDir, serverPackDir)
			_ = os.WriteFile(filepath.Join(serverPackDir, bsmInstalledMarker), []byte("installed-by-bsm\n"), 0644)

			// Ensure active in world configuration
			refs, _ := GetWorldPackRefs(serverDir, cleanWorldName, detectedType)
			found := false
			for _, r := range refs {
				if strings.EqualFold(r.PackID, mf.Header.UUID) {
					found = true
					break
				}
			}
			if !found {
				refs = append(refs, WorldPackRef{
					PackID:  mf.Header.UUID,
					Version: mf.Header.Version,
				})
				_ = SaveWorldPackRefs(serverDir, cleanWorldName, detectedType, refs)
			}

			// Ensure mirrored into world packs dir under normalized folderName
			_ = copyDirRecursive(serverPackDir, filepath.Join(targetWorldDir, detectedType+"_packs", folderName))

			if detectedType == "resource" {
				hasResource = true
			}

			verStr := fmt.Sprintf("%d.%d.%d", mf.Header.Version[0], mf.Header.Version[1], mf.Header.Version[2])
			name := mf.Header.Name
			if name == "" {
				name = folderName
			}
			bundledPacks = append(bundledPacks, InstalledPack{
				Type:        detectedType,
				Folder:      folderName,
				Name:        name,
				Description: mf.Header.Description,
				UUID:        mf.Header.UUID,
				Version:     verStr,
				VersionInts: mf.Header.Version,
				Active:      true,
			})
		}
	}

	// 7. Check for nested .mcpack or .mcaddon archives inside the archive
	for _, f := range zipReader.File {
		normName := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(normName, "__MACOSX/") || strings.HasPrefix(filepath.Base(normName), "._") {
			continue
		}
		lowerName := strings.ToLower(normName)
		if strings.HasSuffix(lowerName, ".mcpack") || strings.HasSuffix(lowerName, ".mcaddon") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			packBytes, _ := io.ReadAll(rc)
			rc.Close()
			if len(packBytes) > 0 {
				subPacks, err := InstallAllPacks(serverDir, bytes.NewReader(packBytes), int64(len(packBytes)))
				if err == nil {
					for _, sp := range subPacks {
						if sp.Type == "behavior" || sp.Type == "resource" {
							_, _ = setPackActiveInternal(serverDir, sp.Type, sp.Folder, true, true)
							bundledPacks = append(bundledPacks, sp)
							if sp.Type == "resource" {
								hasResource = true
							}
						}
					}
				}
			}
		}
	}

	// 8. Update level-name in server.properties and invalidate cache
	_ = UpdateActiveLevelNameProp(serverDir, cleanWorldName)

	if hasResource {
		_ = updateTexturePackRequiredProp(serverDir, true)
	}

	invalidateKnownPacksCache(serverDir)

	worldPack := InstalledPack{
		Type:        "world",
		Folder:      cleanWorldName,
		Name:        cleanWorldName,
		Description: fmt.Sprintf("Bedrock world map installed to worlds/%s", cleanWorldName),
		UUID:        cleanWorldName,
		Active:      true,
	}

	return append([]InstalledPack{worldPack}, bundledPacks...), nil
}

// InstallAllPacks unzips an archive and extracts all behavior packs, resource packs, or world maps found within it.
func InstallAllPacks(serverDir string, r io.ReaderAt, size int64, defaultName ...string) ([]InstalledPack, error) {
	zipReader, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("invalid archive: %w", err)
	}

	if isWorldArchive(zipReader) {
		name := ""
		if len(defaultName) > 0 {
			name = defaultName[0]
		}
		return InstallWorldArchive(serverDir, r, size, name)
	}

	type manifestEntry struct {
		data            []byte
		prefix          string
		mf              *PackManifest
		isWorldTemplate bool
	}

	var manifests []manifestEntry
	for _, f := range zipReader.File {
		normName := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(normName, "__MACOSX/") || strings.HasPrefix(filepath.Base(normName), "._") {
			continue
		}
		if strings.EqualFold(filepath.Base(normName), "manifest.json") {
			rc, err := f.Open()
			if err == nil {
				data, _ := io.ReadAll(rc)
				rc.Close()
				if len(data) > 0 {
					cleaned := filepath.Clean(normName)
					dir := filepath.Dir(cleaned)
					prefix := ""
					if dir != "." && dir != "/" && dir != "" {
						prefix = dir + "/"
					}
					if mf, isWT, err := parseManifest(data); err == nil && mf.Header.UUID != "" {
						manifests = append(manifests, manifestEntry{
							data:            data,
							prefix:          prefix,
							mf:              mf,
							isWorldTemplate: isWT,
						})
					}
				}
			}
		}
	}

	var installedPacks []InstalledPack

	// Also extract any nested .mcpack, .mcaddon, or .zip archives contained inside the archive
	for _, f := range zipReader.File {
		normName := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(normName, "__MACOSX/") || strings.HasPrefix(filepath.Base(normName), "._") {
			continue
		}
		lowerName := strings.ToLower(normName)
		if strings.HasSuffix(lowerName, ".mcpack") || strings.HasSuffix(lowerName, ".mcaddon") || strings.HasSuffix(lowerName, ".zip") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			packBytes, err := io.ReadAll(rc)
			rc.Close()
			if err != nil || len(packBytes) == 0 {
				continue
			}
			subPacks, err := InstallAllPacks(serverDir, bytes.NewReader(packBytes), int64(len(packBytes)))
			if err == nil && len(subPacks) > 0 {
				installedPacks = append(installedPacks, subPacks...)
			}
		}
	}

	if len(manifests) == 0 {
		if len(installedPacks) > 0 {
			return installedPacks, nil
		}
		return nil, fmt.Errorf("archive does not contain a valid manifest.json or .mcpack files")
	}

	// Process each manifest found in the archive
	for _, m := range manifests {
		if m.isWorldTemplate && (len(manifests) > 1 || len(installedPacks) > 0) {
			continue
		}
		mf := m.mf
		packType := detectPackType(mf)

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
			continue
		}

		for _, f := range zipReader.File {
			normName := strings.ReplaceAll(f.Name, "\\", "/")
			if strings.HasPrefix(normName, "__MACOSX/") || strings.HasPrefix(filepath.Base(normName), "._") {
				continue
			}
			cleaned := filepath.Clean(normName)
			if strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") {
				continue
			}

			if m.prefix != "" {
				if strings.HasPrefix(normName, m.prefix) {
					cleaned = filepath.Clean(strings.TrimPrefix(normName, m.prefix))
				} else {
					continue
				}
			}
			if cleaned == "" || cleaned == "." {
				continue
			}

			// Skip other sub-pack prefixes when a root manifest coexists with sub-packs
			skipOtherPack := false
			for _, other := range manifests {
				if other.prefix != "" && other.prefix != m.prefix && strings.HasPrefix(normName, other.prefix) {
					skipOtherPack = true
					break
				}
			}
			if skipOtherPack {
				continue
			}

			destPath := filepath.Join(targetDir, cleaned)
			targetDirClean := filepath.Clean(targetDir) + string(filepath.Separator)
			if !strings.HasPrefix(filepath.Clean(destPath), targetDirClean) {
				continue
			}

			if f.FileInfo().IsDir() {
				_ = os.MkdirAll(destPath, 0755)
				continue
			}

			_ = os.MkdirAll(filepath.Dir(destPath), 0755)
			outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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

		_ = os.WriteFile(filepath.Join(targetDir, bsmInstalledMarker), []byte("installed-by-bsm\n"), 0644)

		verStr := fmt.Sprintf("%d.%d.%d", mf.Header.Version[0], mf.Header.Version[1], mf.Header.Version[2])

		installedPacks = append(installedPacks, InstalledPack{
			Type:        packType,
			Folder:      folderName,
			Name:        mf.Header.Name,
			Description: mf.Header.Description,
			UUID:        mf.Header.UUID,
			Version:     verStr,
			VersionInts: mf.Header.Version,
		})
	}

	if len(installedPacks) == 0 {
		return nil, fmt.Errorf("failed to extract any valid packs from archive")
	}

	invalidateKnownPacksCache(serverDir)
	return installedPacks, nil
}

// InstallPack unzips a .mcpack, .mcaddon, .mcworld, or .zip into the appropriate server directory.
func InstallPack(serverDir string, r io.ReaderAt, size int64, defaultName ...string) (*InstalledPack, error) {
	packs, err := InstallAllPacks(serverDir, r, size, defaultName...)
	if err != nil {
		return nil, err
	}
	if len(packs) == 0 {
		return nil, fmt.Errorf("no packs found in archive")
	}
	return &packs[0], nil
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

	invalidateKnownPacksCache(serverDir)
	return os.RemoveAll(targetDir)
}

// SetPackActive activates or deactivates an installed pack in the active world's configuration file.
func SetPackActive(serverDir, packType, folder string, active bool) (*InstalledPack, error) {
	return setPackActiveInternal(serverDir, packType, folder, active, true)
}

func setPackActiveInternal(serverDir, packType, folder string, active bool, followDeps bool) (*InstalledPack, error) {
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

	mf, _, err := parseManifest(data)
	if err != nil {
		return nil, fmt.Errorf("invalid pack manifest: %w", err)
	}

	packUUID := mf.Header.UUID
	if packUUID == "" {
		return nil, fmt.Errorf("pack manifest missing header.uuid")
	}

	verInts := mf.Header.Version

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

	// Mirror or remove the pack inside worlds/<levelName>/<type>_packs/<folder>
	syncPackFolderToWorlds(serverDir, levelName, packType, cleanFolder, active)

	if active {
		if packType == "resource" {
			_ = updateTexturePackRequiredProp(serverDir, true)
		}
		if followDeps {
			companionType := "resource"
			if packType == "resource" {
				companionType = "behavior"
			}
			companionDir := filepath.Join(serverDir, companionType+"_packs")
			if entries, err := os.ReadDir(companionDir); err == nil {
				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}
					compManifestPath := filepath.Join(companionDir, entry.Name(), "manifest.json")
					if compData, err := os.ReadFile(compManifestPath); err == nil {
						if compMf, _, err := parseManifest(compData); err == nil {
							matched := false
							for _, dep := range mf.Dependencies {
								if dep.UUID != "" && strings.EqualFold(dep.UUID, compMf.Header.UUID) {
									matched = true
									break
								}
							}
							if !matched {
								for _, dep := range compMf.Dependencies {
									if dep.UUID != "" && strings.EqualFold(dep.UUID, packUUID) {
										matched = true
										break
									}
								}
							}
							if matched {
								_, _ = setPackActiveInternal(serverDir, companionType, entry.Name(), true, false)
							}
						}
					}
				}
			}
		}
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

func updateTexturePackRequiredProp(serverDir string, texturePackRequired bool) error {
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

// SyncActivePacksToWorlds ensures all active packs have their world_<type>_packs.json and world folders synced.
func SyncActivePacksToWorlds(serverDir string) {
	levelName := GetActiveLevelName(serverDir)
	packs, err := ListInstalledPacks(serverDir)
	if err != nil {
		return
	}
	for _, p := range packs {
		if p.Active {
			_, _ = setPackActiveInternal(serverDir, p.Type, p.Folder, true, false)
		}
	}
	for _, pt := range []string{"behavior", "resource"} {
		if refs, err := GetWorldPackRefs(serverDir, levelName, pt); err == nil && len(refs) > 0 {
			_ = SaveWorldPackRefs(serverDir, levelName, pt, refs)
		}
	}
}

// UpdateAddonConfig updates addon-related settings in server.properties or pending properties,
// and ensures installed packs are synced to the active world.
func UpdateAddonConfig(serverDir string, texturePackRequired bool) error {
	if err := updateTexturePackRequiredProp(serverDir, texturePackRequired); err != nil {
		return err
	}

	if texturePackRequired {
		packs, err := ListInstalledPacks(serverDir)
		if err == nil && len(packs) > 0 {
			anyActive := false
			for _, p := range packs {
				if p.Active {
					anyActive = true
					break
				}
			}
			for _, p := range packs {
				if !anyActive || p.Active {
					_, _ = setPackActiveInternal(serverDir, p.Type, p.Folder, true, true)
				}
			}
		}
	}

	SyncActivePacksToWorlds(serverDir)
	return nil
}

// InstallAllFromURL downloads a pack or world archive from a direct URL and installs all packs/worlds inside it.
func InstallAllFromURL(ctx context.Context, serverDir, downloadURL string, defaultName ...string) ([]InstalledPack, error) {
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
		Timeout: 300 * time.Second,
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

	// Max 500MB limit to accommodate rich world maps and textures
	size, err := io.Copy(tempFile, io.LimitReader(resp.Body, 500<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read download: %w", err)
	}
	if size == 0 {
		return nil, fmt.Errorf("downloaded file is empty")
	}

	return InstallAllPacks(serverDir, tempFile, size, defaultName...)
}

// InstallFromURL downloads a pack or world archive from a direct URL and installs it.
func InstallFromURL(ctx context.Context, serverDir, downloadURL string, defaultName ...string) (*InstalledPack, error) {
	packs, err := InstallAllFromURL(ctx, serverDir, downloadURL, defaultName...)
	if err != nil {
		return nil, err
	}
	if len(packs) == 0 {
		return nil, fmt.Errorf("no packs found in archive")
	}
	return &packs[0], nil
}
