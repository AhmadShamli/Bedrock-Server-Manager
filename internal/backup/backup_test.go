package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func TestHotBackupAndRetentionFlow(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "server1")
	worldsDir := filepath.Join(serverDir, "worlds", "Bedrock level")
	backupDir := filepath.Join(tmpDir, "backups")
	_ = os.MkdirAll(worldsDir, 0755)

	// Create sample world files
	_ = os.WriteFile(filepath.Join(worldsDir, "level.dat"), []byte("sample-level-dat-content"), 0644)
	_ = os.WriteFile(filepath.Join(worldsDir, "levelname.txt"), []byte("My World"), 0644)

	db, err := database.OpenManagerDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}
	defer db.Close()

	srv := &models.Server{
		ID:     "srv-backup-test",
		Name:   "Backup World",
		Status: models.ServerStatusRunning,
	}
	_ = db.CreateServer(ctx, srv)

	mockEng := engine.NewMockEngine()

	// 1. Trigger hot backup
	b1, err := CreateHotBackup(ctx, srv, serverDir, backupDir, mockEng, "manual", false, db)
	if err != nil {
		t.Fatalf("CreateHotBackup failed: %v", err)
	}

	if b1.SizeBytes <= 0 {
		t.Fatalf("expected non-zero size, got %d", b1.SizeBytes)
	}

	// Verify file exists on disk
	if _, err := os.Stat(filepath.Join(backupDir, b1.Filename)); err != nil {
		t.Fatalf("expected backup file on disk: %v", err)
	}

	// 2. Trigger a second locked backup
	b2, err := CreateHotBackup(ctx, srv, serverDir, backupDir, mockEng, "manual", true, db)
	if err != nil {
		t.Fatalf("CreateHotBackup 2 failed: %v", err)
	}
	if !b2.IsLocked {
		t.Fatalf("expected b2 to be locked")
	}

	// 3. Test Retention Policy: MaxCount = 1 unpinned backup
	policy := RetentionPolicy{
		MaxCount: 1,
	}

	// Trigger third unpinned backup, should prune b1, but KEEP b2 (since b2 is locked!)
	b3, err := CreateHotBackup(ctx, srv, serverDir, backupDir, mockEng, "manual", false, db)
	if err != nil {
		t.Fatalf("CreateHotBackup 3 failed: %v", err)
	}

	pruned, err := EnforceRetention(ctx, srv.ID, policy, backupDir, db)
	if err != nil {
		t.Fatalf("EnforceRetention failed: %v", err)
	}
	if len(pruned) != 1 || pruned[0] != b1.ID {
		t.Fatalf("expected b1 (#%d) to be pruned, got %v", b1.ID, pruned)
	}

	// Verify b1 file removed from disk
	if _, err := os.Stat(filepath.Join(backupDir, b1.Filename)); !os.IsNotExist(err) {
		t.Fatalf("expected b1 file to be deleted from disk")
	}

	// Verify b2 (locked) and b3 (latest unpinned) remain in DB
	remaining, _ := db.ListBackups(ctx, srv.ID)
	if len(remaining) != 2 {
		t.Fatalf("expected 2 backups remaining, got %d", len(remaining))
	}

	// 4. Test Restore Backup
	// First test error when server is running
	err = RestoreBackup(ctx, srv, serverDir, filepath.Join(backupDir, b3.Filename), mockEng)
	if err == nil {
		t.Fatalf("expected error when restoring to a running server")
	}

	// Stop server and restore
	srv.Status = models.ServerStatusStopped
	err = RestoreBackup(ctx, srv, serverDir, filepath.Join(backupDir, b3.Filename), mockEng)
	if err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}

	// Verify level.dat restored
	restoredDat, err := os.ReadFile(filepath.Join(serverDir, "worlds", "Bedrock level", "level.dat"))
	if err != nil || string(restoredDat) != "sample-level-dat-content" {
		t.Fatalf("restored data mismatch or error: %v", err)
	}
}

