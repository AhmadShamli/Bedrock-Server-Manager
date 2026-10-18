package ddns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type NoIPProvider struct {
	httpClient *http.Client
	baseURL    string
}

func NewNoIPProvider(client *http.Client) *NoIPProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &NoIPProvider{
		httpClient: client,
		baseURL:    "https://dynupdate.no-ip.com/nic/update",
	}
}

func (p *NoIPProvider) Name() string {
	return ProviderNoIP
}

func (p *NoIPProvider) Update(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error) {
	if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("no-ip username and password are required")
	}
	domain := strings.TrimSpace(cfg.Domain)
	if domain == "" {
		return nil, errors.New("no-ip hostname is required")
	}

	ipToUpdate := ipv4
	if ipToUpdate == "" && cfg.EnableIPv6 {
		ipToUpdate = ipv6
	}
	if ipToUpdate == "" {
		return nil, errors.New("no valid IP available for No-IP update")
	}

	params := url.Values{}
	params.Set("hostname", domain)
	params.Set("myip", ipToUpdate)

	reqURL := fmt.Sprintf("%s?%s", p.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	// No-IP strictly requires HTTP Basic Auth and custom User-Agent
	req.SetBasicAuth(cfg.Username, cfg.Password)
	req.Header.Set("User-Agent", "Bedrock-Server-Manager/2.0 bsm-admin@internal.local")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact No-IP: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return nil, err
	}

	respStr := strings.TrimSpace(string(bodyBytes))
	parts := strings.Fields(respStr)
	status := ""
	if len(parts) > 0 {
		status = parts[0]
	}

	switch status {
	case "good":
		return &SyncResult{
			Success:   true,
			Domain:    domain,
			IPv4:      ipv4,
			IPv6:      ipv6,
			Message:   fmt.Sprintf("No-IP updated successfully: %s", respStr),
			Timestamp: time.Now().UTC(),
		}, nil
	case "nochg":
		return &SyncResult{
			Success:   true,
			Domain:    domain,
			IPv4:      ipv4,
			IPv6:      ipv6,
			Message:   fmt.Sprintf("No-IP already up-to-date: %s", respStr),
			Timestamp: time.Now().UTC(),
		}, nil
	case "badauth":
		return nil, errors.New("No-IP authentication failed: invalid username or password")
	case "nohost":
		return nil, fmt.Errorf("No-IP error: hostname '%s' not found on account", domain)
	case "abuse":
		return nil, errors.New("No-IP error: account blocked due to abuse or excessive update frequency")
	case "!donator":
		return nil, errors.New("No-IP error: requested feature requires a paid donator account")
	case "911":
		return nil, errors.New("No-IP server error (code 911), please retry later")
	default:
		return nil, fmt.Errorf("No-IP returned unrecognized response: %s", respStr)
	}
}

func (p *NoIPProvider) Test(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error) {
	return p.Update(ctx, cfg, ipv4, ipv6)
}
