package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/player"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/telemetry"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/version"
)

type cpuStat struct {
	idle  uint64
	total uint64
}

// DashboardHandler provides aggregate metrics and stats for the manager dashboard.
type DashboardHandler struct {
	db                 *database.ManagerDB
	engine             engine.ServerEngine
	playerManager      *player.Manager
	telemetryCollector *telemetry.TelemetryCollector
	startTime          time.Time
	mu                 sync.Mutex
	lastCPUStat        cpuStat
	lastCPUTime        time.Time
	lastCPUPercent     float64
}

// NewDashboardHandler initializes a new DashboardHandler.
func NewDashboardHandler(
	db *database.ManagerDB,
	eng engine.ServerEngine,
	pm *player.Manager,
	tc *telemetry.TelemetryCollector,
) *DashboardHandler {
	h := &DashboardHandler{
		db:                 db,
		engine:             eng,
		playerManager:      pm,
		telemetryCollector: tc,
		startTime:          time.Now(),
		lastCPUTime:        time.Now(),
	}
	if tot, idl, ok := readRawCPUStat(); ok {
		h.lastCPUStat = cpuStat{total: tot, idle: idl}
	}
	return h
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

// HostSystemSummary holds Go runtime and host OS physical telemetry.
type HostSystemSummary struct {
	Version           string  `json:"version"`
	AppName           string  `json:"app_name"`
	GoVersion         string  `json:"go_version"`
	Goroutines        int     `json:"goroutines"`
	OS                string  `json:"os"`
	Arch              string  `json:"arch"`
	UptimeSec         int64   `json:"uptime_sec"`
	AllocMB           float64 `json:"alloc_mb"`
	SysMB             float64 `json:"sys_mb"`
	HostCPUCores      int     `json:"host_cpu_cores"`
	HostCPUPercent    float64 `json:"host_cpu_percent"`
	HostTotalRAMBytes int64   `json:"host_total_ram_bytes"`
	HostUsedRAMBytes  int64   `json:"host_used_ram_bytes"`
	HostRAMPercent    float64 `json:"host_ram_percent"`
	HostLoadAvg1      float64 `json:"host_load_avg_1"`
	HostLoadAvg5      float64 `json:"host_load_avg_5"`
	HostLoadAvg15     float64 `json:"host_load_avg_15"`
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
	TotalUsedCPUPercent float64            `json:"total_used_cpu_percent"`
	TotalUsedCPUCores   float64            `json:"total_used_cpu_cores"`
	AverageCPUPercent   float64            `json:"average_cpu_percent"`
	HostSystem          HostSystemSummary  `json:"host_system"`
	TotalBackupsCount   int64              `json:"total_backups_count"`
	TotalBackupsBytes   int64              `json:"total_backups_bytes"`
	ActiveLeasesCount   int                `json:"active_leases_count"`
	AllowRulesCount     int                `json:"allow_rules_count"`
	PortGateBansCount   int                `json:"portgate_bans_count"`
	Servers             []ServerSummary    `json:"servers"`
}

// readHostMemory extracts real physical memory metrics on Linux with cross-platform fallback.
func readHostMemory() (totalBytes int64, usedBytes int64, percent float64) {
	if runtime.GOOS != "linux" {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return int64(m.Sys), int64(m.Alloc), float64(m.Alloc) / float64(m.Sys) * 100.0
	}

	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0
	}

	var memTotalKb, memAvailKb int64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			switch fields[0] {
			case "MemTotal:":
				memTotalKb, _ = strconv.ParseInt(fields[1], 10, 64)
			case "MemAvailable:":
				memAvailKb, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		}
	}

	if memTotalKb > 0 {
		totalBytes = memTotalKb * 1024
		availBytes := memAvailKb * 1024
		usedBytes = totalBytes - availBytes
		if usedBytes < 0 {
			usedBytes = 0
		}
		percent = (float64(usedBytes) / float64(totalBytes)) * 100.0
	}
	return
}

// readHostLoadAvg parses 1m, 5m, 15m system load averages on Linux.
func readHostLoadAvg() (l1, l5, l15 float64) {
	if runtime.GOOS != "linux" {
		return 0, 0, 0
	}
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		l1, _ = strconv.ParseFloat(fields[0], 64)
		l5, _ = strconv.ParseFloat(fields[1], 64)
		l15, _ = strconv.ParseFloat(fields[2], 64)
	}
	return
}

// readRawCPUStat reads aggregate CPU ticks from /proc/stat.
func readRawCPUStat() (total, idle uint64, ok bool) {
	if runtime.GOOS != "linux" {
		return 0, 0, false
	}
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0, 0, false
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, false
	}
	for i := 1; i < len(fields); i++ {
		val, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			continue
		}
		total += val
		if i == 4 || i == 5 { // idle + iowait
			idle += val
		}
	}
	return total, idle, true
}

// sampleHostCPU computes host CPU utilization percentage since the last sample.
func (h *DashboardHandler) sampleHostCPU() float64 {
	total, idle, ok := readRawCPUStat()
	if !ok {
		return 0
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	if h.lastCPUStat.total > 0 && total > h.lastCPUStat.total {
		diffTotal := float64(total - h.lastCPUStat.total)
		diffIdle := float64(idle - h.lastCPUStat.idle)
		if diffTotal > 0 {
			pct := (1.0 - (diffIdle / diffTotal)) * 100.0
			if pct < 0 {
				pct = 0
			} else if pct > 100 {
				pct = 100
			}
			h.lastCPUPercent = pct
		}
	}

	h.lastCPUStat = cpuStat{idle: idle, total: total}
	h.lastCPUTime = now
	return h.lastCPUPercent
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

	hostTotRAM, hostUsedRAM, hostRAMPct := readHostMemory()
	hostL1, hostL5, hostL15 := readHostLoadAvg()
	hostCPUPct := h.sampleHostCPU()

	hostSystem := HostSystemSummary{
		Version:           version.Version,
		AppName:           version.AppName,
		GoVersion:         runtime.Version(),
		Goroutines:        runtime.NumGoroutine(),
		OS:                runtime.GOOS,
		Arch:              runtime.GOARCH,
		UptimeSec:         int64(time.Since(h.startTime).Seconds()),
		AllocMB:           float64(m.Alloc) / (1024 * 1024),
		SysMB:             float64(m.Sys) / (1024 * 1024),
		HostCPUCores:      runtime.NumCPU(),
		HostCPUPercent:    hostCPUPct,
		HostTotalRAMBytes: hostTotRAM,
		HostUsedRAMBytes:  hostUsedRAM,
		HostRAMPercent:    hostRAMPct,
		HostLoadAvg1:      hostL1,
		HostLoadAvg5:      hostL5,
		HostLoadAvg15:     hostL15,
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
		TotalUsedCPUPercent: totalCPUPercent,
		TotalUsedCPUCores:   totalCPUPercent / 100.0,
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
