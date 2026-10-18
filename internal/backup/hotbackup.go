package backup

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
	var snapshotFiles map[string]int64
	if isRunning {
		snapshotFiles = waitForBedrockSnapshot(ctx, srv, eng)
	}

	// Always resume Bedrock saving afterwards
	defer func() {
		if isRunning {
			_ = eng.SendConsoleCommand(context.Background(), srv, "save resume")
		}
	}()

	// 3. Zip worlds directory into destination zip
	if err := zipDirectory(worldsDir, destZipPath, snapshotFiles); err != nil {
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
	_, _ = EnforceRetention(ctx, srv.ID, ResolveRetentionPolicy(ctx, mgrDB), backupDir, mgrDB)

	return backupRecord, nil
}

// waitForBedrockSnapshot issues save hold and queries until snapshot is staged.
func waitForBedrockSnapshot(ctx context.Context, srv *models.Server, eng engine.ServerEngine) map[string]int64 {
	logCh, unsub := eng.SubscribeLogs(srv.ID)
	defer unsub()

	_ = eng.SendConsoleCommand(ctx, srv, "save hold")

	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(8 * time.Second)

	_ = eng.SendConsoleCommand(ctx, srv, "save query")

	snapshot := make(map[string]int64)
	ready := false

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timeout:
			return snapshot // Return whatever was discovered or empty fallback
		case line := <-logCh:
			clean := strings.TrimSpace(line)
			if strings.Contains(clean, "Data saved. Files are now ready to be copied") {
				ready = true
				continue
			}
			if ready {
				// Parse file list: e.g. "world/db/000005.ldb:12345, world/level.dat:5678"
				parts := strings.Split(clean, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if idx := strings.LastIndex(p, ":"); idx > 0 {
						fn := strings.TrimSpace(p[:idx])
						lenStr := strings.TrimSpace(p[idx+1:])
						if l, err := strconv.ParseInt(lenStr, 10, 64); err == nil && fn != "" {
							snapshot[filepath.ToSlash(filepath.Clean(fn))] = l
						}
					}
				}
				if len(snapshot) > 0 {
					return snapshot
				}
			}
		case <-ticker.C:
			if !ready {
				_ = eng.SendConsoleCommand(ctx, srv, "save query")
			}
		}
	}
}

// zipDirectory recursively compresses srcDir into a zip archive at destZipPath, skipping LevelDB LOCK files.
func zipDirectory(srcDir, destZipPath string, snapshot map[string]int64) error {
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

		baseName := filepath.Base(path)
		if baseName == "LOCK" || strings.HasSuffix(baseName, ".lock") {
			return nil // Skip LevelDB LOCK file to prevent Windows sharing violations
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

		return func() error {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			var reader io.Reader = file
			cleanRel := filepath.ToSlash(relPath)
			if maxLen, ok := snapshot[cleanRel]; ok && maxLen >= 0 {
				reader = io.LimitReader(file, maxLen)
			} else {
				for snapPath, snapLen := range snapshot {
					if strings.HasSuffix(cleanRel, snapPath) && snapLen >= 0 {
						reader = io.LimitReader(file, snapLen)
						break
					}
				}
			}

			_, err = io.Copy(writer, reader)
			return err
		}()
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

	destDirClean := filepath.Clean(destDir) + string(filepath.Separator)

	for _, f := range r.File {
		cleaned := filepath.Clean(f.Name)
		if strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") {
			continue // Skip dangerous path traversals
		}

		target := filepath.Join(destDir, cleaned)
		if !strings.HasPrefix(filepath.Clean(target), destDirClean) {
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
