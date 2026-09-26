package configfile

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestReadWorldSeedFromLevelDat(t *testing.T) {
	tempDir := t.TempDir()
	levelDatPath := filepath.Join(tempDir, "level.dat")

	// Construct mock Bedrock level.dat
	// 8-byte header: 4 bytes version, 4 bytes length
	var buf []byte
	header := make([]byte, 8)
	binary.LittleEndian.PutUint32(header[0:4], 10)
	binary.LittleEndian.PutUint32(header[4:8], 100)
	buf = append(buf, header...)

	// Add random preceding bytes
	buf = append(buf, []byte{0x0A, 0x00, 0x00}...) // Compound tag

	// Add TAG_Long RandomSeed
	expectedSeed := int64(-5492817491028471928)
	tag := []byte{0x04, 0x0a, 0x00, 'R', 'a', 'n', 'd', 'o', 'm', 'S', 'e', 'e', 'd'}
	buf = append(buf, tag...)

	seedBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(seedBytes, uint64(expectedSeed))
	buf = append(buf, seedBytes...)

	// Write to file
	if err := os.WriteFile(levelDatPath, buf, 0644); err != nil {
		t.Fatalf("failed to write test level.dat: %v", err)
	}

	seed, err := ReadWorldSeedFromLevelDat(levelDatPath)
	if err != nil {
		t.Fatalf("ReadWorldSeedFromLevelDat failed: %v", err)
	}
	if seed != expectedSeed {
		t.Fatalf("expected seed %d, got %d", expectedSeed, seed)
	}
}

func TestDetectServerSeed(t *testing.T) {
	tempDir := t.TempDir()
	serverID := "srv-seed-test"

	// 1. Without properties or level.dat -> ""
	if s := DetectServerSeed(tempDir, serverID); s != "" {
		t.Errorf("expected empty seed, got %s", s)
	}

	// 2. With level.dat in worlds/Bedrock level/
	worldDir := filepath.Join(tempDir, "servers", serverID, "worlds", "Bedrock level")
	if err := os.MkdirAll(worldDir, 0755); err != nil {
		t.Fatalf("failed to create world dir: %v", err)
	}
	levelDatPath := filepath.Join(worldDir, "level.dat")

	var buf []byte
	buf = append(buf, make([]byte, 8)...)
	buf = append(buf, []byte{0x04, 0x0a, 0x00, 'R', 'a', 'n', 'd', 'o', 'm', 'S', 'e', 'e', 'd'}...)
	seedBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(seedBytes, uint64(9876543210))
	buf = append(buf, seedBytes...)
	_ = os.WriteFile(levelDatPath, buf, 0644)

	if s := DetectServerSeed(tempDir, serverID); s != "9876543210" {
		t.Errorf("expected seed '9876543210', got '%s'", s)
	}

	// 3. With server.properties having level-seed set, it takes priority
	propPath := filepath.Join(tempDir, "servers", serverID, "server.properties")
	_ = os.WriteFile(propPath, []byte("level-seed=explicit-seed-123\n"), 0644)

	if s := DetectServerSeed(tempDir, serverID); s != "explicit-seed-123" {
		t.Errorf("expected 'explicit-seed-123', got '%s'", s)
	}
}
