package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/allocator"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/configfile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/player"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/raknet"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/telemetry"
	"github.com/go-chi/chi/v5"
)

type ServerHandler struct {
	db            *database.ManagerDB
	engine        engine.ServerEngine
	allocator     *allocator.PortAllocator
	dataDir       string
	telemetry     *telemetry.TelemetryCollector
	metricsDB     *database.MetricsDB
	playerManager *player.Manager
}

func NewServerHandler(
	db *database.ManagerDB,
	eng engine.ServerEngine,
	pa *allocator.PortAllocator,
	dataDir string,
	tc *telemetry.TelemetryCollector,
	mdb *database.MetricsDB,
	pm *player.Manager,
) *ServerHandler {
	return &ServerHandler{
		db:            db,
		engine:        eng,
		allocator:     pa,
		dataDir:       dataDir,
		telemetry:     tc,
		metricsDB:     mdb,
		playerManager: pm,
	}
}

// List returns all servers accessible to the caller.
func (h *ServerHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	servers, err := h.db.ListServers(r.Context())
	if err != nil {
		http.Error(w, `{"error": "Failed to list servers"}`, http.StatusInternalServerError)
		return
	}

	// Dynamic live status check & seed detection
	for i := range servers {
		if status, err := h.engine.GetServerStatus(r.Context(), &servers[i]); err == nil && status != servers[i].Status {
			servers[i].Status = status
			_ = h.db.UpdateServerStatus(r.Context(), servers[i].ID, status, servers[i].ContainerID)
		}
		if servers[i].Seed == "" {
			if detected := configfile.DetectServerSeed(h.dataDir, servers[i].ID); detected != "" {
				servers[i].Seed = detected
				_ = h.db.UpdateServer(r.Context(), &servers[i])
			}
		}
	}

	var filtered []models.Server
	if claims != nil && claims.Role != models.RoleAdmin {
		allowed, _ := h.db.GetUserServerAccess(r.Context(), claims.UserID)
		allowedMap := make(map[string]bool)
		for _, id := range allowed {
			allowedMap[id] = true
		}
		for _, s := range servers {
			if allowedMap[s.ID] {
				filtered = append(filtered, s)
			}
		}
	} else {
		filtered = servers
	}

	if filtered == nil {
		filtered = []models.Server{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(filtered)
}

// Get returns details for a single server.
func (h *ServerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	// Update with live status
	if status, err := h.engine.GetServerStatus(r.Context(), server); err == nil && status != server.Status {
		server.Status = status
		_ = h.db.UpdateServerStatus(r.Context(), server.ID, status, server.ContainerID)
	}

	// Auto-detect seed from server.properties or level.dat if not yet recorded
	if server.Seed == "" {
		if detected := configfile.DetectServerSeed(h.dataDir, server.ID); detected != "" {
			server.Seed = detected
			_ = h.db.UpdateServer(r.Context(), server)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(server)
}

// SuggestPorts suggests next available (IPv4, IPv6) port pair.
func (h *ServerHandler) SuggestPorts(w http.ResponseWriter, r *http.Request) {
	existing, _ := h.db.ListServers(r.Context())
	startPort := 19132
	if p := r.URL.Query().Get("start"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 1024 {
			startPort = val
		}
	}

	p4, p6, err := h.allocator.FindAvailablePortPair(startPort, existing)
	if err != nil {
		http.Error(w, `{"error": "No available ports"}`, http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{
		"port":   p4,
		"portv6": p6,
	})
}

// ListNetworks returns available Docker deployment networks on the host.
func (h *ServerHandler) ListNetworks(w http.ResponseWriter, r *http.Request) {
	networks, err := h.engine.ListNetworks(r.Context())
	if err != nil || len(networks) == 0 {
		networks = []string{"bridge", "host"}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"networks": networks,
		"default":  "bridge",
	})
}

// Create registers and provisions a new server instance.
func (h *ServerHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var s models.Server
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, http.StatusBadRequest)
		return
	}

	if s.ID == "" || s.Name == "" {
		http.Error(w, `{"error": "Server ID and Name are required"}`, http.StatusBadRequest)
		return
	}
	if s.Port == 0 {
		s.Port = 19132
	}
	if s.PortV6 == 0 {
		s.PortV6 = 19133
	}
	if s.Status == "" {
		s.Status = models.ServerStatusStopped
	}
	if s.Mode == "" {
		s.Mode = "survival"
	}
	if s.Difficulty == "" {
		s.Difficulty = "normal"
	}
	if s.MemoryLimit == "" {
		s.MemoryLimit = "2G"
	}
	if s.CPULimit <= 0 {
		s.CPULimit = 2.0
	}
	if s.PortGateTimeout <= 0 {
		s.PortGateTimeout = 7200
	}
	s.GameServerAddress = strings.TrimSpace(s.GameServerAddress)

	existing, _ := h.db.ListServers(r.Context())

	// If user is a normal user (RoleUser), enforce Plan limits
	if claims.Role == models.RoleUser {
		user, err := h.db.GetUserByID(r.Context(), claims.UserID)
		if err != nil {
			http.Error(w, `{"error": "User record not found"}`, http.StatusForbidden)
			return
		}

		var plan *models.Plan
		if user.PlanID != nil {
			plan, _ = h.db.GetPlan(r.Context(), *user.PlanID)
		}
		if plan == nil {
			plan, _ = h.db.GetDefaultPlan(r.Context())
		}
		if plan == nil {
			plan = &models.Plan{Name: "Default Plan", MaxServers: 1, MaxMemory: "2G", MaxCPU: 2.0}
		}

		// 1. Enforce Server Count Quota
		activeCount, _ := h.db.CountServersByOwner(r.Context(), claims.UserID)
		if activeCount >= plan.MaxServers {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":          fmt.Sprintf("Server quota reached. Your plan (%s) allows a maximum of %d server(s).", plan.Name, plan.MaxServers),
				"quota_exceeded": true,
				"max_servers":    plan.MaxServers,
				"active_servers": activeCount,
			})
			return
		}

		// 2. Enforce Memory Limit
		reqBytes, _ := engine.ParseMemoryBytes(s.MemoryLimit)
		maxBytes, _ := engine.ParseMemoryBytes(plan.MaxMemory)
		if maxBytes > 0 && reqBytes > maxBytes {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Requested memory (%s) exceeds plan limit (%s)", s.MemoryLimit, plan.MaxMemory),
			})
			return
		}

		// 3. Enforce CPU Limit
		if plan.MaxCPU > 0 && s.CPULimit > plan.MaxCPU {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Requested CPU (%.1f cores) exceeds plan limit (%.1f cores)", s.CPULimit, plan.MaxCPU),
			})
			return
		}

		// 4. Enforce Preview Versions
		if !plan.AllowPreviewVersions {
			vLower := strings.ToLower(s.Version)
			if strings.Contains(vLower, "preview") || strings.Contains(vLower, "beta") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "Preview / beta versions are not permitted under your current plan",
				})
				return
			}
		}

		// 5. Enforce Port Selection (Auto-allocated if custom ports not allowed)
		if !plan.AllowCustomPort {
			p4, p6, err := h.allocator.FindAvailablePortPair(19132, existing)
			if err != nil {
				http.Error(w, `{"error": "No available ports for auto-allocation"}`, http.StatusConflict)
				return
			}
			s.Port = p4
			s.PortV6 = p6
		}

		// 6. Enforce Custom Seed
		if !plan.AllowCustomSeed {
			s.Seed = ""
		}

		// 7. Enforce Network Mode (Normal users deploy with host mode by default)
		s.NetworkMode = "host"

		s.OwnerUserID = &claims.UserID
	} else if claims.Role == models.RoleAdmin {
		if s.NetworkMode != "" {
			s.NetworkMode = strings.TrimSpace(s.NetworkMode)
		} else {
			s.NetworkMode = "host"
		}
		if s.OwnerUserID != nil {
			if _, err := h.db.GetUserByID(r.Context(), *s.OwnerUserID); err != nil {
				s.OwnerUserID = nil
			}
		}
	} else {
		http.Error(w, `{"error": "Operator accounts cannot deploy new servers"}`, http.StatusForbidden)
		return
	}

	// Validate ports
	if err := h.allocator.ValidatePortAssignment(s.Port, s.PortV6, "", existing); err != nil {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Provision container via engine
	cid, err := h.engine.CreateServer(r.Context(), &s, h.dataDir)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Container engine creation failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	s.ContainerID = cid

	if err := h.db.CreateServer(r.Context(), &s); err != nil {
		_ = h.engine.RemoveServer(r.Context(), &s, true)
		http.Error(w, `{"error": "Failed to record server, ID may already exist"}`, http.StatusConflict)
		return
	}

	// Automatically grant owner server access
	if s.OwnerUserID != nil {
		_ = h.db.GrantServerAccess(r.Context(), *s.OwnerUserID, s.ID)
	}

	// Automatically merge global allowlist and permissions on deploy/create
	_, _ = player.SyncServerWithGlobal(r.Context(), h.dataDir, s.ID, h.db, nil)

	// Update level-seed only if server.properties already exists on disk
	if s.Seed != "" {
		if propPath, err := configfile.SafePath(h.dataDir, s.ID, "server.properties"); err == nil {
			_, _ = configfile.UpdateExistingPropertyFile(propPath, map[string]string{
				"level-seed": s.Seed,
			})
		}
	}

	actorName := claims.Username
	userID := &claims.UserID

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    userID,
		ActorType: "user",
		ActorName: actorName,
		Action:    "create_server",
		Target:    s.ID,
		Details:   fmt.Sprintf(`{"name": "%s", "port": %d, "container_id": "%s", "owner_id": %v}`, s.Name, s.Port, cid, s.OwnerUserID),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(s)
}

