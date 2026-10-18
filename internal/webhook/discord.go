package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	ColorEmerald = 1096065  // #10b981
	ColorRed     = 15680580 // #ef4444
	ColorCyan    = 440020   // #06b6d4
	ColorAmber   = 16096779 // #f59e0b
)

// EmbedField represents a field inside a Discord embed.
type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// Embed represents a Discord webhook rich embed.
type Embed struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color"`
	Fields      []EmbedField `json:"fields,omitempty"`
	Timestamp   string       `json:"timestamp"`
	Footer      struct {
		Text string `json:"text"`
	} `json:"footer"`
}

// Payload represents the root Discord webhook payload.
type Payload struct {
	Username  string  `json:"username,omitempty"`
	AvatarURL string  `json:"avatar_url,omitempty"`
	Embeds    []Embed `json:"embeds"`
}

// Dispatcher manages outbound webhook notifications.
type Dispatcher struct {
	httpClient *http.Client
}

// NewDispatcher creates a webhook dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send dispatches an embed payload to a Discord webhook URL.
func (d *Dispatcher) Send(ctx context.Context, webhookURL string, embed Embed) error {
	if webhookURL == "" {
		return nil
	}

	embed.Timestamp = time.Now().UTC().Format(time.RFC3339)
	embed.Footer.Text = "Bedrock Server Manager (BSM)"

	payload := Payload{
		Username: "Bedrock Server Manager",
		Embeds:   []Embed{embed},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook responded with status %d", resp.StatusCode)
	}

	return nil
}

func (d *Dispatcher) NotifyServerStarted(ctx context.Context, webhookURL, name, id string, port int) error {
	return d.Send(ctx, webhookURL, Embed{
		Title:       fmt.Sprintf("🟢 Server Started: %s", name),
		Description: fmt.Sprintf("Instance **%s** is now online and accepting connections.", id),
		Color:       ColorEmerald,
		Fields: []EmbedField{
			{Name: "UDP Port", Value: fmt.Sprint(port), Inline: true},
			{Name: "Status", Value: "Running", Inline: true},
		},
	})
}

func (d *Dispatcher) NotifyServerStopped(ctx context.Context, webhookURL, name, id string) error {
	return d.Send(ctx, webhookURL, Embed{
		Title:       fmt.Sprintf("⏹️ Server Stopped: %s", name),
		Description: fmt.Sprintf("Instance **%s** was stopped cleanly.", id),
		Color:       ColorAmber,
	})
}

func (d *Dispatcher) NotifyServerCrashed(ctx context.Context, webhookURL, name, id, logTail string) error {
	return d.Send(ctx, webhookURL, Embed{
		Title:       fmt.Sprintf("🚨 Server Crashed: %s", name),
		Description: fmt.Sprintf("Instance **%s** encountered an unexpected exit.", id),
		Color:       ColorRed,
		Fields: []EmbedField{
			{Name: "Recent Logs", Value: fmt.Sprintf("```\n%s\n```", logTail), Inline: false},
		},
	})
}

func (d *Dispatcher) NotifyPlayerJoined(ctx context.Context, webhookURL, serverName, gamertag string, onlineCount int) error {
	return d.Send(ctx, webhookURL, Embed{
		Title:       fmt.Sprintf("👋 Player Joined: %s", gamertag),
		Description: fmt.Sprintf("**%s** joined **%s**.", gamertag, serverName),
		Color:       ColorCyan,
		Fields: []EmbedField{
			{Name: "Online Count", Value: fmt.Sprint(onlineCount), Inline: true},
		},
	})
}

