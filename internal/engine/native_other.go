//go:build !windows

package engine

import (
	"context"
	"fmt"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/bds"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// NativeEngine stub for non-Windows platforms where native bedrock_server.exe cannot run.
type NativeEngine struct {
	dataDir     string
	bdsCache    *bds.CacheManager
	bdsResolver *bds.Resolver
	db          bds.SettingGetter
}

// NewNativeEngine creates a NativeEngine stub on non-Windows systems.
func NewNativeEngine(dataDir string, bdsCache *bds.CacheManager, bdsResolver *bds.Resolver, db bds.SettingGetter) (*NativeEngine, error) {
	return &NativeEngine{
		dataDir:     dataDir,
		bdsCache:    bdsCache,
		bdsResolver: bdsResolver,
		db:          db,
	}, nil
}

func (e *NativeEngine) CreateServer(ctx context.Context, server *models.Server, dataDir string) (string, error) {
	return "", fmt.Errorf("native BDS engine is only supported on Windows hosts (running bedrock_server.exe)")
}

func (e *NativeEngine) StartServer(ctx context.Context, server *models.Server) error {
	return fmt.Errorf("native BDS engine is only supported on Windows hosts (running bedrock_server.exe)")
}

func (e *NativeEngine) StopServer(ctx context.Context, server *models.Server, timeoutSeconds int) error {
	return fmt.Errorf("native BDS engine is only supported on Windows hosts (running bedrock_server.exe)")
}

func (e *NativeEngine) RestartServer(ctx context.Context, server *models.Server) error {
	return fmt.Errorf("native BDS engine is only supported on Windows hosts (running bedrock_server.exe)")
}

func (e *NativeEngine) RemoveServer(ctx context.Context, server *models.Server, removeData bool) error {
	return nil
}

func (e *NativeEngine) GetServerStatus(ctx context.Context, server *models.Server) (string, error) {
	return models.ServerStatusStopped, nil
}

func (e *NativeEngine) GetContainerStats(ctx context.Context, server *models.Server) (*models.MetricRaw, error) {
	return nil, nil
}

func (e *NativeEngine) SendConsoleCommand(ctx context.Context, server *models.Server, cmd string) error {
	return fmt.Errorf("native BDS engine is only supported on Windows hosts (running bedrock_server.exe)")
}

func (e *NativeEngine) GetRecentLogs(serverID string) []string {
	return []string{}
}

func (e *NativeEngine) SubscribeLogs(serverID string) (<-chan string, func()) {
	ch := make(chan string)
	close(ch)
	return ch, func() {}
}

func (e *NativeEngine) AttachLogCapture(serverID, containerID string) {}

func (e *NativeEngine) SetLogListener(listener func(serverID, line string)) {}

func (e *NativeEngine) ListNetworks(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

// StopAllGracefully satisfies graceful shutdown during daemon stop.
func (e *NativeEngine) StopAllGracefully(timeoutSeconds int) {}

// CheckVCRuntime stub for non-Windows platforms.
func CheckVCRuntime() error {
	return nil
}
