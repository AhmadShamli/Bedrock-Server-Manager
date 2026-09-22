package ipresolver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Resolver handles safe client IP resolution based on trusted proxy configuration.
type Resolver struct {
	mu             sync.RWMutex
	proxyMode      string // "direct", "reverse_proxy", "cloudflare"
	trustedProxies []netip.Prefix
	httpClient     *http.Client
}

// NewResolver initializes an IP resolver.
func NewResolver(proxyMode string, trustedProxies []netip.Prefix) *Resolver {
	return &Resolver{
		proxyMode:      proxyMode,
		trustedProxies: trustedProxies,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

// ResolveClientIP safely extracts the genuine client IP from an incoming HTTP request.
func (r *Resolver) ResolveClientIP(req *http.Request) netip.Addr {
	peerIP := extractPeerIP(req.RemoteAddr)
	if !peerIP.IsValid() {
		return netip.Addr{}
	}

	r.mu.RLock()
	mode := r.proxyMode
	trusted := r.isTrusted(peerIP)
	r.mu.RUnlock()

	// If direct mode or peer is not a trusted proxy, always use the direct socket peer IP
	if mode == "direct" || !trusted {
		return peerIP
	}

	// 1. Cloudflare mode: check CF-Connecting-IP
	if mode == "cloudflare" {
		if cfIP := req.Header.Get("CF-Connecting-IP"); cfIP != "" {
			if addr, err := netip.ParseAddr(strings.TrimSpace(cfIP)); err == nil && addr.IsValid() {
				return addr
			}
		}
	}

	// 2. RFC 7239: Forwarded: for=192.0.2.60;proto=http;by=203.0.113.43
	if fwd := req.Header.Get("Forwarded"); fwd != "" {
		if addr := parseRFC7239(fwd); addr.IsValid() {
			return addr
		}
	}

	// 3. X-Forwarded-For: client, proxy1, proxy2
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		// Parse from right to left to find the first untrusted IP
		for i := len(ips) - 1; i >= 0; i-- {
			raw := strings.TrimSpace(ips[i])
			if addr, err := netip.ParseAddr(raw); err == nil && addr.IsValid() {
				r.mu.RLock()
				isHopTrusted := r.isTrusted(addr)
				r.mu.RUnlock()

				if !isHopTrusted {
					return addr
				}
			}
		}
		// If all were trusted, take the leftmost valid IP
		for _, raw := range ips {
			raw = strings.TrimSpace(raw)
			if addr, err := netip.ParseAddr(raw); err == nil && addr.IsValid() {
				return addr
			}
		}
	}

	// 4. X-Real-IP fallback
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		if addr, err := netip.ParseAddr(strings.TrimSpace(xri)); err == nil && addr.IsValid() {
			return addr
		}
	}

	return peerIP
}

// AddTrustedProxies adds additional prefixes (e.g. dynamic Cloudflare CIDRs).
func (r *Resolver) AddTrustedProxies(prefixes []netip.Prefix) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trustedProxies = append(r.trustedProxies, prefixes...)
}

func (r *Resolver) isTrusted(ip netip.Addr) bool {
	// If trustedProxies is empty in reverse_proxy mode, treat loopback and RFC 1918 private as trusted
	if len(r.trustedProxies) == 0 {
		return ip.IsLoopback() || ip.IsPrivate()
	}

	for _, prefix := range r.trustedProxies {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func extractPeerIP(remoteAddr string) netip.Addr {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr
}

func parseRFC7239(header string) netip.Addr {
	parts := strings.Split(header, ";")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && strings.EqualFold(kv[0], "for") {
			raw := strings.Trim(kv[1], "\"")
			if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
				raw = raw[1 : len(raw)-1]
			}
			if addr, err := netip.ParseAddr(raw); err == nil {
				return addr
			}
		}
	}
	return netip.Addr{}
}

// FetchCloudflareIPs downloads the latest official Cloudflare CIDR blocks.
func (r *Resolver) FetchCloudflareIPs(ctx context.Context) ([]netip.Prefix, error) {
	urls := []string{
		"https://www.cloudflare.com/ips-v4",
		"https://www.cloudflare.com/ips-v6",
	}

	var prefixes []netip.Prefix
	for _, u := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := r.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %s: %w", u, err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		lines := strings.Split(string(body), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			prefix, err := netip.ParsePrefix(line)
			if err == nil {
				prefixes = append(prefixes, prefix)
			}
		}
	}
	return prefixes, nil
}
