package bds

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// ProtectedConfigFiles lists file names that must never be overwritten during an update.
var ProtectedConfigFiles = map[string]bool{
	"server.properties": true,
	"allowlist.json":    true,
	"whitelist.json":    true,
	"permissions.json":  true,
}

// InstallServerFiles copies BDS binaries and standard asset directories from bdsCacheDir into serverDir.
// If isUpdate is true, user configurations (server.properties, allowlist, permissions) and worlds are preserved.
func InstallServerFiles(serverDir, bdsCacheDir string, isUpdate bool) error {
	if err := os.MkdirAll(serverDir, 0750); err != nil {
		return fmt.Errorf("failed to create server dir: %w", err)
	}

	entries, err := os.ReadDir(bdsCacheDir)
	if err != nil {
		return fmt.Errorf("failed to read BDS cache dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == ".complete" {
			continue
		}

		srcPath := filepath.Join(bdsCacheDir, name)
		dstPath := filepath.Join(serverDir, name)

		if entry.IsDir() {
			// Subdirectories like definitions, resource_packs, behavior_packs
			if isUpdate && strings.EqualFold(name, "worlds") {
				// Never touch existing worlds folder during BDS update
				continue
			}
			if err := copyDirectory(srcPath, dstPath, isUpdate); err != nil {
				return fmt.Errorf("failed copying directory %s: %w", name, err)
			}
		} else {
			// Files
			lowerName := strings.ToLower(name)
			if isUpdate && ProtectedConfigFiles[lowerName] {
				// Don't overwrite existing config files if they already exist
				if _, err := os.Stat(dstPath); err == nil {
					continue
				}
			}

			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed copying file %s: %w", name, err)
			}
		}
	}

	log.Printf("[BDS] Successfully installed BDS binaries from %s into %s (isUpdate=%v)", bdsCacheDir, serverDir, isUpdate)
	return nil
}

// RollbackServerFiles restores binaries and assets from previousCacheDir into serverDir, preserving worlds and configs.
func RollbackServerFiles(serverDir, previousCacheDir string) error {
	log.Printf("[BDS] Initiating BDS rollback to %s for server at %s", previousCacheDir, serverDir)
	return InstallServerFiles(serverDir, previousCacheDir, true)
}

func copyDirectory(srcDir, dstDir string, isUpdate bool) error {
	if err := os.MkdirAll(dstDir, 0750); err != nil {
		return err
	}

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

		dstPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0750)
		}

		lowerName := strings.ToLower(info.Name())
		if isUpdate && ProtectedConfigFiles[lowerName] {
			if _, statErr := os.Stat(dstPath); statErr == nil {
				return nil // Keep existing config file
			}
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	srcInfo, err := in.Stat()
	if err != nil {
		return err
	}

	mode := srcInfo.Mode()
	if strings.HasSuffix(strings.ToLower(src), ".exe") {
		mode = 0750
	} else if mode == 0 {
		mode = 0640
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
