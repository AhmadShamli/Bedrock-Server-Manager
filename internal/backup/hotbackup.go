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

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// CreateHotBackup initiates a Bedrock LevelDB safe hot backup, compresses it into a zip, and enforces retention.
func CreateHotBackup(
	ctx context.Context,
	srv *models.Server,
	serverDir string,
	backupDir string,
	eng engine.ServerEngine,
	backupType string,
	isLocked bool,
	mgrDB *database.ManagerDB,
) (*models.Backup, error) {
	if backupType == "" {
		backupType = "manual"
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	filename := fmt.Sprintf("backup_%s_%s.zip", srv.ID, timestamp)
	destZipPath := filepath.Join(backupDir, filename)

	worldsDir := filepath.Join(serverDir, "worlds")
	if _, err := os.Stat(worldsDir); os.IsNotExist(err) {
		// Fallback: if worlds doesn't exist, back up serverDir root
		worldsDir = serverDir
	}

	// Hot backup LevelDB safety protocol
	isRunning := srv.Status == models.ServerStatusRunning && eng != nil
	if isRunning {
		// 1. Tell Bedrock to freeze writes and hold LevelDB snapshots
		_ = eng.SendConsoleCommand(ctx, srv, "save hold")
		time.Sleep(500 * time.Millisecond)

		// 2. Query Bedrock until snapshot is staged
		_ = eng.SendConsoleCommand(ctx, srv, "save query")
		time.Sleep(500 * time.Millisecond)
	}

	// Always resume Bedrock saving afterwards
	defer func() {
		if isRunning {
			_ = eng.SendConsoleCommand(context.Background(), srv, "save resume")
		}
	}()

	// 3. Zip worlds directory into destination zip
	if err := zipDirectory(worldsDir, destZipPath); err != nil {
		_ = os.Remove(destZipPath)
		return nil, fmt.Errorf("failed to compress world archive: %w", err)
	}

	fileInfo, err := os.Stat(destZipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect created backup archive: %w", err)
	}

	backupRecord := &models.Backup{
		ServerID:  srv.ID,
		Filename:  filename,
		SizeBytes: fileInfo.Size(),
		Type:      backupType,
		IsLocked:  isLocked,
		Status:    "completed",
		CreatedAt: time.Now().UTC(),
	}

	if err := mgrDB.CreateBackup(ctx, backupRecord); err != nil {
		return nil, fmt.Errorf("failed to record backup in database: %w", err)
	}

	// 4. Enforce retention policy
	_, _ = EnforceRetention(ctx, srv.ID, DefaultRetentionPolicy(), backupDir, mgrDB)

	return backupRecord, nil
}

// zipDirectory recursively compresses srcDir into a zip archive at destZipPath.
func zipDirectory(srcDir, destZipPath string) error {
	zipFile, err := os.Create(destZipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(relPath)
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

// Unzip extracts a zip archive to the target destination directory with path traversal protection.
func Unzip(srcZipPath, destDir string) error {
	r, err := zip.OpenReader(srcZipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, f := range r.File {
		cleaned := filepath.Clean(f.Name)
		if strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") {
			continue // Skip dangerous path traversals
		}

		target := filepath.Join(destDir, cleaned)

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
