package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/go-chi/chi/v5"
)

type UserHandler struct {
	db *database.ManagerDB
}

func NewUserHandler(db *database.ManagerDB) *UserHandler {
	return &UserHandler{db: db}
}

// List returns all users, redacting password hashes.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.ListUsers(r.Context())
	if err != nil {
		http.Error(w, `{"error": "Failed to list users"}`, http.StatusInternalServerError)
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

// CreateUserRequest payload.
type CreateUserRequest struct {
	Username      string     `json:"username"`
	Password      string     `json:"password"`
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	PlanID        *int64     `json:"plan_id,omitempty"`
	PlanStatus    string     `json:"plan_status,omitempty"`
	PlanExpiresAt *time.Time `json:"plan_expires_at,omitempty"`
}

// Create registers a new user account (admin only).
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))

	if req.Username == "" || len(req.Password) < 8 {
		http.Error(w, `{"error": "Username is required and password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}

	if req.Role != models.RoleAdmin && req.Role != models.RoleOperator && req.Role != models.RoleUser {
		req.Role = models.RoleUser
	}

	// Check if user already exists
	if _, err := h.db.GetUserByUsername(r.Context(), req.Username); err == nil {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Username already exists"})
		return
	}

	pwHash, err := auth.HashPassword(req.Password, 8)
	if err != nil {
		http.Error(w, `{"error": "Failed to hash password"}`, http.StatusInternalServerError)
		return
	}

	user, err := h.db.CreateUserExtended(r.Context(), req.Username, req.Email, pwHash, req.Role, req.PlanID, req.PlanStatus, req.PlanExpiresAt)
	if err != nil {
		http.Error(w, `{"error": "Failed to create user"}`, http.StatusInternalServerError)
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
		Action:    "user_created",
		Target:    user.Username,
		Details:   fmt.Sprintf(`{"user_id": %d, "role": "%s", "plan": "%s"}`, user.ID, user.Role, user.PlanName),
		ClientIP:  GetClientIP(r).String(),
	})

	user.PasswordHash = ""
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(user)
}

// UpdateUserRequest payload.
type UpdateUserRequest struct {
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	PlanID        *int64     `json:"plan_id"`
	PlanStatus    string     `json:"plan_status"`
	PlanExpiresAt *time.Time `json:"plan_expires_at"`
}

// Update modifies an existing user's role, email, and plan assignment (admin only).
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if req.Role != "" {
		if req.Role != models.RoleAdmin && req.Role != models.RoleOperator && req.Role != models.RoleUser {
			http.Error(w, `{"error": "Invalid role"}`, http.StatusBadRequest)
			return
		}
		user.Role = req.Role
	}
	user.Email = req.Email
	user.PlanID = req.PlanID
	if req.PlanStatus != "" {
		user.PlanStatus = req.PlanStatus
	}
	user.PlanExpiresAt = req.PlanExpiresAt

	if err := h.db.UpdateUser(r.Context(), user); err != nil {
		http.Error(w, `{"error": "Failed to update user"}`, http.StatusInternalServerError)
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
		Action:    "user_updated",
		Target:    user.Username,
		Details:   fmt.Sprintf(`{"user_id": %d, "role": "%s", "plan_id": %v}`, user.ID, user.Role, user.PlanID),
		ClientIP:  GetClientIP(r).String(),
	})

	updated, _ := h.db.GetUserByID(r.Context(), id)
	if updated != nil {
		updated.PasswordHash = ""
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(updated)
		return
	}

	user.PasswordHash = ""
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

// UpdatePassword updates an existing user's password.
func (h *UserHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByID(r.Context(), id)
	if err != nil || user == nil {
		http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Password) < 8 {
		http.Error(w, `{"error": "New password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}

	newHash, err := auth.HashPassword(req.Password, 8)
	if err != nil {
		http.Error(w, `{"error": "Failed to hash password"}`, http.StatusInternalServerError)
		return
	}

	if err := h.db.UpdateUserPassword(r.Context(), id, newHash); err != nil {
		http.Error(w, `{"error": "Failed to update password"}`, http.StatusInternalServerError)
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
		Action:    "user_password_changed",
		Target:    user.Username,
		Details:   fmt.Sprintf(`{"user_id": %d}`, user.ID),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Delete removes a user account.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	claims := GetUserClaims(r)
	if claims != nil && claims.UserID == id {
		http.Error(w, `{"error": "Cannot delete your own account"}`, http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByID(r.Context(), id)
	if err != nil || user == nil {
		http.Error(w, `{"error": "User not found"}`, http.StatusNotFound)
		return
	}

	if err := h.db.DeleteUser(r.Context(), id); err != nil {
		http.Error(w, `{"error": "Failed to delete user"}`, http.StatusInternalServerError)
		return
	}

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
		Action:    "user_deleted",
		Target:    user.Username,
		Details:   fmt.Sprintf(`{"user_id": %d}`, user.ID),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// GetServerAccess returns the list of server IDs assigned to a user.
func (h *UserHandler) GetServerAccess(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	serverIDs, err := h.db.GetUserServerAccess(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error": "Failed to get user server access"}`, http.StatusInternalServerError)
		return
	}
	if serverIDs == nil {
		serverIDs = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(serverIDs)
}

// UpdateServerAccess sets the list of servers assigned to an operator user.
func (h *UserHandler) UpdateServerAccess(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		ServerIDs []string `json:"server_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	// Revoke existing access
	existing, _ := h.db.GetUserServerAccess(r.Context(), id)
	for _, sID := range existing {
		_ = h.db.RevokeServerAccess(r.Context(), id, sID)
	}

	// Grant new access
	for _, sID := range req.ServerIDs {
		if err := h.db.GrantServerAccess(r.Context(), id, sID); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Failed to grant server access to %s: %s"}`, sID, err.Error()), http.StatusBadRequest)
			return
		}
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
		Action:    "user_server_access_updated",
		Target:    idStr,
		Details:   fmt.Sprintf(`{"server_ids": %v}`, req.ServerIDs),
		ClientIP:  GetClientIP(r).String(),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
