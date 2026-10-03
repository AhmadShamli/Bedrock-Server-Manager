package resourcemonitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// Default settings
const (
	DefaultThresholdPercent = 85.0
	DefaultAlertInterval    = 1 * time.Minute
)

// CommandSender executes commands against the server console.
type CommandSender interface {
	SendConsoleCommand(ctx context.Context, server *models.Server, cmd string) error
}

// ChatBroadcaster posts chat messages to server chat feeds.
type ChatBroadcaster interface {
	AddChatMessage(serverID, gamertag, message string)
}

// DBStore abstracts manager database operations for settings and audit logs.
type DBStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	CreateAuditLog(ctx context.Context, log *models.AuditLog) error
	GetUserByID(ctx context.Context, id int64) (*models.User, error)
}

// WebhookSender dispatches alerts to outbound webhooks (e.g. Discord).
type WebhookSender interface {
	NotifyResourceAlert(ctx context.Context, webhookURL, serverName, serverID string, ramUsed, ramLimit int64, ramPct float64, cpuUsed, cpuLimitCores, cpuPct float64) error
}

// Config configures the resource monitoring thresholds and intervals.
type Config struct {
	ThresholdPercent float64       // Usage percentage (e.g. 85.0 = 85%) considered nearly full
	AlertInterval    time.Duration // Minimum interval between automated alert broadcasts (default 1m)
	Enabled          bool
}

// ResourceStatus captures evaluated resource consumption against allocated limits.
type ResourceStatus struct {
	ServerID          string    `json:"server_id"`
	ServerName        string    `json:"server_name"`
	RAMUsedBytes      int64     `json:"ram_used_bytes"`
	RAMAllocatedBytes int64     `json:"ram_allocated_bytes"`
	RAMPercent        float64   `json:"ram_percent"`
	CPUUsedPercent    float64   `json:"cpu_used_percent"`
	CPUAllocatedCores float64   `json:"cpu_allocated_cores"`
	CPUPercent        float64   `json:"cpu_percent"` // CPU used as percentage of allocated cores
	IsNearlyFull      bool      `json:"is_nearly_full"`
	TriggeredResource string    `json:"triggered_resource,omitempty"` // "ram", "cpu", or "both"
	ThresholdPercent  float64   `json:"threshold_percent"`
	LastAlertTime     time.Time `json:"last_alert_time,omitempty"`
}

// ResourceMonitor watches containerized server metrics against allocated quotas and broadcasts alerts.
type ResourceMonitor struct {
	engine          CommandSender
	chatBroadcaster ChatBroadcaster
	db              DBStore
	webhook         WebhookSender
	cfg             Config

	mu           sync.RWMutex
	lastAlert    map[string]time.Time
	latestStatus map[string]*ResourceStatus
}

// NewResourceMonitor creates a new ResourceMonitor instance.
func NewResourceMonitor(
	eng CommandSender,
	chat ChatBroadcaster,
	db DBStore,
	webhook WebhookSender,
	cfgs ...Config,
) *ResourceMonitor {
	c := Config{
		ThresholdPercent: DefaultThresholdPercent,
		AlertInterval:    DefaultAlertInterval,
		Enabled:          true,
	}
	if len(cfgs) > 0 {
		if cfgs[0].ThresholdPercent > 0 {
			c.ThresholdPercent = cfgs[0].ThresholdPercent
		}
		if cfgs[0].AlertInterval > 0 {
			c.AlertInterval = cfgs[0].AlertInterval
		}
		c.Enabled = cfgs[0].Enabled
	}

	return &ResourceMonitor{
		engine:          eng,
		chatBroadcaster: chat,
		db:              db,
		webhook:         webhook,
		cfg:             c,
		lastAlert:       make(map[string]time.Time),
		latestStatus:    make(map[string]*ResourceStatus),
	}
}

// FormatBytes formats byte counts into human-readable strings (e.g. "1.82 GB", "512.00 MB").
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// resolveThreshold determines the active threshold percent from DB, environment, or configuration.
func (rm *ResourceMonitor) resolveThreshold(ctx context.Context) float64 {
	if rm.db != nil {
		if val, err := rm.db.GetSetting(ctx, "resource_alert_threshold"); err == nil && strings.TrimSpace(val) != "" {
			if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil && f > 0 {
				if f <= 1.0 {
					return f * 100.0
				}
				return f
			}
		}
	}

	if env := os.Getenv("RESOURCE_ALERT_THRESHOLD"); env != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(env), 64); err == nil && f > 0 {
			if f <= 1.0 {
				return f * 100.0
			}
			return f
		}
	}

	if rm.cfg.ThresholdPercent > 0 {
		return rm.cfg.ThresholdPercent
	}
	return DefaultThresholdPercent
}