// Start boots the server container.
func (h *ServerHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	// Automatically update and merge from global and server list on start
	_, _ = player.SyncServerWithGlobal(r.Context(), h.dataDir, server.ID, h.db, nil)

	if err := h.engine.StartServer(r.Context(), server); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to start server: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.UpdateServerStatus(r.Context(), server.ID, models.ServerStatusRunning, server.ContainerID)
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "start_server",
		Target:    server.ID,
		ClientIP:  GetClientIP(r).String(),
	})

	// If there are pending pre-boot property updates, wait for itzg to unpack server.properties, merge changes, and restart
	go h.applyPendingPropertiesOnStart(server.ID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "running"})
}

// applyPendingPropertiesOnStart checks for pre-boot pending properties, waits for itzg to unpack server.properties, merges pending properties, and restarts the container if any values changed.
func (h *ServerHandler) applyPendingPropertiesOnStart(serverID string) {
	serverDir := filepath.Join(h.dataDir, "servers", serverID)
	pendingProps, _, hasPending, err := configfile.ReadPendingProperties(serverDir)
	if err != nil || !hasPending || len(pendingProps) == 0 {
		return
	}

	propPath := filepath.Join(serverDir, "server.properties")

	// Poll until itzg unzips server.properties on disk (timeout after 45 seconds)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(45 * time.Second)

	var unzipped bool
	for !unzipped {
		select {
		case <-timeout:
			return // Timed out waiting for itzg to initialize
		case <-ticker.C:
			if fi, err := os.Stat(propPath); err == nil && fi.Size() > 0 {
				unzipped = true
			}
		}
	}

	// Short grace period to ensure itzg set-property finished writing
	time.Sleep(1 * time.Second)

	currentProps, currentKeys, err := configfile.ReadProperties(propPath)
	if err != nil {
		return
	}

	// Determine if any pending property differs from current unzipped properties
	hasChanges := false
	for k, v := range pendingProps {
		if curVal, exists := currentProps[k]; !exists || curVal != v {
			hasChanges = true
			break
		}
	}

	if hasChanges {
		mergedProps, mergedKeys := configfile.MergeProperties(currentProps, currentKeys, pendingProps)
		_ = configfile.WriteProperties(propPath, mergedProps, mergedKeys)

		// Synchronize seed if updated
		if sVal, exists := pendingProps["level-seed"]; exists {
			if srv, err := h.db.GetServer(context.Background(), serverID); err == nil && srv != nil {
				if srv.Seed != sVal {
					srv.Seed = sVal
					_ = h.db.UpdateServer(context.Background(), srv)
				}
			}
		}

		// Restart container so BDS reads updated properties
		if srv, err := h.db.GetServer(context.Background(), serverID); err == nil && srv != nil {
			_ = h.engine.RestartServer(context.Background(), srv)
		}

		_ = h.db.CreateAuditLog(context.Background(), &models.AuditLog{
			ActorType: "system",
			ActorName: "manager",
			Action:    "apply_pending_properties",
			Target:    serverID,
			Details:   `{"restarted": true, "reason": "applied_preboot_properties"}`,
		})
	}

	// Clean up pending file
	_ = configfile.RemovePendingProperties(serverDir)
}

