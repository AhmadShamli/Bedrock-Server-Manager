package player

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/configfile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// SyncReport summarizes additions and updates applied when merging global access lists into a server instance.
type SyncReport struct {
	ServerID           string   `json:"server_id"`
	AllowlistAdded     []string `json:"allowlist_added"`
	PermissionsUpdated []string `json:"permissions_updated"`
}

// SyncServerWithGlobal merges global allowlists and permissions into an existing or newly provisioned server.
// If the server is actively running and eng is provided, it triggers zero-downtime BDS console reloads.
func SyncServerWithGlobal(
	ctx context.Context,
	dataDir string,
	serverID string,
	db *database.ManagerDB,
	eng engine.ServerEngine,
) (*SyncReport, error) {
	server, err := db.GetServer(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("server '%s' not found: %w", serverID, err)
	}

	globalPlayers, err := db.ListGlobalPlayers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve global players: %w", err)
	}

	serverDir := filepath.Join(dataDir, "servers", serverID)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to access server directory: %w", err)
	}

	report := &SyncReport{
		ServerID:           serverID,
		AllowlistAdded:     make([]string, 0),
		PermissionsUpdated: make([]string, 0),
	}

	// 1. Merge Allowlist
	allowlistPath := filepath.Join(serverDir, "allowlist.json")
	localAllowlist, _ := configfile.ReadAllowlist(allowlistPath)

	nameMap := make(map[string]bool)
	xuidMap := make(map[string]bool)
	for _, entry := range localAllowlist {
		if entry.Name != "" {
			nameMap[strings.ToLower(entry.Name)] = true
		}
		if entry.XUID != "" {
			xuidMap[entry.XUID] = true
		}
	}

	allowlistModified := false
	for _, gp := range globalPlayers {
		if !gp.IsAllowlisted {
			continue
		}

		nameKey := strings.ToLower(gp.Name)
		if (nameKey != "" && nameMap[nameKey]) || (gp.XUID != "" && xuidMap[gp.XUID]) {
			continue // Player already in allowlist
		}

		localAllowlist = append(localAllowlist, configfile.AllowlistEntry{
			Name:               gp.Name,
			XUID:               gp.XUID,
			IgnoresPlayerLimit: gp.IgnoresPlayerLimit,
		})
		if nameKey != "" {
			nameMap[nameKey] = true
		}
		if gp.XUID != "" {
			xuidMap[gp.XUID] = true
		}
		report.AllowlistAdded = append(report.AllowlistAdded, gp.Name)
		allowlistModified = true
	}

	if allowlistModified {
		if err := configfile.WriteAllowlist(allowlistPath, localAllowlist); err != nil {
			return nil, fmt.Errorf("failed to write updated allowlist: %w", err)
		}
		if eng != nil && server.Status == models.ServerStatusRunning {
			_ = eng.SendConsoleCommand(ctx, server, "allowlist reload")
		}
	}

	// 2. Merge Permissions (Ops, Members, Visitors)
	permPath := filepath.Join(serverDir, "permissions.json")
	localPerms, _ := configfile.ReadPermissions(permPath)

	permMap := make(map[string]configfile.PermissionEntry)
	for _, p := range localPerms {
		if p.XUID != "" {
			permMap[p.XUID] = p
		}
	}

	permsModified := false
	for _, gp := range globalPlayers {
		if gp.Permission == "" || gp.Permission == "none" {
			continue
		}

		// In BDS, permissions.json requires the player's XUID
		if gp.XUID != "" {
			existing, exists := permMap[gp.XUID]
			if !exists || !strings.EqualFold(existing.Permission, gp.Permission) {
				permMap[gp.XUID] = configfile.PermissionEntry{
					Permission: strings.ToLower(gp.Permission),
					XUID:       gp.XUID,
				}
				report.PermissionsUpdated = append(report.PermissionsUpdated, fmt.Sprintf("%s -> %s", gp.Name, gp.Permission))
				permsModified = true
			}
		} else if eng != nil && server.Status == models.ServerStatusRunning && strings.EqualFold(gp.Permission, "operator") {
			// If XUID is not known yet but server is live, grant op by gamertag so BDS resolves XUID
			_ = eng.SendConsoleCommand(ctx, server, fmt.Sprintf("op %s", gp.Name))
			report.PermissionsUpdated = append(report.PermissionsUpdated, fmt.Sprintf("%s -> op (via console)", gp.Name))
		}
	}

	if permsModified {
		updatedPermsList := make([]configfile.PermissionEntry, 0, len(permMap))
		for _, entry := range permMap {
			updatedPermsList = append(updatedPermsList, entry)
		}

		if err := configfile.WritePermissions(permPath, updatedPermsList); err != nil {
			return nil, fmt.Errorf("failed to write updated permissions: %w", err)
		}
		if eng != nil && server.Status == models.ServerStatusRunning {
			_ = eng.SendConsoleCommand(ctx, server, "permission reload")
		}
	}

	return report, nil
}

// SyncAllServersWithGlobal merges global access control lists into every registered server.
func SyncAllServersWithGlobal(
	ctx context.Context,
	dataDir string,
	db *database.ManagerDB,
	eng engine.ServerEngine,
) (map[string]*SyncReport, error) {
	servers, err := db.ListServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	reports := make(map[string]*SyncReport)
	for _, s := range servers {
		rep, err := SyncServerWithGlobal(ctx, dataDir, s.ID, db, eng)
		if err != nil {
			// Record error in report or skip
			reports[s.ID] = &SyncReport{
				ServerID:           s.ID,
				AllowlistAdded:     []string{},
				PermissionsUpdated: []string{fmt.Sprintf("Error: %s", err.Error())},
			}
			continue
		}
		reports[s.ID] = rep
	}

	return reports, nil
}
