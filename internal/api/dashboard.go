package api

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/player"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/telemetry"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/version"
)

// DashboardHandler provides aggregate metrics and stats for the manager dashboard.
type DashboardHandler struct {
	db                 *database.ManagerDB
	engine             engine.ServerEngine
	playerManager      *player.Manager
	telemetryCollector *telemetry.TelemetryCollector
	startTime          time.Time
}

// NewDashboardHandler initializes a new DashboardHandler.
func NewDashboardHandler(
	db *database.ManagerDB,
	eng engine.ServerEngine,
	pm *player.Manager,
	tc *telemetry.TelemetryCollector,
) *DashboardHandler {
	return &DashboardHandler{
		db:                 db,
		engine:             eng,
		playerManager:      pm,
		telemetryCollector: tc,
		startTime:          time.Now(),
	}
}

// ServerSummary represents quick overview information for a server.
type ServerSummary struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	Port            int     `json:"port"`
	Mode            string  `json:"mode"`
	Difficulty      string  `json:"difficulty"`
	CPULimit        float64 `json:"cpu_limit"`
	MemoryLimit     string  `json:"memory_limit"`
	PortGateEnabled bool    `json:"port_gate_enabled"`
	OnlinePlayers   int     `json:"online_players"`
	CPUPercent      float64 `json:"cpu_percent"`
	RAMBytes        int64   `json:"ram_bytes"`
}

// HostSystemSummary holds Go runtime and host OS telemetry.
type HostSystemSummary struct {
	Version    string  `json:"version"`
	AppName    string  `json:"app_name"`
	GoVersion  string  `json:"go_version"`
	Goroutines int     `json:"goroutines"`
	OS         string  `json:"os"`
	Arch       string  `json:"arch"`
	UptimeSec  int64   `json:"uptime_sec"`
	AllocMB    float64 `json:"alloc_mb"`
	SysMB      float64 `json:"sys_mb"`
}

// DashboardSummaryResponse is the comprehensive payload for the dashboard overview.
type DashboardSummaryResponse struct {
	TotalServers        int                `json:"total_servers"`
	RunningServers      int                `json:"running_servers"`
	StoppedServers      int                `json:"stopped_servers"`
	TotalOnlinePlayers  int                `json:"total_online_players"`
	TotalGlobalPlayers  int                `json:"total_global_players"`
	TotalBannedPlayers  int                `json:"total_banned_players"`
	ActivePlayers       []ActivePlayerInfo `json:"active_players"`
	TotalAllocatedCores float64            `json:"total_allocated_cores"`
	TotalAllocatedRAM   int64              `json:"total_allocated_ram"`
	TotalUsedRAM        int64              `json:"total_used_ram"`
	AverageCPUPercent   float64            `json:"average_cpu_percent"`
	HostSystem          HostSystemSummary  `json:"host_system"`
	TotalBackupsCount   int64              `json:"total_backups_count"`
	TotalBackupsBytes   int64              `json:"total_backups_bytes"`
	ActiveLeasesCount   int                `json:"active_leases_count"`
	AllowRulesCount     int                `json:"allow_rules_count"`
	PortGateBansCount   int                `json:"portgate_bans_count"`
	Servers             []ServerSummary    `json:"servers"`
}

