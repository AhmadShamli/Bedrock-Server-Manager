package engine

import (
	"context"
	"log"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// BootManager inspects registered servers on manager launch and starts instances configured for autostart.
type BootManager struct {
	db     *database.ManagerDB
	engine ServerEngine
}

// NewBootManager creates a BootManager.
func NewBootManager(db *database.ManagerDB, engine ServerEngine) *BootManager {
	return &BootManager{
		db:     db,
		engine: engine,
	}
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
			continue
		}

		log.Printf("[BootManager] Autostarting server '%s' (%s)...", s.Name, s.ID)
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
