package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// ServerEngine defines the container orchestration contract for Bedrock server instances.
type ServerEngine interface {
	CreateServer(ctx context.Context, server *models.Server, dataDir string) (string, error)
	StartServer(ctx context.Context, server *models.Server) error
	StopServer(ctx context.Context, server *models.Server, timeoutSeconds int) error
	RestartServer(ctx context.Context, server *models.Server) error
	RemoveServer(ctx context.Context, server *models.Server, removeData bool) error
	GetServerStatus(ctx context.Context, server *models.Server) (string, error)
	GetContainerStats(ctx context.Context, server *models.Server) (*models.MetricRaw, error)
	SendConsoleCommand(ctx context.Context, server *models.Server, cmd string) error
	GetRecentLogs(serverID string) []string
	SubscribeLogs(serverID string) (<-chan string, func())
	AttachLogCapture(serverID, containerID string)
	SetLogListener(listener func(serverID, line string))
	ListNetworks(ctx context.Context) ([]string, error)
}

// ParseMemoryBytes converts memory strings like "512M", "2G", "4GB" into bytes.
func ParseMemoryBytes(memStr string) (int64, error) {
	memStr = strings.TrimSpace(strings.ToUpper(memStr))
	if memStr == "" {
		return 2 * 1024 * 1024 * 1024, nil // Default 2GB
	}

	unitMultiplier := int64(1)
	cleanStr := memStr

	if strings.HasSuffix(memStr, "GB") || strings.HasSuffix(memStr, "G") {
		unitMultiplier = 1024 * 1024 * 1024
		cleanStr = strings.TrimSuffix(strings.TrimSuffix(memStr, "GB"), "G")
	} else if strings.HasSuffix(memStr, "MB") || strings.HasSuffix(memStr, "M") {
		unitMultiplier = 1024 * 1024
		cleanStr = strings.TrimSuffix(strings.TrimSuffix(memStr, "MB"), "M")
	} else if strings.HasSuffix(memStr, "KB") || strings.HasSuffix(memStr, "K") {
		unitMultiplier = 1024
		cleanStr = strings.TrimSuffix(strings.TrimSuffix(memStr, "KB"), "K")
	}

	val, err := strconv.ParseFloat(cleanStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory limit '%s': %w", memStr, err)
	}

	return int64(val * float64(unitMultiplier)), nil
}