func (d *Dispatcher) NotifyPlayerLeft(ctx context.Context, webhookURL, serverName, gamertag string, onlineCount int) error {
	return d.Send(ctx, webhookURL, Embed{
		Title:       fmt.Sprintf("🏃 Player Left: %s", gamertag),
		Description: fmt.Sprintf("**%s** left **%s**.", gamertag, serverName),
		Color:       ColorAmber,
		Fields: []EmbedField{
			{Name: "Online Count", Value: fmt.Sprint(onlineCount), Inline: true},
		},
	})
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (d *Dispatcher) NotifyResourceAlert(ctx context.Context, webhookURL, serverName, serverID string, ramUsed, ramLimit int64, ramPct float64, cpuUsed, cpuLimitCores, cpuPct float64) error {
	return d.Send(ctx, webhookURL, Embed{
		Title:       fmt.Sprintf("⚠️ Resource Alert: %s", serverName),
		Description: fmt.Sprintf("Server **%s** allocated resources are nearly full!", serverID),
		Color:       ColorAmber,
		Fields: []EmbedField{
			{Name: "RAM Usage vs Allocated", Value: fmt.Sprintf("%s / %s (%.1f%%)", formatBytes(ramUsed), formatBytes(ramLimit), ramPct), Inline: true},
			{Name: "CPU Usage vs Allocated", Value: fmt.Sprintf("%.1f%% / %.1f%% (%.1f%%)", cpuUsed, cpuLimitCores*100.0, cpuPct), Inline: true},
		},
	})
}

func (d *Dispatcher) NotifyFailedLogin(ctx context.Context, webhookURL, username, clientIP string, lockedOut bool) error {
	status := "Attempt Failed"
	if lockedOut {
		status = "IP Temporarily Locked Out"
	}
	return d.Send(ctx, webhookURL, Embed{
		Title:       "🚨 Security Alert: Failed Admin Login",
		Description: fmt.Sprintf("A login attempt failed for username **%s** from IP **%s**.", username, clientIP),
		Color:       ColorRed,
		Fields: []EmbedField{
			{Name: "Target Username", Value: username, Inline: true},
			{Name: "Client IP", Value: clientIP, Inline: true},
			{Name: "Status", Value: status, Inline: true},
		},
	})
}

func (d *Dispatcher) NotifyCircuitBreakerTripped(ctx context.Context, webhookURL, serverID, clientIP string, distThreshold int) error {
	return d.Send(ctx, webhookURL, Embed{
		Title:       "🛡️ Defense Triggered: Distributed Attack Circuit Breaker",
		Description: fmt.Sprintf("Knock portal brute-force circuit breaker tripped on server **%s** after %d distinct IPs failed access checks.", serverID, distThreshold),
		Color:       ColorRed,
		Fields: []EmbedField{
			{Name: "Last Offending IP", Value: clientIP, Inline: true},
			{Name: "Action", Value: "New authorizations paused", Inline: true},
		},
	})
}

func (d *Dispatcher) NotifyKnockFailed(ctx context.Context, webhookURL, serverName, serverID, clientIP, gamertag string, lockedOut bool) error {
	status := "Failed Attempt"
	if lockedOut {
		status = "Visitor IP Locked Out"
	}
	return d.Send(ctx, webhookURL, Embed{
		Title:       "⚠️ Port Gate Alert: Invalid Passphrase",
		Description: fmt.Sprintf("Failed knock access attempt on server **%s** (%s) from IP **%s**.", serverName, serverID, clientIP),
		Color:       ColorAmber,
		Fields: []EmbedField{
			{Name: "Client IP", Value: clientIP, Inline: true},
			{Name: "Gamertag", Value: gamertag, Inline: true},
			{Name: "Status", Value: status, Inline: true},
		},
	})
}

// NotifyDDNSSynced sends a notification when public IP changes or DDNS updates successfully.
func (d *Dispatcher) NotifyDDNSSynced(ctx context.Context, webhookURL, domain, provider, ipv4, ipv6 string, ipChanged bool) error {
	title := fmt.Sprintf("🌐 Dynamic DNS Synchronized: %s", domain)
	desc := fmt.Sprintf("Dynamic DNS records successfully updated via **%s**.", provider)
	color := ColorEmerald
	if ipChanged {
		title = fmt.Sprintf("🔄 Public IP Changed: %s", domain)
		desc = fmt.Sprintf("Host server public IP address changed and DDNS was synchronized via **%s**.", provider)
		color = ColorCyan
	}

	fields := []EmbedField{
		{Name: "Domain", Value: domain, Inline: true},
		{Name: "Provider", Value: provider, Inline: true},
	}
	if ipv4 != "" {
		fields = append(fields, EmbedField{Name: "IPv4 Address", Value: ipv4, Inline: true})
	}
	if ipv6 != "" {
		fields = append(fields, EmbedField{Name: "IPv6 Address", Value: ipv6, Inline: true})
	}

	return d.Send(ctx, webhookURL, Embed{
		Title:       title,
		Description: desc,
		Color:       color,
		Fields:      fields,
	})
}

// NotifyDDNSFailed sends an alert when a DDNS update fails.
func (d *Dispatcher) NotifyDDNSFailed(ctx context.Context, webhookURL, domain, provider string, syncErr error) error {
	errMsg := "Unknown error"
	if syncErr != nil {
		errMsg = syncErr.Error()
	}
	return d.Send(ctx, webhookURL, Embed{
		Title:       "🚨 Dynamic DNS Sync Failed",
		Description: fmt.Sprintf("Failed to update Dynamic DNS records for domain **%s** via **%s**.", domain, provider),
		Color:       ColorRed,
		Fields: []EmbedField{
			{Name: "Domain", Value: domain, Inline: true},
			{Name: "Provider", Value: provider, Inline: true},
			{Name: "Error Details", Value: errMsg, Inline: false},
		},
	})
}


