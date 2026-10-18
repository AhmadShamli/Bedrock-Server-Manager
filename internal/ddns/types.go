package ddns

import (
	"context"
	"time"
)

// Provider names
const (
	ProviderCloudflare = "cloudflare"
	ProviderDuckDNS    = "duckdns"
	ProviderNoIP       = "noip"
	ProviderWebhook    = "webhook"
)

// Status states
const (
	StatusIdle    = "idle"
	StatusSyncing = "syncing"
	StatusSuccess = "success"
	StatusError   = "error"
)

// Config represents the Dynamic DNS service configuration.
type Config struct {
	Enabled             bool   `json:"enabled"`
	Provider            string `json:"provider"`
	Domain              string `json:"domain"`
	EnableIPv4          bool   `json:"enable_ipv4"`
	EnableIPv6          bool   `json:"enable_ipv6"`
	IntervalMinutes     int    `json:"interval_minutes"`
	AutoSyncGameAddress bool   `json:"auto_sync_game_address"`

	// Provider-specific credentials (masked on public exposure)
	AuthToken     string `json:"auth_token,omitempty"`
	ZoneID        string `json:"zone_id,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	WebhookURL    string `json:"webhook_url,omitempty"`
	WebhookMethod string `json:"webhook_method,omitempty"`
}

// SyncResult conveys the outcome of a provider update attempt.
type SyncResult struct {
	Success   bool      `json:"success"`
	IPv4      string    `json:"ipv4,omitempty"`
	IPv6      string    `json:"ipv6,omitempty"`
	Domain    string    `json:"domain"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Status represents the runtime status of the DDNS worker and detected IPs.
type Status struct {
	Enabled             bool       `json:"enabled"`
	Provider            string     `json:"provider"`
	Domain              string     `json:"domain"`
	CurrentIPv4         string     `json:"current_ipv4"`
	CurrentIPv6         string     `json:"current_ipv6"`
	LastSyncTime        *time.Time `json:"last_sync_time,omitempty"`
	NextSyncTime        *time.Time `json:"next_sync_time,omitempty"`
	Status              string     `json:"status"` // idle, syncing, success, error
	LastError           string     `json:"last_error,omitempty"`
	IntervalMinutes     int        `json:"interval_minutes"`
	AutoSyncGameAddress bool       `json:"auto_sync_game_address"`
}

// Provider defines the interface for updating external DNS records.
type Provider interface {
	Name() string
	Update(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error)
	Test(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error)
}
