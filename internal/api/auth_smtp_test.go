package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/mail"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func setupTestAuthHandler(t *testing.T) (*database.ManagerDB, *AuthHandler, *mail.Mailer) {
	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	rateLimiter := auth.NewRateLimiter(5, 5*time.Minute, 10, 5*time.Minute, 15*time.Minute)
	jwtSecret := []byte("test_secret_32_bytes_long_123456")
	mailer := mail.NewMailer(db)

	handler := NewAuthHandler(db, rateLimiter, jwtSecret)
	handler.SetMailer(mailer)

	return db, handler, mailer
}

func TestForgotPasswordAndResetFlow(t *testing.T) {
	db, handler, mailer := setupTestAuthHandler(t)
	defer db.Close()
	defer mailer.Close()

	ctx := context.Background()

	// 1. Create a user with email
	pwHash, _ := auth.HashPassword("InitialPassword123", 8)
	user, err := db.CreateUserExtended(ctx, "resetguy", "resetguy@example.com", pwHash, models.RoleUser, nil, "active", nil, true)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 2. Request password reset with username
	body, _ := json.Marshal(map[string]string{
		"email_or_username": "resetguy",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ForgotPassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	// Check that a token was created in database
	tok, err := db.GetAuthTokenByHash(ctx, "", "password_reset") // not matching empty hash, but let's query via token creation
	// Let's create a known raw token directly to verify ResetPassword endpoint
	rawToken := "known_test_token_abcdef1234567890"
	tokenHash := hashToken(rawToken)
	if err := db.CreateAuthToken(ctx, user.ID, "password_reset", tokenHash, time.Now().UTC().Add(15*time.Minute)); err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Verify token is retrievable
	tok, err = db.GetAuthTokenByHash(ctx, tokenHash, "password_reset")
	if err != nil || tok.UserID != user.ID {
		t.Fatalf("failed to fetch auth token: %v", err)
	}

	// 3. Reset password using invalid token (should fail)
	badResetBody, _ := json.Marshal(map[string]string{
		"token":    "wrong_token",
		"password": "NewSecretPassword123",
	})
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(badResetBody))
	reqBad.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	handler.ResetPassword(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad token, got %d", wBad.Code)
	}

	// 4. Reset password with short password (should fail)
	shortBody, _ := json.Marshal(map[string]string{
		"token":    rawToken,
		"password": "short",
	})
	reqShort := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(shortBody))
	wShort := httptest.NewRecorder()
	handler.ResetPassword(wShort, reqShort)
	if wShort.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short password, got %d", wShort.Code)
	}

	// 5. Reset password successfully
	goodBody, _ := json.Marshal(map[string]string{
		"token":    rawToken,
		"password": "NewSecretPassword123",
	})
	reqGood := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(goodBody))
	wGood := httptest.NewRecorder()
	handler.ResetPassword(wGood, reqGood)
	if wGood.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for reset, got %d: %s", wGood.Code, wGood.Body.String())
	}

	// Token should now be consumed
	consumedTok, err := db.GetAuthTokenByHash(ctx, tokenHash, "password_reset")
	if err != nil || consumedTok.ConsumedAt == nil {
		t.Errorf("expected token to be consumed")
	}

	// 6. Verify login works with new password and fails with old password
	loginFailBody, _ := json.Marshal(map[string]string{
		"username": "resetguy",
		"password": "InitialPassword123",
	})
	reqFailLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginFailBody))
	wFailLogin := httptest.NewRecorder()
	handler.Login(wFailLogin, reqFailLogin)
	if wFailLogin.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for old password, got %d", wFailLogin.Code)
	}

	loginOkBody, _ := json.Marshal(map[string]string{
		"username": "resetguy",
		"password": "NewSecretPassword123",
	})
	reqOkLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginOkBody))
	wOkLogin := httptest.NewRecorder()
	handler.Login(wOkLogin, reqOkLogin)
	if wOkLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for new password, got %d: %s", wOkLogin.Code, wOkLogin.Body.String())
	}
}

func TestEmailVerificationEnforcement(t *testing.T) {
	db, handler, mailer := setupTestAuthHandler(t)
	defer db.Close()
	defer mailer.Close()

	ctx := context.Background()

	// Enable require email verification setting
	_ = db.SetSetting(ctx, mail.SettingSMTPRequireEmailVerify, "true")

	// 1. Unverified user created
	pwHash, _ := auth.HashPassword("Password123", 8)
	user, err := db.CreateUserExtended(ctx, "unverified_bob", "bob@example.com", pwHash, models.RoleUser, nil, "active", nil, false)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 2. Login should be blocked with EMAIL_NOT_VERIFIED
	loginBody, _ := json.Marshal(map[string]string{
		"username": "unverified_bob",
		"password": "Password123",
	})
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	wLogin := httptest.NewRecorder()
	handler.Login(wLogin, reqLogin)

	if wLogin.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for unverified user, got %d", wLogin.Code)
	}
	var errResp map[string]interface{}
	_ = json.Unmarshal(wLogin.Body.Bytes(), &errResp)
	if errResp["code"] != "EMAIL_NOT_VERIFIED" {
		t.Errorf("expected code EMAIL_NOT_VERIFIED, got %v", errResp["code"])
	}

	// 3. Admin login should NOT be blocked even if email is unverified (failover protection)
	adminPwHash, _ := auth.HashPassword("AdminPassword123", 8)
	_, _ = db.CreateUserExtended(ctx, "admin_test", "admin@example.com", adminPwHash, models.RoleAdmin, nil, "active", nil, false)
	adminLoginBody, _ := json.Marshal(map[string]string{
		"username": "admin_test",
		"password": "AdminPassword123",
	})
	reqAdmin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(adminLoginBody))
	wAdmin := httptest.NewRecorder()
	handler.Login(wAdmin, reqAdmin)
	if wAdmin.Code != http.StatusOK {
		t.Errorf("expected admin login to succeed without email verification, got %d", wAdmin.Code)
	}

	// 4. Verify email using token
	rawVerifyToken := "test_verify_token_12345"
	verifyHash := hashToken(rawVerifyToken)
	_ = db.CreateAuthToken(ctx, user.ID, "email_verification", verifyHash, time.Now().UTC().Add(24*time.Hour))

	reqVerify := httptest.NewRequest(http.MethodGet, "/api/v1/auth/verify-email?token="+rawVerifyToken, nil)
	wVerify := httptest.NewRecorder()
	handler.VerifyEmail(wVerify, reqVerify)

	if wVerify.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for email verification, got %d: %s", wVerify.Code, wVerify.Body.String())
	}

	// 5. User should now be able to log in successfully
	wLoginAfter := httptest.NewRecorder()
	reqLoginAfter := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	handler.Login(wLoginAfter, reqLoginAfter)
	if wLoginAfter.Code != http.StatusOK {
		t.Errorf("expected login to succeed after verification, got %d: %s", wLoginAfter.Code, wLoginAfter.Body.String())
	}
}
