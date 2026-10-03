package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscordDispatcher(t *testing.T) {
	var receivedPayload Payload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	d := NewDispatcher()
	ctx := context.Background()

	if err := d.NotifyServerStarted(ctx, server.URL, "Test Server", "srv-1", 19132); err != nil {
		t.Fatalf("NotifyServerStarted failed: %v", err)
	}

	if err := d.NotifyPlayerJoined(ctx, server.URL, "Test Server", "Steve", 1); err != nil {
		t.Fatalf("NotifyPlayerJoined failed: %v", err)
	}

	if err := d.NotifyResourceAlert(ctx, server.URL, "Test Server", "srv-1", 1932735283, 2147483648, 90.0, 185.0, 2.0, 92.5); err != nil {
		t.Fatalf("NotifyResourceAlert failed: %v", err)
	}

	_ = receivedPayload
}
