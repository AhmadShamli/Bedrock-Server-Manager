package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
)

const (
	// DefaultSiteVerifyURL is the official Cloudflare Turnstile verification endpoint.
	DefaultSiteVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

	// Setting keys stored in system_settings database table.
	SettingTurnstileEnabled         = "turnstile_enabled"
	SettingTurnstileSiteKey         = "turnstile_site_key"
	SettingTurnstileSecretKey       = "turnstile_secret_key"
	SettingTurnstileLoginEnabled    = "turnstile_login_enabled"
	SettingTurnstileRegisterEnabled = "turnstile_register_enabled"
	SettingTurnstileSetupEnabled    = "turnstile_setup_enabled"
	SettingTurnstileKnockEnabled    = "turnstile_knock_enabled"
	SettingTurnstileFailOpen        = "turnstile_fail_open"
)

// SiteVerifyResponse represents the response payload returned by Cloudflare siteverify.
type SiteVerifyResponse struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	Action      string   `json:"action"`
	CData       string   `json:"cdata"`
}

// Config represents public and internal Turnstile configuration.
type Config struct {
	Enabled         bool   `json:"enabled"`
	SiteKey         string `json:"site_key"`
	LoginEnabled    bool   `json:"login_enabled"`
	RegisterEnabled bool   `json:"register_enabled"`
	SetupEnabled    bool   `json:"setup_enabled"`
	KnockEnabled    bool   `json:"knock_enabled"`
	FailOpen        bool   `json:"fail_open"`
}

// Verifier executes verification queries against Cloudflare's Turnstile siteverify API.
type Verifier struct {
	httpClient *http.Client
	verifyURL  string
}

// NewVerifier creates a new Turnstile Verifier with default settings.
func NewVerifier(httpClient ...*http.Client) *Verifier {
	client := &http.Client{
		Timeout: 6 * time.Second,
	}
	if len(httpClient) > 0 && httpClient[0] != nil {
		client = httpClient[0]
	}
	return &Verifier{
		httpClient: client,
		verifyURL:  DefaultSiteVerifyURL,
	}
}

// SetVerifyURL overrides the endpoint URL (used primarily for unit testing).
func (v *Verifier) SetVerifyURL(u string) {
	v.verifyURL = u
}

// Verify sends the token, secret key, and optional remote client IP to Cloudflare siteverify.
func (v *Verifier) Verify(ctx context.Context, secretKey, responseToken, remoteIP, idempotencyKey string) (*SiteVerifyResponse, error) {
	if strings.TrimSpace(secretKey) == "" {
		return nil, fmt.Errorf("secret key is required")
	}
	if strings.TrimSpace(responseToken) == "" {
		return &SiteVerifyResponse{
			Success:    false,
			ErrorCodes: []string{"missing-input-response"},
		}, nil
	}

	form := url.Values{}
	form.Set("secret", strings.TrimSpace(secretKey))
	form.Set("response", strings.TrimSpace(responseToken))
	if strings.TrimSpace(remoteIP) != "" {
		form.Set("remoteip", strings.TrimSpace(remoteIP))
	}
	if strings.TrimSpace(idempotencyKey) != "" {
		form.Set("idempotency_key", strings.TrimSpace(idempotencyKey))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create turnstile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("turnstile verification request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read turnstile response: %w", err)
	}

	var verifyResp SiteVerifyResponse
	if err := json.Unmarshal(bodyBytes, &verifyResp); err != nil {
		return nil, fmt.Errorf("failed to decode turnstile response JSON: %w (status: %d)", err, resp.StatusCode)
	}

	return &verifyResp, nil
}

// LoadConfig loads the active Turnstile configuration from system_settings.
func LoadConfig(ctx context.Context, db *database.ManagerDB) (*Config, error) {
	if db == nil {
		return &Config{}, nil
	}

	settings, err := db.GetAllSettings(ctx)
	if err != nil {
		return nil, err
	}

	isTrue := func(key string, def bool) bool {
		val, exists := settings[key]
		if !exists {
			return def
		}
		val = strings.ToLower(strings.TrimSpace(val))
		return val == "true" || val == "1" || val == "yes" || val == "on"
	}

	return &Config{
		Enabled:         isTrue(SettingTurnstileEnabled, false),
		SiteKey:         strings.TrimSpace(settings[SettingTurnstileSiteKey]),
		LoginEnabled:    isTrue(SettingTurnstileLoginEnabled, true),
		RegisterEnabled: isTrue(SettingTurnstileRegisterEnabled, true),
		SetupEnabled:    isTrue(SettingTurnstileSetupEnabled, true),
		KnockEnabled:    isTrue(SettingTurnstileKnockEnabled, true),
		FailOpen:        isTrue(SettingTurnstileFailOpen, false),
	}, nil
}

// Check evaluates whether a request requires Turnstile verification and validates it.
// Returns (allowed, failureReason, error).
func Check(ctx context.Context, db *database.ManagerDB, verifier *Verifier, action, token, clientIP string) (bool, string, error) {
	if db == nil {
		return true, "", nil
	}

	cfg, err := LoadConfig(ctx, db)
	if err != nil {
		return false, "failed to load security settings", err
	}

	// If Turnstile is globally disabled, allow.
	if !cfg.Enabled {
		return true, "", nil
	}

	// Check if this specific action has protection enabled.
	actionEnabled := false
	switch action {
	case "login":
		actionEnabled = cfg.LoginEnabled
	case "register":
		actionEnabled = cfg.RegisterEnabled
	case "setup":
		actionEnabled = cfg.SetupEnabled
	case "knock":
		actionEnabled = cfg.KnockEnabled
	default:
		actionEnabled = true
	}

	if !actionEnabled {
		return true, "", nil
	}

	secretKey, _ := db.GetSetting(ctx, SettingTurnstileSecretKey)
	secretKey = strings.TrimSpace(secretKey)

	// If secret key is not configured, bypass gracefully to prevent accidental lockout.
	if secretKey == "" {
		return true, "", nil
	}

	// Token must be provided by the client when action is protected.
	if strings.TrimSpace(token) == "" {
		return false, "Cloudflare Turnstile verification challenge is required", nil
	}

	if verifier == nil {
		verifier = NewVerifier()
	}

	res, err := verifier.Verify(ctx, secretKey, token, clientIP, "")
	if err != nil {
		// Outage / network error occurred
		if cfg.FailOpen {
			return true, "turnstile_verification_unreachable_fail_open", nil
		}
		return false, fmt.Sprintf("Turnstile verification service is currently unavailable: %s", err.Error()), err
	}

	if !res.Success {
		reason := "Cloudflare Turnstile challenge verification failed"
		if len(res.ErrorCodes) > 0 {
			reason = fmt.Sprintf("Turnstile verification failed (%s)", strings.Join(res.ErrorCodes, ", "))
		}
		return false, reason, nil
	}

	return true, "", nil
}

// ParseBool parses common truthy string values.
func ParseBool(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	return v == "yes" || v == "on"
}
