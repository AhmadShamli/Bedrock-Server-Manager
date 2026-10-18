package ddns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type CloudflareProvider struct {
	httpClient *http.Client
	baseURL    string
}

func NewCloudflareProvider(client *http.Client) *CloudflareProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &CloudflareProvider{
		httpClient: client,
		baseURL:    "https://api.cloudflare.com/client/v4",
	}
}

func (p *CloudflareProvider) Name() string {
	return ProviderCloudflare
}

type cfDNSListResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Type    string `json:"type"`
		Content string `json:"content"`
		Proxied bool   `json:"proxied"`
	} `json:"result"`
}

type cfDNSUpdatePayload struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

type cfDNSActionResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func (p *CloudflareProvider) Update(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error) {
	if cfg.AuthToken == "" {
		return nil, errors.New("cloudflare auth token is required")
	}
	if cfg.ZoneID == "" {
		return nil, errors.New("cloudflare zone ID is required")
	}
	domain := strings.TrimSpace(cfg.Domain)
	if domain == "" {
		return nil, errors.New("domain is required")
	}

	var actionsTaken []string

	// Sync IPv4 A record
	if cfg.EnableIPv4 && ipv4 != "" {
		action, err := p.syncRecord(ctx, cfg.ZoneID, cfg.AuthToken, domain, "A", ipv4)
		if err != nil {
			return nil, fmt.Errorf("failed to sync A record for %s: %w", domain, err)
		}
		actionsTaken = append(actionsTaken, fmt.Sprintf("A -> %s (%s)", ipv4, action))
	}

	// Sync IPv6 AAAA record
	if cfg.EnableIPv6 && ipv6 != "" {
		action, err := p.syncRecord(ctx, cfg.ZoneID, cfg.AuthToken, domain, "AAAA", ipv6)
		if err != nil {
			return nil, fmt.Errorf("failed to sync AAAA record for %s: %w", domain, err)
		}
		actionsTaken = append(actionsTaken, fmt.Sprintf("AAAA -> %s (%s)", ipv6, action))
	}

	if len(actionsTaken) == 0 {
		return nil, errors.New("no records updated: neither IPv4 nor IPv6 were available/enabled")
	}

	return &SyncResult{
		Success:   true,
		IPv4:      ipv4,
		IPv6:      ipv6,
		Domain:    domain,
		Message:   strings.Join(actionsTaken, "; "),
		Timestamp: time.Now().UTC(),
	}, nil
}

func (p *CloudflareProvider) Test(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error) {
	if cfg.AuthToken == "" {
		return nil, errors.New("cloudflare auth token is required")
	}
	if cfg.ZoneID == "" {
		return nil, errors.New("cloudflare zone ID is required")
	}

	// Verify zone access by listing zone details or dns records
	url := fmt.Sprintf("%s/zones/%s/dns_records?per_page=1", p.baseURL, cfg.ZoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var listResp cfDNSListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("invalid json response from Cloudflare: %w", err)
	}

	if !listResp.Success {
		var errMsgs []string
		for _, e := range listResp.Errors {
			errMsgs = append(errMsgs, fmt.Sprintf("[%d] %s", e.Code, e.Message))
		}
		return nil, fmt.Errorf("Cloudflare API error: %s", strings.Join(errMsgs, ", "))
	}

	return &SyncResult{
		Success:   true,
		Domain:    cfg.Domain,
		IPv4:      ipv4,
		IPv6:      ipv6,
		Message:   "Cloudflare credentials and Zone ID verified successfully",
		Timestamp: time.Now().UTC(),
	}, nil
}

func (p *CloudflareProvider) syncRecord(ctx context.Context, zoneID, token, domain, recordType, ip string) (string, error) {
	// 1. List existing records matching name and type
	listURL := fmt.Sprintf("%s/zones/%s/dns_records?name=%s&type=%s", p.baseURL, zoneID, domain, recordType)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var listResp cfDNSListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return "", fmt.Errorf("failed to parse dns list: %w", err)
	}

	if !listResp.Success {
		var errMsgs []string
		for _, e := range listResp.Errors {
			errMsgs = append(errMsgs, e.Message)
		}
		return "", fmt.Errorf("cloudflare error: %s", strings.Join(errMsgs, ", "))
	}

	payload := cfDNSUpdatePayload{
		Name:    domain,
		Type:    recordType,
		Content: ip,
		TTL:     1,     // Auto TTL
		Proxied: false, // DNS only (required for Bedrock UDP)
	}
	payloadBytes, _ := json.Marshal(payload)

	// If record exists, update it if needed
	if len(listResp.Result) > 0 {
		record := listResp.Result[0]
		if record.Content == ip && !record.Proxied {
			return "unchanged", nil
		}

		updateURL := fmt.Sprintf("%s/zones/%s/dns_records/%s", p.baseURL, zoneID, record.ID)
		putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, updateURL, bytes.NewReader(payloadBytes))
		if err != nil {
			return "", err
		}
		putReq.Header.Set("Authorization", "Bearer "+token)
		putReq.Header.Set("Content-Type", "application/json")

		putResp, err := p.httpClient.Do(putReq)
		if err != nil {
			return "", err
		}
		defer putResp.Body.Close()

		putBody, _ := io.ReadAll(putResp.Body)
		var putResult cfDNSActionResponse
		_ = json.Unmarshal(putBody, &putResult)
		if !putResult.Success {
			var errMsgs []string
			for _, e := range putResult.Errors {
				errMsgs = append(errMsgs, e.Message)
			}
			return "", fmt.Errorf("failed to update record: %s", strings.Join(errMsgs, ", "))
		}
		return "updated", nil
	}

	// Record does not exist: create it
	createURL := fmt.Sprintf("%s/zones/%s/dns_records", p.baseURL, zoneID)
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}
	postReq.Header.Set("Authorization", "Bearer "+token)
	postReq.Header.Set("Content-Type", "application/json")

	postResp, err := p.httpClient.Do(postReq)
	if err != nil {
		return "", err
	}
	defer postResp.Body.Close()

	postBody, _ := io.ReadAll(postResp.Body)
	var postResult cfDNSActionResponse
	_ = json.Unmarshal(postBody, &postResult)
	if !postResult.Success {
		var errMsgs []string
		for _, e := range postResult.Errors {
			errMsgs = append(errMsgs, e.Message)
		}
		return "", fmt.Errorf("failed to create record: %s", strings.Join(errMsgs, ", "))
	}
	return "created", nil
}
