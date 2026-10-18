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

type DuckDNSProvider struct {
	httpClient *http.Client
	baseURL    string
}

func NewDuckDNSProvider(client *http.Client) *DuckDNSProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &DuckDNSProvider{
		httpClient: client,
		baseURL:    "https://www.duckdns.org/update",
	}
}

func (p *DuckDNSProvider) Name() string {
	return ProviderDuckDNS
}

func cleanDuckDNSDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".duckdns.org")
	return domain
}

func (p *DuckDNSProvider) Update(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error) {
	if cfg.AuthToken == "" {
		return nil, errors.New("duckdns token is required")
	}
	subdomain := cleanDuckDNSDomain(cfg.Domain)
	if subdomain == "" {
		return nil, errors.New("duckdns domain/subdomain is required")
	}

	params := url.Values{}
	params.Set("domains", subdomain)
	params.Set("token", cfg.AuthToken)

	if cfg.EnableIPv4 && ipv4 != "" {
		params.Set("ip", ipv4)
	}
	if cfg.EnableIPv6 && ipv6 != "" {
		params.Set("ipv6", ipv6)
	}

	reqURL := fmt.Sprintf("%s?%s", p.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Bedrock-Server-Manager-DDNS/1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact duckdns: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return nil, err
	}

	respStr := strings.TrimSpace(string(bodyBytes))
	if respStr != "OK" {
		return nil, fmt.Errorf("duckdns update failed, response: %s", respStr)
	}

	fullDomain := fmt.Sprintf("%s.duckdns.org", subdomain)
	return &SyncResult{
		Success:   true,
		Domain:    fullDomain,
		IPv4:      ipv4,
		IPv6:      ipv6,
		Message:   fmt.Sprintf("DuckDNS updated successfully for %s", fullDomain),
		Timestamp: time.Now().UTC(),
	}, nil
}

func (p *DuckDNSProvider) Test(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error) {
	return p.Update(ctx, cfg, ipv4, ipv6)
}