// resolveInterval determines the alert cooldown interval from DB, environment, or configuration.
func (rm *ResourceMonitor) resolveInterval(ctx context.Context) time.Duration {
	if rm.db != nil {
		if val, err := rm.db.GetSetting(ctx, "resource_alert_interval_seconds"); err == nil && strings.TrimSpace(val) != "" {
			if sec, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && sec > 0 {
				return time.Duration(sec) * time.Second
			}
		}
	}

	if env := os.Getenv("RESOURCE_ALERT_INTERVAL"); env != "" {
		if sec, err := strconv.Atoi(strings.TrimSpace(env)); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
		if d, err := time.ParseDuration(env); err == nil && d > 0 {
			return d
		}
	}

	if rm.cfg.AlertInterval > 0 {
		return rm.cfg.AlertInterval
	}
	return DefaultAlertInterval
}

// CheckServer evaluates a server's current RAM and CPU usage against allocated limits.
// If allocated resources are nearly full and the cooldown period (1 minute) has elapsed,
// it automatically broadcasts alerts to connected players, chat feed, and configured webhooks.
func (rm *ResourceMonitor) CheckServer(
	ctx context.Context,
	server *models.Server,
	cpuPercent float64,
	ramBytes int64,
) (*ResourceStatus, bool, error) {
	if server == nil || !rm.cfg.Enabled {
		return nil, false, nil
	}

	// 1. Calculate allocated RAM
	allocatedRAM, err := engine.ParseMemoryBytes(server.MemoryLimit)
	if err != nil || allocatedRAM <= 0 {
		allocatedRAM = 2 * 1024 * 1024 * 1024 // Default 2GB
	}

	// 2. Calculate allocated CPU (cores)
	allocatedCPUCores := server.CPULimit
	if allocatedCPUCores <= 0 {
		allocatedCPUCores = 2.0 // Default 2.0 cores
	}
	allocatedCPUPercent := allocatedCPUCores * 100.0

	// 3. Compute usage ratios against allocated limits
	ramPercent := 0.0
	if allocatedRAM > 0 {
		ramPercent = (float64(ramBytes) / float64(allocatedRAM)) * 100.0
	}

	cpuRatioPercent := 0.0
	if allocatedCPUPercent > 0 {
		cpuRatioPercent = (cpuPercent / allocatedCPUPercent) * 100.0
	}

	threshold := rm.resolveThreshold(ctx)
	interval := rm.resolveInterval(ctx)

	isRAMFull := ramPercent >= threshold
	isCPUFull := cpuRatioPercent >= threshold
	isNearlyFull := isRAMFull || isCPUFull

	var triggeredResource string
	if isRAMFull && isCPUFull {
		triggeredResource = "both"
	} else if isRAMFull {
		triggeredResource = "ram"
	} else if isCPUFull {
		triggeredResource = "cpu"
	}

	status := &ResourceStatus{
		ServerID:          server.ID,
		ServerName:        server.Name,
		RAMUsedBytes:      ramBytes,
		RAMAllocatedBytes: allocatedRAM,
		RAMPercent:        ramPercent,
		CPUUsedPercent:    cpuPercent,
		CPUAllocatedCores: allocatedCPUCores,
		CPUPercent:        cpuRatioPercent,
		IsNearlyFull:      isNearlyFull,
		TriggeredResource: triggeredResource,
		ThresholdPercent:  threshold,
	}

	rm.mu.Lock()
	if last, ok := rm.lastAlert[server.ID]; ok {
		status.LastAlertTime = last
	}
	rm.latestStatus[server.ID] = status
	rm.mu.Unlock()

	if !isNearlyFull {
		return status, false, nil
	}

	// Check 1-minute alert cooldown
	rm.mu.Lock()
	lastAlert, hasAlerted := rm.lastAlert[server.ID]
	if hasAlerted && time.Since(lastAlert) < interval {
		rm.mu.Unlock()
		return status, false, nil
	}
	now := time.Now()
	rm.lastAlert[server.ID] = now
	status.LastAlertTime = now
	rm.mu.Unlock()

	// Broadcast alert to in-game users, player hub chat feed, Discord, and audit logs
	broadcastErr := rm.broadcastAlert(ctx, server, status)
	return status, true, broadcastErr
}

// FormatAlertMessage formats a human-readable alert message displaying RAM and CPU usage vs allocated.
func (rm *ResourceMonitor) FormatAlertMessage(status *ResourceStatus) string {
	return fmt.Sprintf(
		"[RESOURCE ALERT] Server resources nearly full! RAM: %s / %s (%.1f%%) | CPU: %.1f%% / %.1f%% (%.1f%%)",
		FormatBytes(status.RAMUsedBytes),
		FormatBytes(status.RAMAllocatedBytes),
		status.RAMPercent,
		status.CPUUsedPercent,
		status.CPUAllocatedCores*100.0,
		status.CPUPercent,
	)
}