// Stop gracefully halts the server container.
func (h *ServerHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	if err := h.engine.StopServer(r.Context(), server, 15); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to stop server: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.UpdateServerStatus(r.Context(), server.ID, models.ServerStatusStopped, server.ContainerID)
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "stop_server",
		Target:    server.ID,
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

// Restart restarts the server container.
func (h *ServerHandler) Restart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	if err := h.engine.RestartServer(r.Context(), server); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to restart server: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.UpdateServerStatus(r.Context(), server.ID, models.ServerStatusRunning, server.ContainerID)
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "restart_server",
		Target:    server.ID,
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "running"})
}

// SendCommand executes a console command on the server stdin.
func (h *ServerHandler) SendCommand(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	var payload struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Command == "" {
		http.Error(w, `{"error": "Command is required"}`, http.StatusBadRequest)
		return
	}

	cleanCmd := strings.TrimSpace(payload.Command)

	// For normal users (RoleUser), enforce safe gameplay command whitelist
	if claims := GetUserClaims(r); claims != nil && claims.Role == models.RoleUser {
		if !isSafeCommand(cleanCmd) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "Command not permitted for normal user accounts. Allowed commands: say, tell, me, list, time, weather, difficulty, gamemode, kick, whitelist, gamerule.",
			})
			return
		}
	}

	// Bedrock syntax quirk: BDS parser fails with `Syntax error: Unexpected "["` if `say` starts with `[`
	cmdWithoutSlash := strings.TrimPrefix(cleanCmd, "/")
	if strings.HasPrefix(cmdWithoutSlash, "say [") {
		msg := strings.TrimSpace(strings.TrimPrefix(cmdWithoutSlash, "say"))
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(map[string]interface{}{
			"rawtext": []map[string]string{
				{"text": msg},
			},
		}); err == nil {
			cleanCmd = fmt.Sprintf("tellraw @a %s", strings.TrimSpace(buf.String()))
		}
	}

	if err := h.engine.SendConsoleCommand(r.Context(), server, cleanCmd); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Command execution failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		ActorType: "user",
		ActorName: GetUserClaims(r).Username,
		Action:    "console_command",
		Target:    server.ID,
		Details:   fmt.Sprintf(`{"command": "%s"}`, payload.Command),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "dispatched"})
}

