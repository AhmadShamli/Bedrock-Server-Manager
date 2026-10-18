package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/mail"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/turnstile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/webhook"
)

type AuthHandler struct {
	db                *database.ManagerDB
	rateLimiter       *auth.RateLimiter
	jwtSecret         []byte
	turnstileVerifier *turnstile.Verifier
	mailer            *mail.Mailer
}

func (h *AuthHandler) SetTurnstileVerifier(tv *turnstile.Verifier) {
	h.turnstileVerifier = tv
}

func (h *AuthHandler) SetMailer(m *mail.Mailer) {
	h.mailer = m
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func getPublicBaseURL(r *http.Request, db *database.ManagerDB) string {
	if db != nil {
		if appURL, err := db.GetSetting(r.Context(), mail.SettingAppURL); err == nil && strings.TrimSpace(appURL) != "" {
			return strings.TrimRight(strings.TrimSpace(appURL), "/")
		}
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
		host = xfh
	}
	return fmt.Sprintf("%s://%s", scheme, host)
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
	Username       string `json:"username"`
	Password       string `json:"password"`
	TurnstileToken string `json:"turnstile_token,omitempty"`
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

	clientIP := GetClientIP(r).String()
	if ok, reason, _ := turnstile.Check(r.Context(), h.db, h.turnstileVerifier, "setup", req.TurnstileToken, clientIP); !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": reason})
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
	Username       string `json:"username"`
	Password       string `json:"password"`
	Email          string `json:"email"`
	TurnstileToken string `json:"turnstile_token,omitempty"`
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

	if ok, reason, _ := turnstile.Check(r.Context(), h.db, h.turnstileVerifier, "register", req.TurnstileToken, clientIP); !ok {
		lockedOut := h.rateLimiter.RecordLoginFailure(clientIP, req.Username, now)
		_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
			ActorType: "user",
			ActorName: req.Username,
			Action:    "turnstile_register_failed",
			Target:    "auth",
			Details:   fmt.Sprintf(`{"error": "%s", "locked_out": %t}`, reason, lockedOut),
			ClientIP:  clientIP,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": reason})
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

	reqVerify, _ := h.db.GetSetting(r.Context(), mail.SettingSMTPRequireEmailVerify)
	needsVerification := (reqVerify == "true" || reqVerify == "1") && req.Email != ""

	user, err := h.db.CreateUserExtended(r.Context(), req.Username, req.Email, pwHash, models.RoleUser, planID, "active", nil, !needsVerification)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to register user account"})
		return
	}

	baseURL := getPublicBaseURL(r, h.db)

	if needsVerification {
		if h.mailer != nil {
			if rawToken, err := generateRandomToken(); err == nil {
				tokenHash := hashToken(rawToken)
				expiresAt := now.Add(24 * time.Hour)
				_ = h.db.CreateAuthToken(r.Context(), user.ID, "email_verification", tokenHash, expiresAt)
				verifyURL := fmt.Sprintf("%s/verify-email?token=%s", baseURL, rawToken)
				h.mailer.SendEmailVerification(user.Email, user.Username, verifyURL)
			}
		}

		_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
			UserID:    &user.ID,
			ActorType: "user",
			ActorName: user.Username,
			Action:    "user_registered_pending_verification",
			Target:    "system",
			Details:   `{"role": "user", "plan": "` + user.PlanName + `"}`,
			ClientIP:  clientIP,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":               true,
			"requires_verification": true,
			"message":               "Account registered successfully. Please check your email to verify your account before logging in.",
			"user":                  user,
		})
		return
	}

	// Verification not required: Send welcome email if enabled
	if h.mailer != nil && req.Email != "" {
		sendWelcome, _ := h.db.GetSetting(r.Context(), mail.SettingSMTPSendWelcomeEmail)
		if sendWelcome == "true" || sendWelcome == "1" {
			h.mailer.SendWelcomeEmail(user.Email, user.Username, user.PlanName, baseURL)
		}
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
	Username       string `json:"username"`
	Password       string `json:"password"`
	RememberMe     bool   `json:"remember_me"`
	TurnstileToken string `json:"turnstile_token,omitempty"`
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

	if ok, reason, _ := turnstile.Check(r.Context(), h.db, h.turnstileVerifier, "login", req.TurnstileToken, clientIP); !ok {
		lockedOut := h.rateLimiter.RecordLoginFailure(clientIP, req.Username, now)
		_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
			ActorType: "user",
			ActorName: req.Username,
			Action:    "turnstile_login_failed",
			Target:    "auth",
			Details:   fmt.Sprintf(`{"error": "%s", "locked_out": %t}`, reason, lockedOut),
			ClientIP:  clientIP,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": reason})
		return
	}

	user, err := h.db.GetUserByUsername(r.Context(), req.Username)
	if err != nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
		lockedOut := h.rateLimiter.RecordLoginFailure(clientIP, req.Username, now)
		_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
			ActorType: "user",
			ActorName: req.Username,
			Action:    "login_failed",
			Target:    "auth",
			Details:   fmt.Sprintf(`{"error": "Invalid credentials", "locked_out": %t}`, lockedOut),
			ClientIP:  clientIP,
		})

		status := "Attempt Failed"
		if lockedOut {
			status = "IP Temporarily Locked Out"
		}

		// Dispatch security alert to Discord if configured
		if secAlert, _ := h.db.GetSetting(r.Context(), "security_alert_discord"); secAlert == "true" || secAlert == "1" {
			if webhookURL, _ := h.db.GetSetting(r.Context(), "discord_webhook_url"); webhookURL != "" {
				dispatcher := webhook.NewDispatcher()
				_ = dispatcher.NotifyFailedLogin(r.Context(), webhookURL, req.Username, clientIP, lockedOut)
			}
		}

		// Dispatch security alert to Mailer if configured
		if h.mailer != nil {
			baseURL := getPublicBaseURL(r, h.db)
			h.mailer.SendSystemAlert(r.Context(), "security", "SECURITY", "Failed Admin Login Attempt",
				fmt.Sprintf("A login attempt failed for username %s from IP %s.", req.Username, clientIP),
				map[string]string{
					"Target Username": req.Username,
					"Client IP":       clientIP,
					"Status":          status,
				},
				baseURL+"/settings",
			)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid username or password"})
		return
	}

	// Check email verification enforcement for non-admin users
	if user.Role != models.RoleAdmin && !user.EmailVerified {
		reqVerify, _ := h.db.GetSetting(r.Context(), mail.SettingSMTPRequireEmailVerify)
		if reqVerify == "true" || reqVerify == "1" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":    "Email verification is required before logging in.",
				"code":     "EMAIL_NOT_VERIFIED",
				"email":    user.Email,
				"username": user.Username,
			})
			return
		}
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

