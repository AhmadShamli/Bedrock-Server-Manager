package ddns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

var (
	ErrNoIPv4 = errors.New("failed to detect public IPv4 address from all endpoints")
	ErrNoIPv6 = errors.New("failed to detect public IPv6 address from all endpoints")
)

// IPDetector queries external services with automatic fallback to identify public IPs.
type IPDetector struct {
	httpClient *http.Client
	v4URLs     []string
	v6URLs     []string
}

// NewIPDetector initializes an IP detector with resilient defaults.
func NewIPDetector(timeout time.Duration) *IPDetector {
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	return &IPDetector{
		httpClient: &http.Client{Timeout: timeout},
		v4URLs: []string{
			"https://1.1.1.1/cdn-cgi/trace",
			"https://api.ipify.org",
			"https://icanhazip.com",
		},
		v6URLs: []string{
			"https://[2606:4700:4700::1111]/cdn-cgi/trace",
			"https://api64.ipify.org",
			"https://ipv6.icanhazip.com",
		},
	}
}

// DetectIPv4 checks redundant endpoints to find the caller's public IPv4.
func (d *IPDetector) DetectIPv4(ctx context.Context) (netip.Addr, error) {
	for _, url := range d.v4URLs {
		addr, err := d.fetchAndParseIP(ctx, url, false)
		if err == nil && addr.IsValid() && addr.Is4() && !addr.IsLoopback() && !addr.IsPrivate() {
			return addr, nil
		}
	}
	return netip.Addr{}, ErrNoIPv4
}

// DetectIPv6 checks redundant endpoints to find the caller's public IPv6.
func (d *IPDetector) DetectIPv6(ctx context.Context) (netip.Addr, error) {
	for _, url := range d.v6URLs {
		addr, err := d.fetchAndParseIP(ctx, url, true)
		if err == nil && addr.IsValid() && addr.Is6() && !addr.IsLoopback() && !addr.IsPrivate() && !addr.IsLinkLocalUnicast() {
			return addr, nil
		}
	}
	return netip.Addr{}, ErrNoIPv6
}

// DetectIPs queries both IPv4 and IPv6 according to the requested flags.
func (d *IPDetector) DetectIPs(ctx context.Context, wantV4, wantV6 bool) (ipv4, ipv6 string, err error) {
	if wantV4 {
		v4, errV4 := d.DetectIPv4(ctx)
		if errV4 == nil {
			ipv4 = v4.String()
		} else if !wantV6 {
			return "", "", fmt.Errorf("IPv4 detection failed: %w", errV4)
		}
	}

	if wantV6 {
		v6, errV6 := d.DetectIPv6(ctx)
		if errV6 == nil {
			ipv6 = v6.String()
		} else if !wantV4 {
			return "", "", fmt.Errorf("IPv6 detection failed: %w", errV6)
		}
	}

	if wantV4 && wantV6 && ipv4 == "" && ipv6 == "" {
		return "", "", errors.New("neither public IPv4 nor public IPv6 could be detected")
	}

	return ipv4, ipv6, nil
}

func (d *IPDetector) fetchAndParseIP(ctx context.Context, endpoint string, expectV6 bool) (netip.Addr, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return netip.Addr{}, err
	}
	req.Header.Set("User-Agent", "Bedrock-Server-Manager-DDNS/1.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return netip.Addr{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return netip.Addr{}, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return netip.Addr{}, err
	}

	raw := string(body)

	// Check for Cloudflare /cdn-cgi/trace key=value format
	if strings.Contains(endpoint, "/cdn-cgi/trace") {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ip=") {
				raw = strings.TrimPrefix(line, "ip=")
				break
			}
		}
	}

	raw = strings.TrimSpace(raw)
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid IP format: %w", err)
	}

	if expectV6 && !addr.Is6() {
		return netip.Addr{}, errors.New("expected IPv6 address, received IPv4")
	}
	if !expectV6 && !addr.Is4() {
		return netip.Addr{}, errors.New("expected IPv4 address, received IPv6")
	}

	return addr, nil
}