// Stats returns live CPU and RAM telemetry for this server.
func (h *ServerHandler) Stats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	stats, err := h.engine.GetContainerStats(r.Context(), server)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to sample stats: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	if h.playerManager != nil {
		stats.PlayerCount = len(h.playerManager.GetOnlinePlayers(server.ID))
		if stats.PlayerCount == 0 && server.Port > 0 && server.Status == models.ServerStatusRunning {
			if pong, err := raknet.PingServer("127.0.0.1", server.Port, 300*time.Millisecond); err == nil && pong.OnlinePlayers > 0 {
				stats.PlayerCount = pong.OnlinePlayers
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

// MetricsResponse represents the telemetry payload for charts in instance view.
type MetricsResponse struct {
	ServerID         string            `json:"server_id"`
	Range            string            `json:"range"`
	CPULimit         float64           `json:"cpu_limit"`
	MemoryLimitBytes int64             `json:"memory_limit_bytes"`
	MaxPlayers       int               `json:"max_players"`
	TotalAllowlist   int               `json:"total_allowlist"`
	Current          *models.MetricRaw `json:"current,omitempty"`
	Series           []MetricPoint     `json:"series"`
}

type MetricPoint struct {
	Timestamp     string  `json:"timestamp"`
	CPUPercent    float64 `json:"cpu_percent"`
	RAMBytes      int64   `json:"ram_bytes"`
	ActivePlayers int     `json:"active_players"`
}

// Metrics returns time-series telemetry data and capacity bounds for charts.
func (h *ServerHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	rangeParam := r.URL.Query().Get("range")
	var duration time.Duration
	switch rangeParam {
	case "15m":
		duration = 15 * time.Minute
	case "6h":
		duration = 6 * time.Hour
	case "24h":
		duration = 24 * time.Hour
	default:
		rangeParam = "1h"
		duration = 1 * time.Hour
	}

	since := time.Now().UTC().Add(-duration)

	var raw []models.MetricRaw
	if h.telemetry != nil {
		raw, _ = h.telemetry.QueryRaw(r.Context(), server.ID, since)
	} else if h.metricsDB != nil {
		raw, _ = h.metricsDB.QueryRaw(r.Context(), server.ID, since)
	}

	// Max players from server.properties
	maxPlayers := 10
	if propPath, err := configfile.SafePath(h.dataDir, server.ID, "server.properties"); err == nil {
		if props, _, err := configfile.ReadProperties(propPath); err == nil {
			if mpStr, ok := props["max-players"]; ok {
				if mp, err := strconv.Atoi(mpStr); err == nil && mp > 0 {
					maxPlayers = mp
				}
			}
		}
	}

	// Total players on allowlist
	totalAllowlist := 0
	if alPath, err := configfile.SafePath(h.dataDir, server.ID, "allowlist.json"); err == nil {
		if al, err := configfile.ReadAllowlist(alPath); err == nil {
			totalAllowlist = len(al)
		}
	}

	memBytes, _ := engine.ParseMemoryBytes(server.MemoryLimit)
	if memBytes <= 0 {
		memBytes = 2 * 1024 * 1024 * 1024 // Default 2GB fallback
	}

	cpuLimit := server.CPULimit
	if cpuLimit <= 0 {
		cpuLimit = 2.0
	}

	// Live stats if server running
	var currentStats *models.MetricRaw
	if server.Status == models.ServerStatusRunning {
		if cs, err := h.engine.GetContainerStats(r.Context(), server); err == nil && cs != nil {
			if h.playerManager != nil {
				cs.PlayerCount = len(h.playerManager.GetOnlinePlayers(server.ID))
				if cs.PlayerCount == 0 && server.Port > 0 {
					if pong, err := raknet.PingServer("127.0.0.1", server.Port, 300*time.Millisecond); err == nil && pong.OnlinePlayers > 0 {
						cs.PlayerCount = pong.OnlinePlayers
					}
				}
			}
			currentStats = cs
		}
	}

	series := make([]MetricPoint, 0, len(raw))
	if len(raw) > 200 {
		step := float64(len(raw)) / 150.0
		for i := 0.0; int(i) < len(raw); i += step {
			idx := int(i)
			series = append(series, MetricPoint{
				Timestamp:     database.FormatTime(raw[idx].Timestamp),
				CPUPercent:    raw[idx].CPUPercent,
				RAMBytes:      raw[idx].RAMBytes,
				ActivePlayers: raw[idx].PlayerCount,
			})
		}
	} else {
		for _, m := range raw {
			series = append(series, MetricPoint{
				Timestamp:     database.FormatTime(m.Timestamp),
				CPUPercent:    m.CPUPercent,
				RAMBytes:      m.RAMBytes,
				ActivePlayers: m.PlayerCount,
			})
		}
	}

	// If running, ensure latest live point is included if series is empty or last point older than 5s
	if currentStats != nil {
		nowStr := database.FormatTime(time.Now().UTC())
		appendCurrent := len(series) == 0
		if !appendCurrent {
			lastTs, err := database.ParseTime(series[len(series)-1].Timestamp)
			if err == nil && time.Since(lastTs) > 5*time.Second {
				appendCurrent = true
			}
		}
		if appendCurrent {
			series = append(series, MetricPoint{
				Timestamp:     nowStr,
				CPUPercent:    currentStats.CPUPercent,
				RAMBytes:      currentStats.RAMBytes,
				ActivePlayers: currentStats.PlayerCount,
			})
		}
	}

	resp := MetricsResponse{
		ServerID:         server.ID,
		Range:            rangeParam,
		CPULimit:         cpuLimit,
		MemoryLimitBytes: memBytes,
		MaxPlayers:       maxPlayers,
		TotalAllowlist:   totalAllowlist,
		Current:          currentStats,
		Series:           series,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Update updates server configuration.
func (h *ServerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.db.GetServer(r.Context(), id)
	if err != nil || existing == nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	var req struct {
		Name            *string  `json:"name"`
		Port            *int     `json:"port"`
		PortV6          *int     `json:"portv6"`
		Mode            *string  `json:"mode"`
		Difficulty      *string  `json:"difficulty"`
		Version         *string  `json:"version"`
		AutostartOnBoot *bool    `json:"autostart_on_boot"`
		PortGateEnabled *bool    `json:"port_gate_enabled"`
		PortGateMode    *string  `json:"port_gate_mode"`
		PortGateTimeout *int     `json:"port_gate_timeout"`
		MemoryLimit     *string  `json:"memory_limit"`
		CPULimit          *float64 `json:"cpu_limit"`
		Seed              *string  `json:"seed"`
		GameServerAddress *string  `json:"game_server_address"`
		NetworkMode       *string  `json:"network_mode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, http.StatusBadRequest)
		return
	}

	targetPort := existing.Port
	targetPortV6 := existing.PortV6

	if req.Port != nil && *req.Port > 0 {
		targetPort = *req.Port
	}
	if req.PortV6 != nil && *req.PortV6 > 0 {
		targetPortV6 = *req.PortV6
	}

	// If ports are changing, validate they aren't taken by other servers
	if targetPort != existing.Port || targetPortV6 != existing.PortV6 {
		existingServers, _ := h.db.ListServers(r.Context())
		if err := h.allocator.ValidatePortAssignment(targetPort, targetPortV6, existing.ID, existingServers); err != nil {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		existing.Port = targetPort
		existing.PortV6 = targetPortV6
	}

	if req.Name != nil && *req.Name != "" {
		existing.Name = *req.Name
	}
	if req.Mode != nil && *req.Mode != "" {
		existing.Mode = *req.Mode
	}
	if req.Difficulty != nil && *req.Difficulty != "" {
		existing.Difficulty = *req.Difficulty
	}
	if req.Version != nil && *req.Version != "" {
		existing.Version = *req.Version
	}
	if req.AutostartOnBoot != nil {
		existing.AutostartOnBoot = *req.AutostartOnBoot
	}
	if req.PortGateEnabled != nil {
		existing.PortGateEnabled = *req.PortGateEnabled
	}
	if req.PortGateMode != nil && *req.PortGateMode != "" {
		existing.PortGateMode = *req.PortGateMode
	}
	if req.PortGateTimeout != nil && *req.PortGateTimeout > 0 {
		existing.PortGateTimeout = *req.PortGateTimeout
	}
	if req.MemoryLimit != nil && *req.MemoryLimit != "" {
		existing.MemoryLimit = *req.MemoryLimit
	}
	if req.CPULimit != nil && *req.CPULimit > 0 {
		existing.CPULimit = *req.CPULimit
	}
	if req.Seed != nil {
		existing.Seed = *req.Seed
		propPath, err := configfile.SafePath(h.dataDir, existing.ID, "server.properties")
		if err == nil {
			props, keys, _ := configfile.ReadProperties(propPath)
			if props == nil {
				props = make(map[string]string)
			}
			props["level-seed"] = *req.Seed
			hasKey := false
			for _, k := range keys {
				if k == "level-seed" {
					hasKey = true
					break
				}
			}
			if !hasKey {
				keys = append(keys, "level-seed")
			}
			_ = configfile.WriteProperties(propPath, props, keys)
		}
	}
	if req.GameServerAddress != nil {
		existing.GameServerAddress = strings.TrimSpace(*req.GameServerAddress)
	}

	networkModeChanged := false
	if req.NetworkMode != nil {
		newMode := strings.TrimSpace(*req.NetworkMode)
		if newMode == "" {
			newMode = "bridge"
		}
		if newMode != existing.NetworkMode {
			existing.NetworkMode = newMode
			networkModeChanged = true
		}
	}

	if networkModeChanged {
		status, _ := h.engine.GetServerStatus(r.Context(), existing)
		wasRunning := status == models.ServerStatusRunning

		// Stop and remove old container
		_ = h.engine.RemoveServer(r.Context(), existing, false)

		// Recreate container with new network mode
		cid, err := h.engine.CreateServer(r.Context(), existing, h.dataDir)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Failed to recreate container with new network mode: %s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		existing.ContainerID = cid

		if wasRunning {
			if err := h.engine.StartServer(r.Context(), existing); err == nil {
				existing.Status = models.ServerStatusRunning
				_ = h.db.UpdateServerStatus(r.Context(), existing.ID, models.ServerStatusRunning, cid)
			}
		} else {
			_ = h.db.UpdateServerStatus(r.Context(), existing.ID, models.ServerStatusStopped, cid)
		}
	}

	if err := h.db.UpdateServer(r.Context(), existing); err != nil {
		http.Error(w, `{"error": "Failed to update server"}`, http.StatusInternalServerError)
		return
	}

	claims := GetUserClaims(r)
	actorName := "admin"
	var userID *int64
	if claims != nil {
		actorName = claims.Username
		userID = &claims.UserID
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    userID,
		ActorType: "user",
		ActorName: actorName,
		Action:    "update_server",
		Target:    existing.ID,
		Details:   fmt.Sprintf(`{"name": "%s", "port": %d}`, existing.Name, existing.Port),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(existing)
}

// Delete removes a server and its container.
func (h *ServerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	claims := GetUserClaims(r)
	if claims == nil {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	isOwner := server.OwnerUserID != nil && *server.OwnerUserID == claims.UserID
	if claims.Role != models.RoleAdmin && !isOwner {
		http.Error(w, `{"error": "You do not have permission to delete this server"}`, http.StatusForbidden)
		return
	}

	_ = h.engine.RemoveServer(r.Context(), server, false)
	if err := h.db.DeleteServer(r.Context(), id); err != nil {
		http.Error(w, `{"error": "Failed to delete server"}`, http.StatusInternalServerError)
		return
	}

	actorName := claims.Username
	userID := &claims.UserID

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    userID,
		ActorType: "user",
		ActorName: actorName,
		Action:    "delete_server",
		Target:    id,
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// ListCollaborators returns users assigned to this server.
func (h *ServerHandler) ListCollaborators(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	users, err := h.db.ListCollaboratorUsers(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Failed to list collaborators"}`, http.StatusInternalServerError)
		return
	}
	for i := range users {
		users[i].PasswordHash = ""
	}
	if users == nil {
		users = []models.User{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(users)
}

// AddCollaborator grants an existing user access to this server.
func (h *ServerHandler) AddCollaborator(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	claims := GetUserClaims(r)
	if claims == nil {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	isOwner := server.OwnerUserID != nil && *server.OwnerUserID == claims.UserID
	if claims.Role != models.RoleAdmin && !isOwner {
		http.Error(w, `{"error": "Only the server owner or an administrator can add collaborators"}`, http.StatusForbidden)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		http.Error(w, `{"error": "Username is required"}`, http.StatusBadRequest)
		return
	}

	targetUser, err := h.db.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
		return
	}

	// Check collaborator limits if owner
	if claims.Role != models.RoleAdmin {
		ownerUser, _ := h.db.GetUserByID(r.Context(), claims.UserID)
		var plan *models.Plan
		if ownerUser != nil && ownerUser.PlanID != nil {
			plan, _ = h.db.GetPlan(r.Context(), *ownerUser.PlanID)
		}
		if plan == nil {
			plan, _ = h.db.GetDefaultPlan(r.Context())
		}
		maxCollab := 0
		if plan != nil {
			maxCollab = plan.MaxCollaborators
		}
		existingCollabs, _ := h.db.ListCollaboratorUsers(r.Context(), id)
		count := 0
		for _, u := range existingCollabs {
			if server.OwnerUserID == nil || u.ID != *server.OwnerUserID {
				count++
			}
		}
		if count >= maxCollab {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Collaborator limit reached. Your plan allows up to %d collaborator(s).", maxCollab),
			})
			return
		}
	}

	if err := h.db.GrantServerAccess(r.Context(), targetUser.ID, server.ID); err != nil {
		http.Error(w, `{"error": "Failed to add collaborator"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// RemoveCollaborator revokes user access to this server.
func (h *ServerHandler) RemoveCollaborator(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	targetUserIDStr := chi.URLParam(r, "userId")
	targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	server, err := h.db.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	claims := GetUserClaims(r)
	if claims == nil {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	isOwner := server.OwnerUserID != nil && *server.OwnerUserID == claims.UserID
	if claims.Role != models.RoleAdmin && !isOwner {
		http.Error(w, `{"error": "Only the server owner or an administrator can remove collaborators"}`, http.StatusForbidden)
		return
	}

	if err := h.db.RevokeServerAccess(r.Context(), targetUserID, server.ID); err != nil {
		http.Error(w, `{"error": "Failed to remove collaborator"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func isSafeCommand(cmd string) bool {
	clean := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(cmd, "/")))
	parts := strings.Fields(clean)
	if len(parts) == 0 {
		return false
	}
	root := parts[0]
	safeRoots := map[string]bool{
		"say":        true,
		"tell":       true,
		"me":         true,
		"list":       true,
		"time":       true,
		"weather":    true,
		"difficulty": true,
		"gamemode":   true,
		"kick":       true,
		"whitelist":  true,
		"allowlist":  true,
		"gamerule":   true,
		"seed":       true,
		"help":       true,
		"tp":         true,
		"teleport":   true,
	}
	return safeRoots[root]
}