// ForgotPasswordRequest payload.
type ForgotPasswordRequest struct {
	EmailOrUsername string `json:"email_or_username"`
	TurnstileToken  string `json:"turnstile_token,omitempty"`
}

// ForgotPassword initiates a secure password reset flow.
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
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

	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if ok, reason, _ := turnstile.Check(r.Context(), h.db, h.turnstileVerifier, "password_reset", req.TurnstileToken, clientIP); !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": reason})
		return
	}

	req.EmailOrUsername = strings.TrimSpace(req.EmailOrUsername)
	if req.EmailOrUsername != "" {
		user, err := h.db.GetUserByUsernameOrEmail(r.Context(), req.EmailOrUsername)
		if err == nil && user != nil && user.Email != "" && h.mailer != nil {
			rawToken, err := generateRandomToken()
			if err == nil {
				tokenHash := hashToken(rawToken)
				_ = h.db.InvalidateUserTokens(r.Context(), user.ID, "password_reset")
				expiresAt := now.Add(15 * time.Minute)
				if err := h.db.CreateAuthToken(r.Context(), user.ID, "password_reset", tokenHash, expiresAt); err == nil {
					baseURL := getPublicBaseURL(r, h.db)
					resetURL := fmt.Sprintf("%s/reset-password?token=%s", baseURL, rawToken)
					h.mailer.SendPasswordReset(user.Email, user.Username, resetURL)

					_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
						UserID:    &user.ID,
						ActorType: "user",
						ActorName: user.Username,
						Action:    "password_reset_requested",
						Target:    "auth",
						Details:   `{"email": "` + user.Email + `"}`,
						ClientIP:  clientIP,
					})
				}
			}
		}
	}

	// Uniform anti-enumeration response
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "If an account matches that username or email address, password reset instructions have been sent.",
	})
}

