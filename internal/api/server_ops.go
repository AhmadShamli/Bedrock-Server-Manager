package api

import (
	"context"
	"fmt"
	"log"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// WithServerStopped ensures that if the server is currently running, it is stopped gracefully
// before executing operation fn, and restarted afterwards if it was running.
func (h *ServerHandler) WithServerStopped(
	ctx context.Context,
	server *models.Server,
	actorType string,
	actorName string,
	actionName string,
	clientIP string,
	fn func() error,
) error {
	wasRunning := false
	if h.engine != nil {
		liveStatus, err := h.engine.GetServerStatus(ctx, server)
		if err == nil && (liveStatus == models.ServerStatusRunning || liveStatus == models.ServerStatusStarting) {
			wasRunning = true
		} else if server.Status == models.ServerStatusRunning || server.Status == models.ServerStatusStarting {
			wasRunning = true
		}
	} else if server.Status == models.ServerStatusRunning || server.Status == models.ServerStatusStarting {
		wasRunning = true
	}

	if wasRunning && h.engine != nil {
		log.Printf("[ServerOps] Temporarily stopping server '%s' (%s) for %s...", server.Name, server.ID, actionName)
		_ = h.engine.StopServer(ctx, server, 15)
		server.Status = models.ServerStatusStopped
		if h.db != nil {
			_ = h.db.UpdateServerStatus(ctx, server.ID, models.ServerStatusStopped, server.ContainerID)
		}
		if h.resourceMonitor != nil {
			h.resourceMonitor.ResetIdleTimer(server.ID)
		}
	}

	opErr := fn()

	if wasRunning {
		log.Printf("[ServerOps] Restarting server '%s' (%s) after %s...", server.Name, server.ID, actionName)
		if err := h.StartServerInstance(ctx, server, actorType, actorName, fmt.Sprintf("%s_restart", actionName), clientIP); err != nil {
			log.Printf("[ServerOps] Warning: failed to restart server '%s' after %s: %v", server.ID, actionName, err)
		}
	}

	return opErr
}