// Summary compiles aggregate server, runtime, host, player, and storage statistics.
func (h *DashboardHandler) Summary(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	servers, err := h.db.ListServers(r.Context())
	if err != nil {
		http.Error(w, `{"error": "Failed to list servers"}`, http.StatusInternalServerError)
		return
	}

	// Filter by user role if non-admin
	if claims != nil && claims.Role != models.RoleAdmin {
		allowed, _ := h.db.GetUserServerAccess(r.Context(), claims.UserID)
		allowedMap := make(map[string]bool)
		for _, id := range allowed {
			allowedMap[id] = true
		}
		var filtered []models.Server
		for _, s := range servers {
			if allowedMap[s.ID] {
				filtered = append(filtered, s)
			}
		}
		servers = filtered
	}

	totalServers := len(servers)
	runningServers := 0
	stoppedServers := 0
	var totalAllocatedCores float64
	var totalAllocatedRAM int64
	var totalUsedRAM int64
	var totalCPUPercent float64
	sampledServers := 0

	serverSummaries := make([]ServerSummary, 0, len(servers))
	serverNameMap := make(map[string]string)

	for i := range servers {
		s := servers[i]
		serverNameMap[s.ID] = s.Name

		// Sync live engine status if available
		if h.engine != nil {
			if st, err := h.engine.GetServerStatus(r.Context(), &s); err == nil && st != s.Status {
				s.Status = st
				_ = h.db.UpdateServerStatus(r.Context(), s.ID, st, s.ContainerID)
			}
		}

		totalAllocatedCores += s.CPULimit
		if memBytes, err := engine.ParseMemoryBytes(s.MemoryLimit); err == nil {
			totalAllocatedRAM += memBytes
		}

		var onlineCount int
		if h.playerManager != nil {
			onlineCount = len(h.playerManager.GetOnlinePlayers(s.ID))
		}

		var cpuPercent float64
		var ramBytes int64

		if s.Status == models.ServerStatusRunning {
			runningServers++
			if h.engine != nil {
				ctxTimeout, cancel := context.WithTimeout(r.Context(), 1200*time.Millisecond)
				stats, err := h.engine.GetContainerStats(ctxTimeout, &s)
				cancel()
				if err == nil && stats != nil {
					cpuPercent = stats.CPUPercent
					ramBytes = stats.RAMBytes
					totalUsedRAM += ramBytes
					totalCPUPercent += cpuPercent
					sampledServers++
				}
			}
		} else {
			stoppedServers++
		}

		serverSummaries = append(serverSummaries, ServerSummary{
			ID:              s.ID,
			Name:            s.Name,
			Status:          s.Status,
			Port:            s.Port,
			Mode:            s.Mode,
			Difficulty:      s.Difficulty,
			CPULimit:        s.CPULimit,
			MemoryLimit:     s.MemoryLimit,
			PortGateEnabled: s.PortGateEnabled,
			OnlinePlayers:   onlineCount,
			CPUPercent:      cpuPercent,
			RAMBytes:        ramBytes,
		})
	}

	var avgCPU float64
	if sampledServers > 0 {
		avgCPU = totalCPUPercent / float64(sampledServers)
	}

	// Active players roster
	activePlayers := make([]ActivePlayerInfo, 0)
	if h.playerManager != nil {
		onlineAll := h.playerManager.GetAllOnlinePlayers()
		for _, p := range onlineAll {
			// Non-admin can only see players on servers they have access to
			if claims != nil && claims.Role != models.RoleAdmin {
				if _, ok := serverNameMap[p.ServerID]; !ok {
					continue
				}
			}
			sName := serverNameMap[p.ServerID]
			if sName == "" {
				sName = p.ServerID
			}
			activePlayers = append(activePlayers, ActivePlayerInfo{
				ServerID:   p.ServerID,
				ServerName: sName,
				Gamertag:   p.Gamertag,
				XUID:       p.XUID,
				JoinedAt:   p.JoinedAt,
				Permission: p.Permission,
				IsOp:       p.IsOp,
			})
		}
	}

	// DB Stats
	dbStats, err := h.db.GetDashboardDBStats(r.Context())
	if err != nil {
		dbStats = &database.DashboardDBStats{}
	}

	// Host runtime info
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	hostSystem := HostSystemSummary{
		Version:    version.Version,
		AppName:    version.AppName,
		GoVersion:  runtime.Version(),
		Goroutines: runtime.NumGoroutine(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		UptimeSec:  int64(time.Since(h.startTime).Seconds()),
		AllocMB:    float64(m.Alloc) / (1024 * 1024),
		SysMB:      float64(m.Sys) / (1024 * 1024),
	}

	res := DashboardSummaryResponse{
		TotalServers:        totalServers,
		RunningServers:      runningServers,
		StoppedServers:      stoppedServers,
		TotalOnlinePlayers:  len(activePlayers),
		TotalGlobalPlayers:  dbStats.GlobalPlayersCount,
		TotalBannedPlayers:  dbStats.BannedPlayersCount,
		ActivePlayers:       activePlayers,
		TotalAllocatedCores: totalAllocatedCores,
		TotalAllocatedRAM:   totalAllocatedRAM,
		TotalUsedRAM:        totalUsedRAM,
		AverageCPUPercent:   avgCPU,
		HostSystem:          hostSystem,
		TotalBackupsCount:   dbStats.TotalBackupsCount,
		TotalBackupsBytes:   dbStats.TotalBackupsBytes,
		ActiveLeasesCount:   dbStats.ActiveLeasesCount,
		AllowRulesCount:     dbStats.AllowRulesCount,
		PortGateBansCount:   dbStats.PortGateBansCount,
		Servers:             serverSummaries,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
