package ddns

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIPDetector_DetectIPv4_Trace(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fl=123\nip=203.0.113.195\nts=123456\n"))
	}))
	defer ts.Close()

	d := &IPDetector{
		httpClient: ts.Client(),
		v4URLs:     []string{ts.URL + "/cdn-cgi/trace"},
	}

	addr, err := d.DetectIPv4(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr.String() != "203.0.113.195" {
		t.Fatalf("expected 203.0.113.195, got %s", addr.String())
	}
}

func TestIPDetector_DetectIPv4_PlainText_Fallback(t *testing.T) {
	// First server fails
	tsFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsFail.Close()

	// Second server succeeds with plain text
	tsSuccess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("198.51.100.42\n"))
	}))
	defer tsSuccess.Close()

	d := &IPDetector{
		httpClient: &http.Client{Timeout: 2 * time.Second},
		v4URLs:     []string{tsFail.URL, tsSuccess.URL},
	}

	addr, err := d.DetectIPv4(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr.String() != "198.51.100.42" {
		t.Fatalf("expected 198.51.100.42, got %s", addr.String())
	}
}

func TestIPDetector_DetectIPv6(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("2001:db8::8a2e:370:7334\n"))
	}))
	defer ts.Close()

	d := &IPDetector{
		httpClient: ts.Client(),
		v6URLs:     []string{ts.URL},
	}

	addr, err := d.DetectIPv6(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr.String() != "2001:db8::8a2e:370:7334" {
		t.Fatalf("expected 2001:db8::8a2e:370:7334, got %s", addr.String())
	}
}
