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

func TestIsLocalOrLAN(t *testing.T) {
	testCases := []struct {
		ip       string
		expected bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"localhost", true},
		{"localhost:8080", true},
		{"127.0.0.1:54321", true},
		{"::ffff:127.0.0.1", true},
		{"192.168.1.100", true},
		{"10.0.0.15", true},
		{"172.16.0.1", true},
		{"172.31.255.254", true},
		{"169.254.10.20", true},
		{"fe80::1ff:fe00:1", true},
		{"fc00::1", true},
		{"fd00::1", true},
		{"::ffff:192.168.1.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"172.32.0.1", false},
		{"203.0.113.5", false},
		{"2606:4700:4700::1111", false},
	}

	for _, tc := range testCases {
		if got := IsLocalOrLAN(tc.ip); got != tc.expected {
			t.Errorf("IsLocalOrLAN(%q) = %v, expected %v", tc.ip, got, tc.expected)
		}
	}
}

func TestCheck_BypassLocalBehavior(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	_ = db.SetSetting(ctx, SettingTurnstileEnabled, "true")
	_ = db.SetSetting(ctx, SettingTurnstileSecretKey, "test-secret")
	_ = db.SetSetting(ctx, SettingTurnstileLoginEnabled, "true")
	_ = db.SetSetting(ctx, SettingTurnstileBypassLocal, "true")

	verifier := NewVerifier()

	// 1. Local / LAN IPs without token should pass immediately when bypass_local is true
	localIPs := []string{"127.0.0.1", "::1", "localhost", "192.168.1.100", "10.0.5.1", "172.20.1.1"}
	for _, ip := range localIPs {
		allowed, reason, err := Check(ctx, db, verifier, "login", "", ip)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", ip, err)
		}
		if !allowed {
			t.Errorf("expected allowed=true for local IP %s when bypass_local=true, got reason: %s", ip, reason)
		}
	}

	// 2. Public IP without token must still be blocked
	allowedPub, reasonPub, errPub := Check(ctx, db, verifier, "login", "", "203.0.113.10")
	if errPub != nil {
		t.Fatalf("unexpected error: %v", errPub)
	}
	if allowedPub {
		t.Errorf("expected allowed=false for public IP without token, got reason=%s", reasonPub)
	}

	// 3. When bypass_local is false, local IPs must NOT be bypassed
	_ = db.SetSetting(ctx, SettingTurnstileBypassLocal, "false")
	allowedLocalBlocked, _, _ := Check(ctx, db, verifier, "login", "", "127.0.0.1")
	if allowedLocalBlocked {
		t.Errorf("expected allowed=false for 127.0.0.1 when bypass_local=false and token missing")
	}
}

func TestIsRequestLocalOrLAN(t *testing.T) {
	testCases := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		nilRequest bool
		expected   bool
	}{
		{
			name:       "nil request",
			nilRequest: true,
			expected:   false,
		},
		{
			name:       "direct loopback",
			remoteAddr: "127.0.0.1:45678",
			expected:   true,
		},
		{
			name:       "direct private 192.168.x",
			remoteAddr: "192.168.1.50:12345",
			expected:   true,
		},
		{
			name:       "direct private 10.x",
			remoteAddr: "10.0.0.15:12345",
			expected:   true,
		},
		{
			name:       "direct IPv6 loopback",
			remoteAddr: "[::1]:12345",
			expected:   true,
		},
		{
			name:       "direct public peer",
			remoteAddr: "203.0.113.195:12345",
			expected:   false,
		},
		{
			name:       "public peer spoofing X-Forwarded-For with local IP",
			remoteAddr: "203.0.113.195:12345",
			headers: map[string]string{
				"X-Forwarded-For": "127.0.0.1",
			},
			expected: false,
		},
		{
			name:       "local reverse proxy with external X-Forwarded-For",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.10",
			},
			expected: false,
		},
		{
			name:       "local reverse proxy with multi-hop X-Forwarded-For where client is external",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.10, 192.168.1.100",
			},
			expected: false,
		},
		{
			name:       "local reverse proxy with internal LAN X-Forwarded-For",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.50, 10.0.0.2",
			},
			expected: true,
		},
		{
			name:       "local reverse proxy with external CF-Connecting-IP",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"CF-Connecting-IP": "198.51.100.25",
			},
			expected: false,
		},
		{
			name:       "local reverse proxy with internal CF-Connecting-IP",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"CF-Connecting-IP": "192.168.1.25",
			},
			expected: true,
		},
		{
			name:       "local reverse proxy with external True-Client-IP",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"True-Client-IP": "198.51.100.5",
			},
			expected: false,
		},
		{
			name:       "local reverse proxy with external X-Real-IP",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.88",
			},
			expected: false,
		},
		{
			name:       "local reverse proxy with local X-Real-IP",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"X-Real-IP": "10.0.0.5",
			},
			expected: true,
		},
		{
			name:       "local reverse proxy with RFC 7239 external IPv4 Forwarded",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"Forwarded": "for=203.0.113.195;proto=https;by=127.0.0.1",
			},
			expected: false,
		},
		{
			name:       "local reverse proxy with RFC 7239 external IPv6 Forwarded",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"Forwarded": `for="[2001:db8::1]";proto=https`,
			},
			expected: false,
		},
		{
			name:       "local reverse proxy with RFC 7239 local Forwarded",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"Forwarded": "for=192.168.1.55;proto=http",
			},
			expected: true,
		},
		{
			name:       "local reverse proxy with external X-Forwarded-Host and Via",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"X-Forwarded-Host": "minecraft.example.com",
				"Via":              "1.1 caddy",
			},
			expected: false,
		},
		{
			name:       "local reverse proxy with local X-Forwarded-Host and Via",
			remoteAddr: "127.0.0.1:45678",
			headers: map[string]string{
				"X-Forwarded-Host": "myserver.local",
				"Via":              "1.1 caddy",
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.nilRequest {
				got := IsRequestLocalOrLAN(nil)
				if got != tc.expected {
					t.Errorf("IsRequestLocalOrLAN(nil) = %v, expected %v", got, tc.expected)
				}
				return
			}

			req := httptest.NewRequest(http.MethodGet, "http://localhost/test", nil)
			req.RemoteAddr = tc.remoteAddr
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			got := IsRequestLocalOrLAN(req)
			if got != tc.expected {
				t.Errorf("IsRequestLocalOrLAN() = %v, expected %v", got, tc.expected)
			}
		})
	}
}

