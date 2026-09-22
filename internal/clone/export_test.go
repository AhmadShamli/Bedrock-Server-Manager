package clone

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/allocator"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func TestCloneAndExport(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}
	defer db.Close()

	pa := allocator.NewPortAllocator()
	eng := engine.NewMockEngine()

	// 1. Setup source server
	srcServer := &models.Server{
		ID:          "source-srv",
		Name:        "Original World",
		Version:     "latest",
		Port:        19132,
		PortV6:      19133,
		Status:      models.ServerStatusStopped,
		MemoryLimit: "2G",
		CPULimit:    2.0,
	}
	_ = db.CreateServer(ctx, srcServer)

	// Create source files
	srcDir := filepath.Join(tempDir, "servers", "source-srv")
	_ = os.MkdirAll(srcDir, 0755)
	_ = os.WriteFile(filepath.Join(srcDir, "server.properties"), []byte("server-name=Original World\nserver-port=19132\n"), 0644)

	// 2. Clone Server
	cloned, err := CloneServer(ctx, "source-srv", "cloned-srv", "Cloned World", tempDir, db, pa, eng)
	if err != nil {
		t.Fatalf("CloneServer failed: %v", err)
	}

	if cloned.ID != "cloned-srv" || cloned.Name != "Cloned World" {
		t.Errorf("unexpected cloned server: %+v", cloned)
	}
	if cloned.Port == 19132 {
		t.Errorf("expected new allocated port, got %d", cloned.Port)
	}

	// Verify cloned files
	dstProp := filepath.Join(tempDir, "servers", "cloned-srv", "server.properties")
	if _, err := os.Stat(dstProp); os.IsNotExist(err) {
		t.Errorf("cloned properties file does not exist")
	}

	// 3. Export Server to zip
	var buf bytes.Buffer
	if err := ExportServer("cloned-srv", tempDir, &buf); err != nil {
		t.Fatalf("ExportServer failed: %v", err)
	}

	// Verify zip contents
	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("failed to read exported zip: %v", err)
	}

	foundProp := false
	for _, f := range zipReader.File {
		if f.Name == "server.properties" {
			foundProp = true
		}
	}
	if !foundProp {
		t.Errorf("expected server.properties inside exported zip archive")
	}
}
