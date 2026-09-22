package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/allocator"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/player"
	"github.com/go-chi/chi/v5"
)

type ServerHandler struct {
	db        *database.ManagerDB
	engine    engine.ServerEngine
	allocator *allocator.PortAllocator
	dataDir   string
}

func NewServerHandler(db *database.ManagerDB, eng engine.ServerEngine, pa *allocator.PortAllocator, dataDir string) *ServerHandler {
	return &ServerHandler{
		db:        db,
		engine:    eng,
		allocator: pa,
		dataDir:   dataDir,
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

	// Dynamic live status check
	for i := range servers {
		if status, err := h.engine.GetServerStatus(r.Context(), &servers[i]); err == nil && status != servers[i].Status {
			servers[i].Status = status
			_ = h.db.UpdateServerStatus(r.Context(), servers[i].ID, status, servers[i].ContainerID)
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

// Create registers and provisions a new server instance.
func (h *ServerHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	// Validate ports
	existing, _ := h.db.ListServers(r.Context())
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

	// Automatically merge global allowlist and permissions on deploy/create
	_, _ = player.SyncServerWithGlobal(r.Context(), h.dataDir, s.ID, h.db, nil)

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
		Action:    "create_server",
		Target:    s.ID,
		Details:   fmt.Sprintf(`{"name": "%s", "port": %d, "container_id": "%s"}`, s.Name, s.Port, cid),
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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "running"})
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

	if err := h.engine.SendConsoleCommand(r.Context(), server, payload.Command); err != nil {
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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

// Update updates server configuration.
func (h *ServerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var s models.Server
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, http.StatusBadRequest)
		return
	}
	s.ID = id

	if err := h.db.UpdateServer(r.Context(), &s); err != nil {
		http.Error(w, `{"error": "Failed to update server"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// Delete removes a server and its container.
func (h *ServerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.db.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Server not found"}`, http.StatusNotFound)
		return
	}

	_ = h.engine.RemoveServer(r.Context(), server, false)
	if err := h.db.DeleteServer(r.Context(), id); err != nil {
		http.Error(w, `{"error": "Failed to delete server"}`, http.StatusInternalServerError)
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
		Action:    "delete_server",
		Target:    id,
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
