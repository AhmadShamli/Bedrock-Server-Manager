package clone

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

// CopyConfigOptions configures which files and settings to sync between servers.
type CopyConfigOptions struct {
	TargetServerIDs []string `json:"target_server_ids"`
	CopyAllowlist   bool     `json:"copy_allowlist"`
	CopyPermissions bool     `json:"copy_permissions"`
	CopyProperties  bool     `json:"copy_properties"`
	Mode            string   `json:"mode"` // "replace" (default) or "merge"
}

// TargetCopyStatus reports the sync status for a single target server.
type TargetCopyStatus struct {
	ServerID string   `json:"server_id"`
	Success  bool     `json:"success"`
	Error    string   `json:"error,omitempty"`
	Copied   []string `json:"copied"`
}

// CopyConfigResult encapsulates the outcome of a batch config sync.
type CopyConfigResult struct {
	SourceServerID string             `json:"source_server_id"`
	Results        []TargetCopyStatus `json:"results"`
}

// CopyConfigs copies or merges security and configuration files from a source server to target servers.
func CopyConfigs(
	ctx context.Context,
	srcServerID string,
	opts CopyConfigOptions,
	dataDir string,
	db *database.ManagerDB,
	eng engine.ServerEngine,
) (*CopyConfigResult, error) {
	if len(opts.TargetServerIDs) == 0 {
		return nil, fmt.Errorf("at least one target server ID must be specified")
	}

	if !opts.CopyAllowlist && !opts.CopyPermissions && !opts.CopyProperties {
		return nil, fmt.Errorf("at least one configuration type (allowlist, permissions, properties) must be selected to copy")
	}

	// Verify source server exists
	if _, err := db.GetServer(ctx, srcServerID); err != nil {
		return nil, fmt.Errorf("source server '%s' not found: %w", srcServerID, err)
	}

	srcDir := filepath.Join(dataDir, "servers", srcServerID)

	// Pre-load source configuration data
	var (
		srcAllowlist   []configfile.AllowlistEntry
		srcPermissions []configfile.PermissionEntry
		srcProps       map[string]string
		srcPropKeys    []string
		err            error
	)

	if opts.CopyAllowlist {
		srcAllowlistPath := filepath.Join(srcDir, "allowlist.json")
		srcAllowlist, err = configfile.ReadAllowlist(srcAllowlistPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read source allowlist: %w", err)
		}
	}

	if opts.CopyPermissions {
		srcPermPath := filepath.Join(srcDir, "permissions.json")
		srcPermissions, err = configfile.ReadPermissions(srcPermPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read source permissions: %w", err)
		}
	}

	if opts.CopyProperties {
		srcPropPath := filepath.Join(srcDir, "server.properties")
		srcProps, srcPropKeys, err = configfile.ReadProperties(srcPropPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read source server.properties: %w", err)
		}
	}

	isMerge := strings.EqualFold(opts.Mode, "merge")
	result := &CopyConfigResult{
		SourceServerID: srcServerID,
		Results:        make([]TargetCopyStatus, 0, len(opts.TargetServerIDs)),
	}

	for _, tgtID := range opts.TargetServerIDs {
		status := TargetCopyStatus{
			ServerID: tgtID,
			Copied:   make([]string, 0),
		}

		if tgtID == srcServerID {
			status.Success = false
			status.Error = "cannot copy configuration to source server itself"
			result.Results = append(result.Results, status)
			continue
		}

		tgtServer, err := db.GetServer(ctx, tgtID)
		if err != nil {
			status.Success = false
			status.Error = fmt.Sprintf("target server not found: %s", err.Error())
			result.Results = append(result.Results, status)
			continue
		}

		tgtDir := filepath.Join(dataDir, "servers", tgtID)
		if err := os.MkdirAll(tgtDir, 0755); err != nil {
			status.Success = false
			status.Error = fmt.Sprintf("failed to create target server directory: %s", err.Error())
			result.Results = append(result.Results, status)
			continue
		}

		targetFailed := false

		// 1. Copy / Merge Allowlist
		if opts.CopyAllowlist {
			tgtAllowlistPath := filepath.Join(tgtDir, "allowlist.json")
			finalAllowlist := srcAllowlist

			if isMerge {
				existingList, _ := configfile.ReadAllowlist(tgtAllowlistPath)
				nameMap := make(map[string]bool)
				xuidMap := make(map[string]bool)

				for _, entry := range existingList {
					if entry.Name != "" {
						nameMap[strings.ToLower(entry.Name)] = true
					}
					if entry.XUID != "" {
						xuidMap[entry.XUID] = true
					}
				}

				merged := append([]configfile.AllowlistEntry{}, existingList...)
				for _, srcEntry := range srcAllowlist {
					nameKey := strings.ToLower(srcEntry.Name)
					if (nameKey != "" && nameMap[nameKey]) || (srcEntry.XUID != "" && xuidMap[srcEntry.XUID]) {
						continue
					}
					merged = append(merged, srcEntry)
					if nameKey != "" {
						nameMap[nameKey] = true
					}
					if srcEntry.XUID != "" {
						xuidMap[srcEntry.XUID] = true
					}
				}
				finalAllowlist = merged
			}

			if err := configfile.WriteAllowlist(tgtAllowlistPath, finalAllowlist); err != nil {
				status.Success = false
				status.Error = fmt.Sprintf("failed to write allowlist: %s", err.Error())
				targetFailed = true
			} else {
				status.Copied = append(status.Copied, "allowlist.json")
				// Live-reload running server allowlist without needing reboot
				if eng != nil && tgtServer.Status == models.ServerStatusRunning {
					_ = eng.SendConsoleCommand(ctx, tgtServer, "allowlist reload")
				}
			}
		}

		if targetFailed {
			result.Results = append(result.Results, status)
			continue
		}

		// 2. Copy / Merge Permissions (Ops)
		if opts.CopyPermissions {
			tgtPermPath := filepath.Join(tgtDir, "permissions.json")
			finalPerms := srcPermissions

			if isMerge {
				existingPerms, _ := configfile.ReadPermissions(tgtPermPath)
				permMap := make(map[string]configfile.PermissionEntry)

				for _, entry := range existingPerms {
					key := entry.XUID
					if key == "" {
						key = strings.ToLower(entry.Permission)
					}
					permMap[key] = entry
				}

				// Source entries take precedence or append
				for _, srcEntry := range srcPermissions {
					key := srcEntry.XUID
					if key == "" {
						key = strings.ToLower(srcEntry.Permission)
					}
					permMap[key] = srcEntry
				}

				merged := make([]configfile.PermissionEntry, 0, len(permMap))
				for _, entry := range permMap {
					merged = append(merged, entry)
				}
				finalPerms = merged
			}

			if err := configfile.WritePermissions(tgtPermPath, finalPerms); err != nil {
				status.Success = false
				status.Error = fmt.Sprintf("failed to write permissions: %s", err.Error())
				targetFailed = true
			} else {
				status.Copied = append(status.Copied, "permissions.json")
				if eng != nil && tgtServer.Status == models.ServerStatusRunning {
					_ = eng.SendConsoleCommand(ctx, tgtServer, "permission reload")
				}
			}
		}

		if targetFailed {
			result.Results = append(result.Results, status)
			continue
		}

		// 3. Copy Properties (Safeguarding ports, server-name, and level-name)
		if opts.CopyProperties {
			tgtPropPath := filepath.Join(tgtDir, "server.properties")
			tgtExistingProps, tgtExistingKeys, _ := configfile.ReadProperties(tgtPropPath)

			// Start with source properties
			mergedProps := make(map[string]string)
			mergedKeys := make([]string, 0)
			keySet := make(map[string]bool)

			// Add source keys
			for _, k := range srcPropKeys {
				mergedProps[k] = srcProps[k]
				mergedKeys = append(mergedKeys, k)
				keySet[k] = true
			}

			// Add existing target keys that might not exist in source
			for _, k := range tgtExistingKeys {
				if !keySet[k] {
					mergedProps[k] = tgtExistingProps[k]
					mergedKeys = append(mergedKeys, k)
					keySet[k] = true
				}
			}

			// CRITICAL: Protect target-specific networking and world identity
			if val, ok := tgtExistingProps["server-port"]; ok && val != "" {
				mergedProps["server-port"] = val
			} else {
				mergedProps["server-port"] = fmt.Sprint(tgtServer.Port)
			}

			if val, ok := tgtExistingProps["server-portv6"]; ok && val != "" {
				mergedProps["server-portv6"] = val
			} else if tgtServer.PortV6 > 0 {
				mergedProps["server-portv6"] = fmt.Sprint(tgtServer.PortV6)
			}

			if val, ok := tgtExistingProps["server-name"]; ok && val != "" {
				mergedProps["server-name"] = val
			} else {
				mergedProps["server-name"] = tgtServer.Name
			}

			if val, ok := tgtExistingProps["level-name"]; ok && val != "" {
				mergedProps["level-name"] = val
			}

			if err := configfile.WriteProperties(tgtPropPath, mergedProps, mergedKeys); err != nil {
				status.Success = false
				status.Error = fmt.Sprintf("failed to write server.properties: %s", err.Error())
				targetFailed = true
			} else {
				status.Copied = append(status.Copied, "server.properties")
			}
		}

		if !targetFailed {
			status.Success = true
		}
		result.Results = append(result.Results, status)
	}

	return result, nil
}
