package ddns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDuckDNSProvider(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("domains") == "myserver" && q.Get("token") == "test-token" && q.Get("ip") == "198.51.100.1" {
			w.Write([]byte("OK"))
			return
		}
		w.Write([]byte("KO"))
	}))
	defer ts.Close()

	p := &DuckDNSProvider{
		httpClient: ts.Client(),
		baseURL:    ts.URL,
	}

	cfg := &Config{
		AuthToken:  "test-token",
		Domain:     "myserver.duckdns.org",
		EnableIPv4: true,
	}

	res, err := p.Update(context.Background(), cfg, "198.51.100.1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got false")
	}
	if res.Domain != "myserver.duckdns.org" {
		t.Fatalf("expected myserver.duckdns.org, got %s", res.Domain)
	}

	// Test KO failure
	cfg.AuthToken = "wrong-token"
	_, err = p.Update(context.Background(), cfg, "198.51.100.1", "")
	if err == nil {
		t.Fatalf("expected error for wrong token, got nil")
	}
}

func TestNoIPProvider(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user1" || pass != "secret" {
			w.Write([]byte("badauth"))
			return
		}
		if r.URL.Query().Get("hostname") != "mybds.ddns.net" {
			w.Write([]byte("nohost"))
			return
		}
		w.Write([]byte("good 203.0.113.10"))
	}))
	defer ts.Close()

	p := &NoIPProvider{
		httpClient: ts.Client(),
		baseURL:    ts.URL,
	}

	cfg := &Config{
		Username:   "user1",
		Password:   "secret",
		Domain:     "mybds.ddns.net",
		EnableIPv4: true,
	}

	res, err := p.Update(context.Background(), cfg, "203.0.113.10", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}

	// Test bad auth
	cfg.Password = "wrong"
	_, err = p.Update(context.Background(), cfg, "203.0.113.10", "")
	if err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("expected auth error, got: %v", err)
	}
}

func TestWebhookProvider(t *testing.T) {
	var receivedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer ts.Close()

	p := NewWebhookProvider(ts.Client())
	cfg := &Config{
		WebhookURL:    ts.URL + "/update?ip={ip}&ipv4={ipv4}&ipv6={ipv6}&host={domain}",
		WebhookMethod: "GET",
		Domain:        "play.example.com",
	}

	res, err := p.Update(context.Background(), cfg, "192.0.2.1", "2001:db8::1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	if !strings.Contains(receivedURL, "ip=192.0.2.1") || !strings.Contains(receivedURL, "ipv6=2001:db8::1") || !strings.Contains(receivedURL, "host=play.example.com") {
		t.Fatalf("placeholder replacement failed, url: %s", receivedURL)
	}
}

func TestCloudflareProvider(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer cf-token" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"errors":  []map[string]string{{"message": "invalid token"}},
			})
			return
		}

		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/dns_records") {
			// Return 1 existing record
			json.NewEncoder(w).Encode(cfDNSListResponse{
				Success: true,
				Result: []struct {
					ID      string `json:"id"`
					Name    string `json:"name"`
					Type    string `json:"type"`
					Content string `json:"content"`
					Proxied bool   `json:"proxied"`
				}{
					{
						ID:      "rec-123",
						Name:    "bds.example.com",
						Type:    "A",
						Content: "192.0.2.5",
						Proxied: false,
					},
				},
			})
			return
		}

		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/dns_records/rec-123") {
			var body cfDNSUpdatePayload
			json.NewDecoder(r.Body).Decode(&body)
			if body.Content == "192.0.2.100" && !body.Proxied {
				json.NewEncoder(w).Encode(cfDNSActionResponse{Success: true})
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	p := &CloudflareProvider{
		httpClient: ts.Client(),
		baseURL:    ts.URL,
	}

	cfg := &Config{
		AuthToken:  "cf-token",
		ZoneID:     "zone-abc",
		Domain:     "bds.example.com",
		EnableIPv4: true,
	}

	res, err := p.Update(context.Background(), cfg, "192.0.2.100", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	if !strings.Contains(res.Message, "updated") {
		t.Fatalf("expected updated action, got: %s", res.Message)
	}
}
