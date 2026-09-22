package engine

import (
	"context"
	"log"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// PreStartHook defines a hook invoked prior to starting a server on boot.
type PreStartHook func(ctx context.Context, serverID string) error

// BootManager inspects registered servers on manager launch and starts instances configured for autostart.
type BootManager struct {
	db           *database.ManagerDB
	engine       ServerEngine
	preStartHook PreStartHook
}

// NewBootManager creates a BootManager with an optional pre-start hook.
func NewBootManager(db *database.ManagerDB, engine ServerEngine, preStart ...PreStartHook) *BootManager {
	bm := &BootManager{
		db:     db,
		engine: engine,
	}
	if len(preStart) > 0 {
		bm.preStartHook = preStart[0]
	}
	return bm
}

// AutostartServers launches all servers marked with autostart_on_boot = true.
func (bm *BootManager) AutostartServers(ctx context.Context) ([]string, error) {
	servers, err := bm.db.ListServers(ctx)
	if err != nil {
		return nil, err
	}

	var started []string
	for _, s := range servers {
		if !s.AutostartOnBoot {
			continue
		}

		status, err := bm.engine.GetServerStatus(ctx, &s)
		if err == nil && status == models.ServerStatusRunning {
			log.Printf("[BootManager] Server '%s' (%s) is already running.", s.Name, s.ID)
			bm.engine.AttachLogCapture(s.ID, s.ContainerID)
			continue
		}

		log.Printf("[BootManager] Autostarting server '%s' (%s)...", s.Name, s.ID)
		if bm.preStartHook != nil {
			if err := bm.preStartHook(ctx, s.ID); err != nil {
				log.Printf("[BootManager] Pre-start sync warning for server '%s': %v", s.ID, err)
			}
		}
		if err := bm.engine.StartServer(ctx, &s); err != nil {
			log.Printf("[BootManager] Failed to autostart server '%s': %v", s.ID, err)
			_ = bm.db.UpdateServerStatus(ctx, s.ID, models.ServerStatusCrashed, s.ContainerID)
			continue
		}

		_ = bm.db.UpdateServerStatus(ctx, s.ID, models.ServerStatusRunning, s.ContainerID)
		_ = bm.db.CreateAuditLog(ctx, &models.AuditLog{
			ActorType: "system",
			ActorName: "BootManager",
			Action:    "server_autostart",
			Target:    s.ID,
			Details:   `{"status": "running"}`,
		})
		started = append(started, s.ID)
	}

	return started, nil
}
