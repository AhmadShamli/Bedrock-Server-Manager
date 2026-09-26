package configfile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// ReadWorldSeedFromLevelDat attempts to extract the RandomSeed from a Bedrock level.dat file.
// In Minecraft Bedrock Edition, level.dat begins with an 8-byte header (storage version + payload length)
// followed by uncompressed Little-Endian NBT data. The TAG_Long named "RandomSeed" is stored as:
// [0x04 (TAG_Long)] [0x0a, 0x00 (length 10)] ["RandomSeed"] [8-byte int64 LE payload]
func ReadWorldSeedFromLevelDat(levelDatPath string) (int64, error) {
	data, err := os.ReadFile(levelDatPath)
	if err != nil {
		return 0, err
	}

	needle := []byte{0x04, 0x0a, 0x00, 'R', 'a', 'n', 'd', 'o', 'm', 'S', 'e', 'e', 'd'}
	idx := bytes.Index(data, needle)
	if idx == -1 {
		return 0, fmt.Errorf("RandomSeed tag not found in level.dat")
	}

	valOffset := idx + len(needle)
	if len(data) < valOffset+8 {
		return 0, fmt.Errorf("truncated level.dat: not enough bytes for RandomSeed value")
	}

	seedUint := binary.LittleEndian.Uint64(data[valOffset : valOffset+8])
	return int64(seedUint), nil
}

// DetectServerSeed inspects server.properties and worlds/*/level.dat to find the world seed.
func DetectServerSeed(dataDir, serverID string) string {
	// 1. Check server.properties for level-seed
	propPath := filepath.Join(dataDir, "servers", serverID, "server.properties")
	if props, _, err := ReadProperties(propPath); err == nil {
		if s, ok := props["level-seed"]; ok && s != "" {
			return s
		}
	}

	// 2. Scan worlds directory for any level.dat
	worldsDir := filepath.Join(dataDir, "servers", serverID, "worlds")
	entries, err := os.ReadDir(worldsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				levelDat := filepath.Join(worldsDir, entry.Name(), "level.dat")
				if seed, err := ReadWorldSeedFromLevelDat(levelDat); err == nil {
					return strconv.FormatInt(seed, 10)
				}
			}
		}
	}

	return ""
}
