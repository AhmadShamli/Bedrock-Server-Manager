package clone

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/allocator"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/configfile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/player"
)

// CopyDir recursively copies directory content from src to dst.
func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return func() error {
			srcFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
			if err != nil {
				return err
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, srcFile)
			return err
		}()
	})
}

// CloneServer creates an exact clone of an existing server with an auto-allocated UDP port.
func CloneServer(
	ctx context.Context,
	srcServerID, newServerID, newName string,
	dataDir string,
	db *database.ManagerDB,
	pa *allocator.PortAllocator,
	eng engine.ServerEngine,
) (*models.Server, error) {
	srcServer, err := db.GetServer(ctx, srcServerID)
	if err != nil {
		return nil, fmt.Errorf("source server not found: %w", err)
	}

	existing, _ := db.ListServers(ctx)
	for _, s := range existing {
		if s.ID == newServerID {
			return nil, fmt.Errorf("server ID '%s' is already in use", newServerID)
		}
	}

	// Auto-allocate new ports
	p4, p6, err := pa.FindAvailablePortPair(srcServer.Port+2, existing)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate free port pair: %w", err)
	}

	srcDir := filepath.Join(dataDir, "servers", srcServerID)
	dstDir := filepath.Join(dataDir, "servers", newServerID)

	if err := CopyDir(srcDir, dstDir); err != nil {
		return nil, fmt.Errorf("failed to copy server data: %w", err)
	}

	// Adjust server.properties in new clone
	propPath := filepath.Join(dstDir, "server.properties")
	props, keys, err := configfile.ReadProperties(propPath)
	if err == nil && props != nil {
		props["server-name"] = newName
		props["server-port"] = fmt.Sprint(p4)
		props["server-portv6"] = fmt.Sprint(p6)
		_ = configfile.WriteProperties(propPath, props, keys)
	}

	clonedServer := &models.Server{
		ID:              newServerID,
		Name:            newName,
		Version:         srcServer.Version,
		Port:            p4,
		PortV6:          p6,
		Status:          models.ServerStatusStopped,
		Mode:            srcServer.Mode,
		Difficulty:      srcServer.Difficulty,
		AutostartOnBoot: false,
		PortGateEnabled: srcServer.PortGateEnabled,
		PortGateMode:    srcServer.PortGateMode,
		PortGateTimeout: srcServer.PortGateTimeout,
		MemoryLimit:     srcServer.MemoryLimit,
		CPULimit:        srcServer.CPULimit,
	}

	// Create container in engine
	cid, err := eng.CreateServer(ctx, clonedServer, dataDir)
	if err != nil {
		_ = os.RemoveAll(dstDir)
		return nil, fmt.Errorf("failed to provision cloned container: %w", err)
	}
	clonedServer.ContainerID = cid

	if err := db.CreateServer(ctx, clonedServer); err != nil {
		_ = eng.RemoveServer(ctx, clonedServer, true)
		return nil, fmt.Errorf("failed to save cloned server record: %w", err)
	}

	// Merge global allowlist and permissions into the new cloned server instance
	_, _ = player.SyncServerWithGlobal(ctx, dataDir, newServerID, db, nil)

	return clonedServer, nil
}

// ExportServer creates a zip archive of the server folder and streams it to w.
func ExportServer(serverID, dataDir string, w io.Writer) error {
	serverDir := filepath.Join(dataDir, "servers", serverID)
	if _, err := os.Stat(serverDir); os.IsNotExist(err) {
		return fmt.Errorf("server directory does not exist")
	}

	archive := zip.NewWriter(w)
	defer archive.Close()

	return filepath.Walk(serverDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(serverDir, path)
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
		header.Name = relPath

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

			_, err = io.Copy(writer, file)
			return err
		}()
	})
}
