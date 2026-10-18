package turnstile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
)

func setupTestDB(t *testing.T) *database.ManagerDB {
	t.Helper()
	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestVerifier_VerifySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		if r.FormValue("secret") != "test-secret" {
			t.Errorf("expected secret test-secret, got %s", r.FormValue("secret"))
		}
		if r.FormValue("response") != "valid-token" {
			t.Errorf("expected response valid-token, got %s", r.FormValue("response"))
		}
		if r.FormValue("remoteip") != "1.2.3.4" {
			t.Errorf("expected remoteip 1.2.3.4, got %s", r.FormValue("remoteip"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SiteVerifyResponse{
			Success:     true,
			ChallengeTS: "2026-10-18T10:00:00Z",
			Hostname:    "example.com",
			Action:      "login",
		})
	}))
	defer server.Close()

	verifier := NewVerifier()
	verifier.SetVerifyURL(server.URL)

	res, err := verifier.Verify(context.Background(), "test-secret", "valid-token", "1.2.3.4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success true, got false")
	}
}

func TestVerifier_VerifyFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SiteVerifyResponse{
			Success:    false,
			ErrorCodes: []string{"invalid-input-response"},
		})
	}))
	defer server.Close()

	verifier := NewVerifier()
	verifier.SetVerifyURL(server.URL)

	res, err := verifier.Verify(context.Background(), "test-secret", "bad-token", "1.2.3.4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success {
		t.Errorf("expected success false, got true")
	}
	if len(res.ErrorCodes) == 0 || res.ErrorCodes[0] != "invalid-input-response" {
		t.Errorf("expected invalid-input-response, got %v", res.ErrorCodes)
	}
}

func TestCheck_TurnstileDisabled(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	// Enabled is false
	_ = db.SetSetting(ctx, SettingTurnstileEnabled, "false")
	_ = db.SetSetting(ctx, SettingTurnstileSecretKey, "some-secret")

	verifier := NewVerifier()
	allowed, reason, err := Check(ctx, db, verifier, "login", "", "1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected allowed=true when turnstile is disabled, reason=%s", reason)
	}
}

func TestCheck_SpecificActionDisabled(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	_ = db.SetSetting(ctx, SettingTurnstileEnabled, "true")
	_ = db.SetSetting(ctx, SettingTurnstileSecretKey, "some-secret")
	_ = db.SetSetting(ctx, SettingTurnstileLoginEnabled, "false")
	_ = db.SetSetting(ctx, SettingTurnstileKnockEnabled, "true")

	verifier := NewVerifier()
	allowed, _, err := Check(ctx, db, verifier, "login", "", "1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected allowed=true when login action is disabled")
	}

	// But knock should require token!
	allowedKnock, _, _ := Check(ctx, db, verifier, "knock", "", "1.2.3.4")
	if allowedKnock {
		t.Errorf("expected allowed=false when knock action is enabled but token is missing")
	}
}

func TestCheck_FailOpenBehavior(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	_ = db.SetSetting(ctx, SettingTurnstileEnabled, "true")
	_ = db.SetSetting(ctx, SettingTurnstileSecretKey, "test-secret")
	_ = db.SetSetting(ctx, SettingTurnstileLoginEnabled, "true")
	_ = db.SetSetting(ctx, SettingTurnstileFailOpen, "true")

	// Set invalid URL to simulate Cloudflare outage / network error
	verifier := NewVerifier()
	verifier.SetVerifyURL("http://127.0.0.1:54321/broken-endpoint")

	allowed, reason, err := Check(ctx, db, verifier, "login", "some-token", "1.2.3.4")
	if err != nil {
		t.Fatalf("expected nil error when fail-open is enabled, got %v", err)
	}
	if !allowed {
		t.Errorf("expected allowed=true with fail-open enabled on network failure, reason=%s", reason)
	}

	// Now disable fail-open: should fail closed!
	_ = db.SetSetting(ctx, SettingTurnstileFailOpen, "false")
	allowedClosed, reasonClosed, errClosed := Check(ctx, db, verifier, "login", "some-token", "1.2.3.4")
	if allowedClosed {
		t.Errorf("expected allowed=false when fail-open is false")
	}
	if errClosed == nil {
		t.Errorf("expected non-nil error when fail-closed on network error")
	}
	_ = reasonClosed
}
