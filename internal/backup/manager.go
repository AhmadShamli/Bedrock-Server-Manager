package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/addon"
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

	return zipDirectory(worldDir, outPath, nil)
}

// ImportWorld extracts an uploaded .mcworld or .zip into the server's worlds folder.
func ImportWorld(serverDir, worldName string, fileReader io.ReaderAt, size int64) error {
	cleanWorldName := filepath.Clean(worldName)
	if strings.Contains(cleanWorldName, "..") || strings.Contains(cleanWorldName, "/") || strings.Contains(cleanWorldName, "\\") || filepath.IsAbs(cleanWorldName) {
		return fmt.Errorf("invalid world name: path traversal attempt")
	}

	_, err := addon.InstallWorldArchive(serverDir, fileReader, size, cleanWorldName)
	return err
}

// ResetWorld deletes all existing world data (the worlds/ directory and any pre-restore world backups)
// so that Bedrock Dedicated Server generates a clean new world on next start.
func ResetWorld(serverDir string) error {
	worldsDir := filepath.Join(serverDir, "worlds")
	if err := os.RemoveAll(worldsDir); err != nil {
		return fmt.Errorf("failed to remove worlds directory: %w", err)
	}

	// Clean up any rotated worlds_pre_restore_* folders in serverDir
	if entries, err := os.ReadDir(serverDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "worlds_pre_restore_") {
				_ = os.RemoveAll(filepath.Join(serverDir, entry.Name()))
			}
		}
	}

	if err := os.MkdirAll(worldsDir, 0755); err != nil {
		return fmt.Errorf("failed to recreate clean worlds directory: %w", err)
	}

	return nil
}

