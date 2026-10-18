package ddns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type WebhookProvider struct {
	httpClient *http.Client
}

func NewWebhookProvider(client *http.Client) *WebhookProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &WebhookProvider{
		httpClient: client,
	}
}

func (p *WebhookProvider) Name() string {
	return ProviderWebhook
}

func (p *WebhookProvider) Update(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error) {
	rawURL := strings.TrimSpace(cfg.WebhookURL)
	if rawURL == "" {
		return nil, errors.New("custom webhook URL is required")
	}

	primaryIP := ipv4
	if primaryIP == "" {
		primaryIP = ipv6
	}

	// Substitute placeholders in URL
	targetURL := rawURL
	targetURL = strings.ReplaceAll(targetURL, "{ip}", primaryIP)
	targetURL = strings.ReplaceAll(targetURL, "{ipv4}", ipv4)
	targetURL = strings.ReplaceAll(targetURL, "{ipv6}", ipv6)
	targetURL = strings.ReplaceAll(targetURL, "{domain}", cfg.Domain)

	method := strings.ToUpper(strings.TrimSpace(cfg.WebhookMethod))
	if method != http.MethodPost {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid webhook request: %w", err)
	}
	req.Header.Set("User-Agent", "Bedrock-Server-Manager-DDNS/1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	respSnippet := strings.TrimSpace(string(bodyBytes))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webhook returned HTTP %d: %s", resp.StatusCode, respSnippet)
	}

	return &SyncResult{
		Success:   true,
		Domain:    cfg.Domain,
		IPv4:      ipv4,
		IPv6:      ipv6,
		Message:   fmt.Sprintf("Webhook call succeeded (HTTP %d)", resp.StatusCode),
		Timestamp: time.Now().UTC(),
	}, nil
}

func (p *WebhookProvider) Test(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error) {
	return p.Update(ctx, cfg, ipv4, ipv6)
}