// ResetPasswordRequest payload.
type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// ResetPassword validates the reset token and updates the user's password.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	clientIP := GetClientIP(r).String()
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" || len(req.Password) < 8 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Valid token and a password of at least 8 characters are required"})
		return
	}

	tokenHash := hashToken(req.Token)
	tok, err := h.db.GetAuthTokenByHash(r.Context(), tokenHash, "password_reset")
	if err != nil || tok == nil || tok.ConsumedAt != nil || time.Now().After(tok.ExpiresAt) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or expired password reset link. Please request a new one."})
		return
	}

	user, err := h.db.GetUserByID(r.Context(), tok.UserID)
	if err != nil || user == nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Associated user account not found"})
		return
	}

	pwHash, err := auth.HashPassword(req.Password, 8)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to hash new password"})
		return
	}

	if err := h.db.UpdateUserPassword(r.Context(), user.ID, pwHash); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update password"})
		return
	}

	_ = h.db.ConsumeAuthToken(r.Context(), tokenHash)
	_ = h.db.InvalidateUserTokens(r.Context(), user.ID, "password_reset")

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    &user.ID,
		ActorType: "user",
		ActorName: user.Username,
		Action:    "password_reset_completed",
		Target:    "auth",
		Details:   `{}`,
		ClientIP:  clientIP,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password reset successfully. You can now log in with your new password.",
	})
}

// VerifyEmail processes an email verification token.
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	clientIP := GetClientIP(r).String()
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" && r.Method == http.MethodPost {
		var req struct {
			Token string `json:"token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		token = strings.TrimSpace(req.Token)
	}

	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Verification token is required"})
		return
	}

	tokenHash := hashToken(token)
	tok, err := h.db.GetAuthTokenByHash(r.Context(), tokenHash, "email_verification")
	if err != nil || tok == nil || tok.ConsumedAt != nil || time.Now().After(tok.ExpiresAt) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or expired verification link. Please request a new one."})
		return
	}

	if err := h.db.SetUserEmailVerified(r.Context(), tok.UserID, true); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to verify email address"})
		return
	}

	_ = h.db.ConsumeAuthToken(r.Context(), tokenHash)

	_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:    &tok.UserID,
		ActorType: "user",
		Action:    "email_verified",
		Target:    "auth",
		Details:   `{}`,
		ClientIP:  clientIP,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Email address verified successfully. You can now log in.",
	})
}

// ResendVerificationRequest payload.
type ResendVerificationRequest struct {
	EmailOrUsername string `json:"email_or_username"`
	TurnstileToken  string `json:"turnstile_token,omitempty"`
}

// ResendVerification resends an email verification link to an unverified user.
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
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

	var req ResendVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if ok, reason, _ := turnstile.Check(r.Context(), h.db, h.turnstileVerifier, "resend_verification", req.TurnstileToken, clientIP); !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": reason})
		return
	}

	req.EmailOrUsername = strings.TrimSpace(req.EmailOrUsername)
	if req.EmailOrUsername != "" {
		user, err := h.db.GetUserByUsernameOrEmail(r.Context(), req.EmailOrUsername)
		if err == nil && user != nil && user.Email != "" && !user.EmailVerified && h.mailer != nil {
			rawToken, err := generateRandomToken()
			if err == nil {
				tokenHash := hashToken(rawToken)
				_ = h.db.InvalidateUserTokens(r.Context(), user.ID, "email_verification")
				expiresAt := now.Add(24 * time.Hour)
				if err := h.db.CreateAuthToken(r.Context(), user.ID, "email_verification", tokenHash, expiresAt); err == nil {
					baseURL := getPublicBaseURL(r, h.db)
					verifyURL := fmt.Sprintf("%s/verify-email?token=%s", baseURL, rawToken)
					h.mailer.SendEmailVerification(user.Email, user.Username, verifyURL)

					_ = h.db.CreateAuditLog(r.Context(), &models.AuditLog{
						UserID:    &user.ID,
						ActorType: "user",
						ActorName: user.Username,
						Action:    "verification_email_resent",
						Target:    "auth",
						Details:   `{"email": "` + user.Email + `"}`,
						ClientIP:  clientIP,
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "If an unverified account matches that username or email address, a new verification link has been sent.",
	})
}
