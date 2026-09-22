package addon

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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
	var packs []InstalledPack

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
			manifestPath := filepath.Join(fullPath, entry.Name(), "manifest.json")
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
	for _, f := range zipReader.File {
		if strings.EqualFold(filepath.Base(f.Name), "manifest.json") {
			rc, err := f.Open()
			if err == nil {
				manifestData, _ = io.ReadAll(rc)
				rc.Close()
				break
			}
		}
	}

	if len(manifestData) == 0 {
		return nil, fmt.Errorf("archive does not contain a valid manifest.json")
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

		destPath := filepath.Join(targetDir, cleaned)
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
	var targetDir string
	if packType == "behavior" {
		targetDir = filepath.Join(serverDir, "behavior_packs", filepath.Clean(folder))
	} else {
		targetDir = filepath.Join(serverDir, "resource_packs", filepath.Clean(folder))
	}

	return os.RemoveAll(targetDir)
}
