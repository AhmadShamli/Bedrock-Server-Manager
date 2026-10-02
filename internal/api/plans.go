package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/go-chi/chi/v5"
)

type PlanHandler struct {
	db *database.ManagerDB
}

func NewPlanHandler(db *database.ManagerDB) *PlanHandler {
	return &PlanHandler{db: db}
}

// List returns all configured plans (admin only).
func (h *PlanHandler) List(w http.ResponseWriter, r *http.Request) {
	plans, err := h.db.ListPlans(r.Context())
	if err != nil {
		http.Error(w, `{"error": "Failed to list plans"}`, http.StatusInternalServerError)
		return
	}
	if plans == nil {
		plans = []models.Plan{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(plans)
}

// Get returns details for a single plan.
func (h *PlanHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid plan ID"}`, http.StatusBadRequest)
		return
	}

	p, err := h.db.GetPlan(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Plan not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

// Create creates a new plan (admin only).
func (h *PlanHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p models.Plan
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		http.Error(w, `{"error": "Plan name is required"}`, http.StatusBadRequest)
		return
	}

	if p.MaxServers <= 0 {
		p.MaxServers = 1
	}
	if p.MaxMemory == "" {
		p.MaxMemory = "2G"
	}
	if p.MaxCPU <= 0 {
		p.MaxCPU = 2.0
	}
	if p.MaxBackupsPerServer <= 0 {
		p.MaxBackupsPerServer = 3
	}
	if p.MaxPlayerSlots <= 0 {
		p.MaxPlayerSlots = 10
	}

	if err := h.db.CreatePlan(r.Context(), &p); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Plan with this name already exists"})
			return
		}
		http.Error(w, `{"error": "Failed to create plan"}`, http.StatusInternalServerError)
		return
	}

	claims := GetUserClaims(r)
	actorName := "admin"
	var actorID *int64
	if claims != nil {
		actorName = claims.Username
		actorID = &claims.UserID
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    actorID,
		ActorType: "user",
		ActorName: actorName,
		Action:    "plan_created",
		Target:    p.Name,
		Details:   fmt.Sprintf(`{"plan_id": %d, "max_servers": %d}`, p.ID, p.MaxServers),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

// Update modifies an existing plan's quotas and settings (admin only).
func (h *PlanHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid plan ID"}`, http.StatusBadRequest)
		return
	}

	existing, err := h.db.GetPlan(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Plan not found"}`, http.StatusNotFound)
		return
	}

	var req models.Plan
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		http.Error(w, `{"error": "Plan name is required"}`, http.StatusBadRequest)
		return
	}

	req.ID = id
	req.IsDefault = existing.IsDefault // IsDefault changes should use SetDefault endpoint

	if req.MaxServers <= 0 {
		req.MaxServers = 1
	}
	if req.MaxMemory == "" {
		req.MaxMemory = "2G"
	}
	if req.MaxCPU <= 0 {
		req.MaxCPU = 2.0
	}
	if req.MaxBackupsPerServer <= 0 {
		req.MaxBackupsPerServer = 3
	}
	if req.MaxPlayerSlots <= 0 {
		req.MaxPlayerSlots = 10
	}

	if err := h.db.UpdatePlan(r.Context(), &req); err != nil {
		http.Error(w, `{"error": "Failed to update plan"}`, http.StatusInternalServerError)
		return
	}

	claims := GetUserClaims(r)
	actorName := "admin"
	var actorID *int64
	if claims != nil {
		actorName = claims.Username
		actorID = &claims.UserID
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    actorID,
		ActorType: "user",
		ActorName: actorName,
		Action:    "plan_updated",
		Target:    req.Name,
		Details:   fmt.Sprintf(`{"plan_id": %d, "max_servers": %d}`, req.ID, req.MaxServers),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(req)
}

// Delete removes a plan if it is not default and has no assigned users.
func (h *PlanHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid plan ID"}`, http.StatusBadRequest)
		return
	}

	plan, err := h.db.GetPlan(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Plan not found"}`, http.StatusNotFound)
		return
	}

	if plan.IsDefault {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Cannot delete the default plan. Designate another plan as default first."})
		return
	}

	assignedCount, err := h.db.CountUsersByPlanID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Database error checking assigned users"}`, http.StatusInternalServerError)
		return
	}

	if assignedCount > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":          fmt.Sprintf("Cannot delete plan: %d active user(s) are assigned to it. Please reassign them first.", assignedCount),
			"assigned_users": assignedCount,
		})
		return
	}

	if err := h.db.DeletePlan(r.Context(), id); err != nil {
		http.Error(w, `{"error": "Failed to delete plan"}`, http.StatusInternalServerError)
		return
	}

	claims := GetUserClaims(r)
	actorName := "admin"
	var actorID *int64
	if claims != nil {
		actorName = claims.Username
		actorID = &claims.UserID
	}
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    actorID,
		ActorType: "user",
		ActorName: actorName,
		Action:    "plan_deleted",
		Target:    plan.Name,
		Details:   fmt.Sprintf(`{"plan_id": %d}`, plan.ID),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// SetDefault designates a plan as the system-wide default plan.
func (h *PlanHandler) SetDefault(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid plan ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.SetDefaultPlan(r.Context(), id); err != nil {
		http.Error(w, `{"error": "Failed to set default plan"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// GetMyPlan returns the current user's assigned plan and live resource usage.
func (h *PlanHandler) GetMyPlan(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	user, err := h.db.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
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
		// Fallback empty plan
		plan = &models.Plan{
			Name:       "Default Plan",
			MaxServers: 1,
			MaxMemory:  "2G",
			MaxCPU:     2.0,
		}
	}

	serverCount, _ := h.db.CountServersByOwner(r.Context(), user.ID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"plan": plan,
		"usage": map[string]interface{}{
			"servers_count":     serverCount,
			"servers_max":       plan.MaxServers,
			"can_deploy_server": user.Role == models.RoleAdmin || serverCount < plan.MaxServers,
		},
		"user": map[string]interface{}{
			"id":              user.ID,
			"username":        user.Username,
			"role":            user.Role,
			"plan_status":     user.PlanStatus,
			"plan_expires_at": user.PlanExpiresAt,
		},
	})
}
