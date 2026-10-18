package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
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
	SettingTurnstileBypassLocal     = "turnstile_bypass_local"
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
	BypassLocal     bool   `json:"bypass_local"`
	IsLocal         bool   `json:"is_local,omitempty"`
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
		BypassLocal:     isTrue(SettingTurnstileBypassLocal, false),
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

	// If local / LAN bypass is enabled and client IP is local or LAN, allow immediately.
	if cfg.BypassLocal && IsLocalOrLAN(clientIP) {
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

// IsLocalOrLAN returns true if the IP address represents loopback (127.0.0.0/8, ::1),
// private network (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7),
// link-local (169.254.0.0/16, fe80::/10), or unspecified (0.0.0.0, ::).
func IsLocalOrLAN(ipStr string) bool {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" || ipStr == "localhost" {
		return true
	}
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = host
	}
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return ipStr == "localhost" || ipStr == "127.0.0.1" || ipStr == "::1"
	}
	addr = addr.Unmap()
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsUnspecified()
}

// IsRequestLocalOrLAN inspects an incoming HTTP request and its direct socket peer address.
// It verifies that the direct socket peer is a genuine local/LAN address AND checks whether
// that local peer is actually a reverse proxy forwarding an external/public client.
//
// If proxy forwarding headers (CF-Connecting-IP, True-Client-IP, X-Real-IP, X-Forwarded-For,
// or RFC 7239 Forwarded) are present, the original forwarded client IP is validated.
// If any forwarded IP in the chain is non-local (public internet), this returns false.
func IsRequestLocalOrLAN(req *http.Request) bool {
	if req == nil {
		return false
	}

	// 1. Check socket peer IP
	peerHost := req.RemoteAddr
	if host, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		peerHost = host
	}
	peerHost = strings.TrimSpace(peerHost)
	if peerHost == "" {
		return false
	}

	peerAddr, err := netip.ParseAddr(peerHost)
	if err != nil {
		if peerHost != "localhost" && peerHost != "127.0.0.1" && peerHost != "::1" {
			return false
		}
	} else {
		peerAddr = peerAddr.Unmap()
		isPeerLocal := peerAddr.IsLoopback() || peerAddr.IsPrivate() || peerAddr.IsLinkLocalUnicast() || peerAddr.IsUnspecified()
		if !isPeerLocal {
			// Direct peer is a public / non-local IP. Cannot be a trusted local client.
			return false
		}
	}

	// 2. Peer is a local/LAN IP. Check if this local peer is a reverse proxy forwarding an external client.

	// Check CF-Connecting-IP (Cloudflare CDN / Cloudflare Tunnel)
	if cfIP := strings.TrimSpace(req.Header.Get("CF-Connecting-IP")); cfIP != "" {
		if !IsLocalOrLAN(cfIP) {
			return false
		}
	}

	// Check True-Client-IP
	if tcIP := strings.TrimSpace(req.Header.Get("True-Client-IP")); tcIP != "" {
		if !IsLocalOrLAN(tcIP) {
			return false
		}
	}

	// Check X-Real-IP
	if xri := strings.TrimSpace(req.Header.Get("X-Real-IP")); xri != "" {
		if !IsLocalOrLAN(xri) {
			return false
		}
	}

	// Check X-Forwarded-For (client, proxy1, proxy2...)
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		for _, raw := range ips {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			if !IsLocalOrLAN(raw) {
				return false
			}
		}
	}

	// Check RFC 7239 Forwarded: for=192.0.2.60;proto=http;by=203.0.113.43
	if fwd := req.Header.Get("Forwarded"); fwd != "" {
		parts := strings.Split(fwd, ";")
		for _, part := range parts {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) == 2 && strings.EqualFold(kv[0], "for") {
				raw := strings.Trim(kv[1], "\"")
				if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
					raw = raw[1 : len(raw)-1]
				}
				if raw != "" && !IsLocalOrLAN(raw) {
					return false
				}
			}
		}
	}

	// Check X-Forwarded-Host if proxy headers (like Via or X-Forwarded-Proto) exist without client IP
	if xfh := strings.TrimSpace(req.Header.Get("X-Forwarded-Host")); xfh != "" {
		host := xfh
		if h, _, err := net.SplitHostPort(xfh); err == nil {
			host = h
		}
		hostLower := strings.ToLower(host)
		if hostLower != "localhost" && !strings.HasSuffix(hostLower, ".local") && !IsLocalOrLAN(host) {
			if req.Header.Get("X-Forwarded-Proto") != "" || req.Header.Get("Via") != "" {
				return false
			}
		}
	}

	return true
}

// CheckRequest evaluates whether an HTTP request requires Turnstile verification and validates it.
// It verifies that the request originates from a genuine local/LAN client and ensures that local
// reverse proxies forwarding external traffic are not bypassed.
func CheckRequest(r *http.Request, db *database.ManagerDB, verifier *Verifier, action, token string) (bool, string, error) {
	if db == nil || r == nil {
		return true, "", nil
	}

	cfg, err := LoadConfig(r.Context(), db)
	if err != nil {
		return false, "failed to load security settings", err
	}

	// If Turnstile is globally disabled, allow.
	if !cfg.Enabled {
		return true, "", nil
	}

	// If local / LAN bypass is enabled, check if the request is genuinely from a local or LAN client.
	if cfg.BypassLocal && IsRequestLocalOrLAN(r) {
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

	secretKey, _ := db.GetSetting(r.Context(), SettingTurnstileSecretKey)
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

	clientIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = host
	}

	res, err := verifier.Verify(r.Context(), secretKey, token, clientIP, "")
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