func TestCheckRequest_ReverseProxy(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	_ = db.SetSetting(ctx, SettingTurnstileEnabled, "true")
	_ = db.SetSetting(ctx, SettingTurnstileSecretKey, "test-secret")
	_ = db.SetSetting(ctx, SettingTurnstileLoginEnabled, "true")
	_ = db.SetSetting(ctx, SettingTurnstileBypassLocal, "true")

	// Mock verifier that succeeds for "valid-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		token := r.FormValue("response")
		w.Header().Set("Content-Type", "application/json")
		if token == "valid-token" {
			_ = json.NewEncoder(w).Encode(SiteVerifyResponse{Success: true})
		} else {
			_ = json.NewEncoder(w).Encode(SiteVerifyResponse{Success: false, ErrorCodes: []string{"invalid-input-response"}})
		}
	}))
	defer server.Close()

	verifier := NewVerifier()
	verifier.SetVerifyURL(server.URL)

	// 1. Direct local connection without token -> Allowed because bypass_local is true
	reqLocal := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	reqLocal.RemoteAddr = "127.0.0.1:12345"
	allowed, reason, err := CheckRequest(reqLocal, db, verifier, "login", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected allowed=true for direct local connection, got reason: %s", reason)
	}

	// 2. Local reverse proxy forwarding external client (X-Forwarded-For: 203.0.113.50) without token -> Must be BLOCKED
	reqProxy := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	reqProxy.RemoteAddr = "127.0.0.1:12345"
	reqProxy.Header.Set("X-Forwarded-For", "203.0.113.50")
	allowedProxy, reasonProxy, errProxy := CheckRequest(reqProxy, db, verifier, "login", "")
	if errProxy != nil {
		t.Fatalf("unexpected error: %v", errProxy)
	}
	if allowedProxy {
		t.Errorf("expected allowed=false for local proxy forwarding external client without token")
	}
	if reasonProxy == "" {
		t.Errorf("expected reason for blocking, got empty")
	}

	// 3. Local reverse proxy forwarding external client WITH valid token -> Allowed
	allowedProxyValid, reasonProxyValid, errProxyValid := CheckRequest(reqProxy, db, verifier, "login", "valid-token")
	if errProxyValid != nil {
		t.Fatalf("unexpected error: %v", errProxyValid)
	}
	if !allowedProxyValid {
		t.Errorf("expected allowed=true with valid token, got reason: %s", reasonProxyValid)
	}

	// 4. Spoofed public peer attempting to bypass Turnstile with X-Forwarded-For: 127.0.0.1 -> Must be BLOCKED
	reqSpoofed := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	reqSpoofed.RemoteAddr = "203.0.113.80:54321"
	reqSpoofed.Header.Set("X-Forwarded-For", "127.0.0.1")
	allowedSpoofed, reasonSpoofed, errSpoofed := CheckRequest(reqSpoofed, db, verifier, "login", "")
	if errSpoofed != nil {
		t.Fatalf("unexpected error: %v", errSpoofed)
	}
	if allowedSpoofed {
		t.Errorf("expected allowed=false for spoofed public peer without token, got reason: %s", reasonSpoofed)
	}
}

