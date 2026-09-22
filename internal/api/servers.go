package api

import (
	"encoding/json"
	"net/http"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/go-chi/chi/v5"
)

type ServerHandler struct {
	db *database.ManagerDB
}

func NewServerHandler(db *database.ManagerDB) *ServerHandler {
	return &ServerHandler{db: db}
}

// List returns all servers accessible to the caller.
func (h *ServerHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	servers, err := h.db.ListServers(r.Context())
	if err != nil {
		http.Error(w, `{"error": "Failed to list servers"}`, http.StatusInternalServerError)
		return
	}

	// Filter by permissions if not admin
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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(server)
}

// Create registers a new server.
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

	if err := h.db.CreateServer(r.Context(), &s); err != nil {
		http.Error(w, `{"error": "Failed to create server, ID may already exist"}`, http.StatusConflict)
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
		Action:    "create_server",
		Target:    s.ID,
		Details:   `{"name": "` + s.Name + `", "port": ` + string(rune(s.Port)) + `}`,
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(s)
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

// Delete removes a server registration.
func (h *ServerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
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
