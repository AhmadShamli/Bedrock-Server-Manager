package backup

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// RestoreBackup safely restores a world backup into the server directory.
// Requires the server to be stopped to prevent database write races.
func RestoreBackup(ctx context.Context, srv *models.Server, serverDir string, backupZipPath string, eng engine.ServerEngine) error {
	if srv.Status == models.ServerStatusRunning {
		return fmt.Errorf("server must be stopped before restoring a backup")
	}

	if _, err := os.Stat(backupZipPath); os.IsNotExist(err) {
		return fmt.Errorf("backup archive not found at %s", backupZipPath)
	}

	worldsDir := filepath.Join(serverDir, "worlds")

	// Rotate existing worlds to a safety backup before overwriting
	if _, err := os.Stat(worldsDir); err == nil {
		backupOldName := filepath.Join(serverDir, fmt.Sprintf("worlds_pre_restore_%d", time.Now().Unix()))
		_ = os.Rename(worldsDir, backupOldName)
	}

	if err := os.MkdirAll(worldsDir, 0755); err != nil {
		return fmt.Errorf("failed to recreate worlds directory: %w", err)
	}

	// Extract backup into worlds directory
	if err := Unzip(backupZipPath, worldsDir); err != nil {
		return fmt.Errorf("failed to unpack backup archive: %w", err)
	}

	return nil
}

// ExportWorld packages a specific world folder as a downloadable .mcworld / .zip archive.
func ExportWorld(serverDir, worldName, outPath string) error {
	cleanWorldName := filepath.Clean(worldName)
	if strings.Contains(cleanWorldName, "..") || filepath.IsAbs(cleanWorldName) {
		return fmt.Errorf("invalid world name: path traversal attempt")
	}

	worldDir := filepath.Join(serverDir, "worlds", cleanWorldName)
	if _, err := os.Stat(worldDir); os.IsNotExist(err) {
		// If specific worldDir doesn't exist, check default Bedrock "Bedrock level"
		worldDir = filepath.Join(serverDir, "worlds")
	}

	return zipDirectory(worldDir, outPath)
}

// ImportWorld extracts an uploaded .mcworld or .zip into the server's worlds folder.
func ImportWorld(serverDir, worldName string, fileReader io.ReaderAt, size int64) error {
	cleanWorldName := filepath.Clean(worldName)
	if strings.Contains(cleanWorldName, "..") || strings.Contains(cleanWorldName, "/") || strings.Contains(cleanWorldName, "\\") || filepath.IsAbs(cleanWorldName) {
		return fmt.Errorf("invalid world name: path traversal attempt")
	}

	zipReader, err := zip.NewReader(fileReader, size)
	if err != nil {
		return fmt.Errorf("invalid world archive: %w", err)
	}

	targetDir := filepath.Join(serverDir, "worlds", cleanWorldName)
	worldsDirClean := filepath.Clean(filepath.Join(serverDir, "worlds")) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(targetDir)+string(filepath.Separator), worldsDirClean) {
		return fmt.Errorf("path traversal attempt detected")
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	// Detect if files are wrapped in an enclosing folder (e.g. MyWorld/level.dat)
	var rootPrefix string
	for _, f := range zipReader.File {
		cleaned := filepath.Clean(f.Name)
		if strings.EqualFold(filepath.Base(cleaned), "level.dat") {
			dir := filepath.Dir(cleaned)
			if dir != "." && dir != "/" && dir != "" {
				rootPrefix = dir + "/"
			}
			break
		}
	}

	for _, f := range zipReader.File {
		cleaned := filepath.Clean(f.Name)
		if strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") {
			continue
		}

		if rootPrefix != "" {
			if strings.HasPrefix(f.Name, rootPrefix) {
				cleaned = filepath.Clean(strings.TrimPrefix(f.Name, rootPrefix))
			} else {
				continue
			}
		}
		if cleaned == "" || cleaned == "." {
			continue
		}

		target := filepath.Join(targetDir, cleaned)
		targetDirClean := filepath.Clean(targetDir) + string(filepath.Separator)
		if !strings.HasPrefix(filepath.Clean(target), targetDirClean) {
			continue // Zip Slip path traversal attempt blocked
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