func TestWorldExportAndImport(t *testing.T) {
	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "server")
	worldDir := filepath.Join(serverDir, "worlds", "TestWorld")
	_ = os.MkdirAll(worldDir, 0755)
	_ = os.WriteFile(filepath.Join(worldDir, "world_icon.jpeg"), []byte("fake-jpeg-data"), 0644)

	exportPath := filepath.Join(tmpDir, "testworld.mcworld")
	err := ExportWorld(serverDir, "TestWorld", exportPath)
	if err != nil {
		t.Fatalf("ExportWorld failed: %v", err)
	}

	// Test ImportWorld
	targetServerDir := filepath.Join(tmpDir, "target_server")
	archiveBytes, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("failed to read exported archive: %v", err)
	}

	reader := bytes.NewReader(archiveBytes)
	err = ImportWorld(targetServerDir, "ImportedWorld", reader, int64(len(archiveBytes)))
	if err != nil {
		t.Fatalf("ImportWorld failed: %v", err)
	}

	importedData, err := os.ReadFile(filepath.Join(targetServerDir, "worlds", "ImportedWorld", "world_icon.jpeg"))
	if err != nil || string(importedData) != "fake-jpeg-data" {
		t.Fatalf("imported file mismatch: %v", err)
	}
}

func TestImportWorldNestedFolder(t *testing.T) {
	tmpDir := t.TempDir()

	// Create zip with enclosing folder: "MyNestedWorld/level.dat"
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	w, _ := zw.Create("MyNestedWorld/level.dat")
	_, _ = w.Write([]byte("fake-level-data"))
	w2, _ := zw.Create("MyNestedWorld/db/CURRENT")
	_, _ = w2.Write([]byte("fake-db-current"))
	_ = zw.Close()

	targetServerDir := filepath.Join(tmpDir, "srv")
	reader := bytes.NewReader(buf.Bytes())
	err := ImportWorld(targetServerDir, "Bedrock level", reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("ImportWorld failed: %v", err)
	}

	// Verify level.dat was extracted directly under worlds/Bedrock level/level.dat
	levelDatPath := filepath.Join(targetServerDir, "worlds", "Bedrock level", "level.dat")
	data, err := os.ReadFile(levelDatPath)
	if err != nil || string(data) != "fake-level-data" {
		t.Fatalf("expected level.dat at world root: %v", err)
	}

	dbCurrentPath := filepath.Join(targetServerDir, "worlds", "Bedrock level", "db", "CURRENT")
	if _, err := os.Stat(dbCurrentPath); err != nil {
		t.Fatalf("expected db/CURRENT at world root: %v", err)
	}
}

func TestZipSlipPrevention(t *testing.T) {
	tmpDir := t.TempDir()

	// Malicious archive with directory traversal
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	w, _ := zw.Create("../../evil.txt")
	_, _ = w.Write([]byte("malicious_payload"))
	w2, _ := zw.Create("legit.txt")
	_, _ = w2.Write([]byte("legit_content"))
	_ = zw.Close()

	destDir := filepath.Join(tmpDir, "extracted")
	zipPath := filepath.Join(tmpDir, "test.zip")
	_ = os.WriteFile(zipPath, buf.Bytes(), 0644)

	err := Unzip(zipPath, destDir)
	if err != nil {
		t.Fatalf("Unzip failed: %v", err)
	}

	// Verify evil.txt was NOT written outside destDir
	escapedFile := filepath.Join(tmpDir, "evil.txt")
	if _, err := os.Stat(escapedFile); !os.IsNotExist(err) {
		t.Fatalf("SECURITY VULNERABILITY: Zip slip file was written outside destination: %s", escapedFile)
	}

	// Verify legit file was extracted
	legitFile := filepath.Join(destDir, "legit.txt")
	if _, err := os.Stat(legitFile); err != nil {
		t.Fatalf("expected legit.txt to be extracted inside destination: %v", err)
	}
}

func TestWorldPathTraversalRejection(t *testing.T) {
	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "srv1")
	_ = os.MkdirAll(filepath.Join(serverDir, "worlds"), 0755)

	// Test ExportWorld with traversal
	err := ExportWorld(serverDir, "../../etc", filepath.Join(tmpDir, "out.mcworld"))
	if err == nil {
		t.Errorf("expected ExportWorld with traversal to fail")
	}

	// Test ImportWorld with traversal
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("level.dat")
	_, _ = w.Write([]byte("fake_dat"))
	_ = zw.Close()

	reader := bytes.NewReader(buf.Bytes())
	err = ImportWorld(serverDir, "../../etc", reader, int64(buf.Len()))
	if err == nil {
		t.Errorf("expected ImportWorld with traversal to fail")
	}
}

