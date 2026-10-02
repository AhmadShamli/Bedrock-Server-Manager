package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

type AuthHandler struct {
	db          *database.ManagerDB
	rateLimiter *auth.RateLimiter
	jwtSecret   []byte
}

func NewAuthHandler(db *database.ManagerDB, rateLimiter *auth.RateLimiter, jwtSecret []byte) *AuthHandler {
	return &AuthHandler{
		db:          db,
		rateLimiter: rateLimiter,
		jwtSecret:   jwtSecret,
	}
}

// SetupStatus checks if the system has at least one admin account.
func (h *AuthHandler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	count, err := h.db.CountUsers(r.Context())
	if err != nil {
		http.Error(w, `{"error": "Failed to check setup status"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"needs_setup": count == 0,
	})
}

// SetupRequest payload for initial wizard.
type SetupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Setup initializes the primary administrator account.
func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	count, err := h.db.CountUsers(r.Context())
	if err != nil {
		http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		return
	}
	if count > 0 {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "System is already initialized"})
		return
	}

	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Username == "" || len(req.Password) < 8 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Username is required and password must be at least 8 characters"})
		return
	}

	pwHash, err := auth.HashPassword(req.Password, 8)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to hash password"})
		return
	}

	user, err := h.db.CreateUser(r.Context(), req.Username, pwHash, models.RoleAdmin)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create administrator account"})
		return
	}

	// Issue initial session token
	token, err := auth.GenerateJWT(h.jwtSecret, user.ID, user.Username, user.Role, 24*time.Hour)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate authentication token"})
		return
	}

	clientIP := GetClientIP(r).String()
	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    &user.ID,
		ActorType: "user",
		ActorName: user.Username,
		Action:    "setup_completed",
		Target:    "system",
		Details:   `{"message": "Initial administrator account created"}`,
		ClientIP:  clientIP,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "bsm_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
		"user":    user,
	})
}

// RegisterRequest payload for self-registration.
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

// Register creates a new normal user account if registration is enabled.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	allowReg, _ := h.db.GetSetting(r.Context(), "allow_registration")
	if allowReg != "true" && allowReg != "1" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Public registration is currently disabled by administrator"})
		return
	}

	clientIP := GetClientIP(r).String()
	now := time.Now()
	rateCheck := h.rateLimiter.CheckLogin(clientIP, now)
	if rateCheck.Blocked {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": rateCheck.Reason,
		})
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" || len(req.Password) < 8 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Username is required and password must be at least 8 characters"})
		return
	}

	if _, err := h.db.GetUserByUsername(r.Context(), req.Username); err == nil {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Username already exists"})
		return
	}

	pwHash, err := auth.HashPassword(req.Password, 8)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to hash password"})
		return
	}

	defPlan, _ := h.db.GetDefaultPlan(r.Context())
	var planID *int64
	if defPlan != nil {
		planID = &defPlan.ID
	}

	user, err := h.db.CreateUserExtended(r.Context(), req.Username, req.Email, pwHash, models.RoleUser, planID, "active", nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to register user account"})
		return
	}

	token, err := auth.GenerateJWT(h.jwtSecret, user.ID, user.Username, user.Role, 24*time.Hour)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate authentication token"})
		return
	}

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    &user.ID,
		ActorType: "user",
		ActorName: user.Username,
		Action:    "user_registered",
		Target:    "system",
		Details:   `{"role": "user", "plan": "` + user.PlanName + `"}`,
		ClientIP:  clientIP,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "bsm_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
		"user":    user,
	})
}

// LoginRequest payload.
type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

// Login authenticates a user and returns a JWT token.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	clientIP := GetClientIP(r).String()
	now := time.Now()

	rateCheck := h.rateLimiter.CheckLogin(clientIP, now)
	if rateCheck.Blocked {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":               rateCheck.Reason,
			"retry_after_seconds": rateCheck.RetryAfterSeconds,
		})
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request payload"})
		return
	}

	user, err := h.db.GetUserByUsername(r.Context(), req.Username)
	if err != nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
		h.rateLimiter.RecordLoginFailure(clientIP, req.Username, now)
		_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
			ActorType: "user",
			ActorName: req.Username,
			Action:    "login_failed",
			Target:    "auth",
			Details:   `{"error": "Invalid credentials"}`,
			ClientIP:  clientIP,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid username or password"})
		return
	}

	duration := 24 * time.Hour
	if req.RememberMe {
		duration = 30 * 24 * time.Hour
	}

	token, err := auth.GenerateJWT(h.jwtSecret, user.ID, user.Username, user.Role, duration)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate session token"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "bsm_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(duration.Seconds()),
	})

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    &user.ID,
		ActorType: "user",
		ActorName: user.Username,
		Action:    "login_success",
		Target:    "auth",
		Details:   `{"remember_me": ` + jsonBool(req.RememberMe) + `}`,
		ClientIP:  clientIP,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
		"user":    user,
	})
}

// Logout clears the session cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "bsm_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
}

// Me returns the profile and server permissions of the authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
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

	var allowedServers []string
	if user.Role == models.RoleAdmin {
		allowedServers = []string{"*"}
	} else {
		allowedServers, _ = h.db.GetUserServerAccess(r.Context(), user.ID)
		if allowedServers == nil {
			allowedServers = []string{}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"user":            user,
		"allowed_servers": allowedServers,
	})
}

// Refresh issues a renewed JWT token.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	claims := GetUserClaims(r)
	if claims == nil {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateJWT(h.jwtSecret, claims.UserID, claims.Username, claims.Role, 24*time.Hour)
	if err != nil {
		http.Error(w, `{"error": "Failed to refresh token"}`, http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "bsm_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func jsonBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
