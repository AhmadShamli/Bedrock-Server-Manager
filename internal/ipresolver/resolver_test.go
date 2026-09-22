package ipresolver

import (
	"net/http"
	"net/netip"
	"testing"
)

func TestDirectMode(t *testing.T) {
	resolver := NewResolver("direct", nil)

	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.195")

	ip := resolver.ResolveClientIP(req)
	if ip.String() != "192.0.2.1" {
		t.Fatalf("expected direct socket IP '192.0.2.1', got '%s'", ip.String())
	}
}

func TestReverseProxyMode(t *testing.T) {
	// RemoteAddr is 127.0.0.1 (trusted loopback proxy)
	resolver := NewResolver("reverse_proxy", nil)

	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-For", "203.0.113.195, 10.0.0.1")

	ip := resolver.ResolveClientIP(req)
	// Right-to-left: 10.0.0.1 is private (trusted), so 203.0.113.195 is the first untrusted IP
	if ip.String() != "203.0.113.195" {
		t.Fatalf("expected client IP '203.0.113.195', got '%s'", ip.String())
	}
}

func TestCloudflareMode(t *testing.T) {
	cfPrefix := netip.MustParsePrefix("173.245.48.0/20")
	resolver := NewResolver("cloudflare", []netip.Prefix{cfPrefix})

	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "173.245.48.10:12345"
	req.Header.Set("CF-Connecting-IP", "198.51.100.77")

	ip := resolver.ResolveClientIP(req)
	if ip.String() != "198.51.100.77" {
		t.Fatalf("expected '198.51.100.77', got '%s'", ip.String())
	}
}