// FormatTellrawCommand constructs a Minecraft Bedrock tellraw command broadcasting to all connected players.
func (rm *ResourceMonitor) FormatTellrawCommand(status *ResourceStatus) (string, error) {
	formattedText := fmt.Sprintf(
		"§c§l[RESOURCE ALERT]§r §eAllocated resources nearly full!§r §fRAM: §c%s / %s (%.1f%%)§r | §fCPU: §c%.1f%% / %.1f%% (%.1f%%)§r",
		FormatBytes(status.RAMUsedBytes),
		FormatBytes(status.RAMAllocatedBytes),
		status.RAMPercent,
		status.CPUUsedPercent,
		status.CPUAllocatedCores*100.0,
		status.CPUPercent,
	)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]interface{}{
		"rawtext": []map[string]string{
			{"text": formattedText},
		},
	}); err != nil {
		return "", err
	}

	return fmt.Sprintf("tellraw @a %s", strings.TrimSpace(buf.String())), nil
}

// broadcastAlert sends alerts across all communication channels.
func (rm *ResourceMonitor) broadcastAlert(ctx context.Context, server *models.Server, status *ResourceStatus) error {
	alertMsg := rm.FormatAlertMessage(status)
	log.Printf("[ResourceMonitor] %s: Server '%s' (%s)", alertMsg, server.Name, server.ID)

	var sendErr error

	// 1. Send in-game broadcast via tellraw @a (with fallback to say)
	if rm.engine != nil {
		tellrawCmd, err := rm.FormatTellrawCommand(status)
		if err == nil {
			if err := rm.engine.SendConsoleCommand(ctx, server, tellrawCmd); err != nil {
				// Fallback to standard say command if tellraw encounters issues
				sayCmd := fmt.Sprintf("say %s", alertMsg)
				if fallbackErr := rm.engine.SendConsoleCommand(ctx, server, sayCmd); fallbackErr != nil {
					sendErr = fmt.Errorf("in-game broadcast failed: %w", fallbackErr)
				}
			}
		} else {
			sayCmd := fmt.Sprintf("say %s", alertMsg)
			_ = rm.engine.SendConsoleCommand(ctx, server, sayCmd)
		}
	}

	// 2. Post alert to Player Hub live chat feed
	if rm.chatBroadcaster != nil {
		rm.chatBroadcaster.AddChatMessage(server.ID, "SYSTEM", alertMsg)
	}

	// 3. Dispatch outbound webhook (e.g. Discord) if configured
	if rm.webhook != nil && rm.db != nil {
		if webhookURL, err := rm.db.GetSetting(ctx, "discord_webhook_url"); err == nil && webhookURL != "" {
			_ = rm.webhook.NotifyResourceAlert(
				ctx,
				webhookURL,
				server.Name,
				server.ID,
				status.RAMUsedBytes,
				status.RAMAllocatedBytes,
				status.RAMPercent,
				status.CPUUsedPercent,
				status.CPUAllocatedCores,
				status.CPUPercent,
			)
		}
	}

	// 4. Record audit log
	if rm.db != nil {
		detailsMap := map[string]interface{}{
			"server_id":           server.ID,
			"server_name":         server.Name,
			"ram_used_bytes":      status.RAMUsedBytes,
			"ram_allocated_bytes": status.RAMAllocatedBytes,
			"ram_percent":         status.RAMPercent,
			"cpu_used_percent":    status.CPUUsedPercent,
			"cpu_allocated_cores": status.CPUAllocatedCores,
			"cpu_percent":         status.CPUPercent,
			"threshold_percent":   status.ThresholdPercent,
			"triggered_resource":  status.TriggeredResource,
		}
		detailsJSON, _ := json.Marshal(detailsMap)

		_ = rm.db.CreateAuditLog(ctx, &models.AuditLog{
			ActorType: "system",
			ActorName: "ResourceMonitor",
			Action:    "resource_alert",
			Target:    server.ID,
			Details:   string(detailsJSON),
			ClientIP:  "127.0.0.1",
			Timestamp: time.Now().UTC(),
		})
	}

	return sendErr
}

// GetServerResourceStatus returns the latest evaluated status for a server.
func (rm *ResourceMonitor) GetServerResourceStatus(serverID string) *ResourceStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.latestStatus[serverID]
}

// GetAllResourceStatuses returns a snapshot of all tracked server statuses.
func (rm *ResourceMonitor) GetAllResourceStatuses() map[string]*ResourceStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	out := make(map[string]*ResourceStatus, len(rm.latestStatus))
	for k, v := range rm.latestStatus {
		statusCopy := *v
		out[k] = &statusCopy
	}
	return out
}

// ResetAlertCooldown clears the alert cooldown timestamp for a server.
func (rm *ResourceMonitor) ResetAlertCooldown(serverID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.lastAlert, serverID)
}

// GetLastAlertTime returns the timestamp of the last sent alert for a server.
func (rm *ResourceMonitor) GetLastAlertTime(serverID string) time.Time {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.lastAlert[serverID]
}
