package ddns

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type mockNotifier struct {
	ipChangedCount int
	failedCount    int
	recoveredCount int
}

func (n *mockNotifier) OnIPChanged(ctx context.Context, domain, provider, ipv4, ipv6 string) {
	n.ipChangedCount++
}

func (n *mockNotifier) OnSyncFailed(ctx context.Context, domain, provider string, syncErr error) {
	n.failedCount++
}

func (n *mockNotifier) OnSyncRecovered(ctx context.Context, domain, provider, ipv4, ipv6 string) {
	n.recoveredCount++
}

type mockAddrSyncer struct {
	lastAddress string
}

func (s *mockAddrSyncer) UpdateGameServerAddress(ctx context.Context, newAddress string) error {
	s.lastAddress = newAddress
	return nil
}

type mockProvider struct {
	name        string
	updateCount int
	shouldFail  bool
}

func (p *mockProvider) Name() string { return p.name }
func (p *mockProvider) Update(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error) {
	p.updateCount++
	if p.shouldFail {
		return nil, errors.New("simulated provider error")
	}
	return &SyncResult{
		Success:   true,
		Domain:    cfg.Domain,
		IPv4:      ipv4,
		IPv6:      ipv6,
		Timestamp: time.Now(),
	}, nil
}
func (p *mockProvider) Test(ctx context.Context, cfg *Config, ipv4, ipv6 string) (*SyncResult, error) {
	return p.Update(ctx, cfg, ipv4, ipv6)
}

func TestManager_ChangeDetectionAndPropagation(t *testing.T) {
	currentIP := "198.51.100.1"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(currentIP))
	}))
	defer ts.Close()

	detector := &IPDetector{
		httpClient: ts.Client(),
		v4URLs:     []string{ts.URL},
	}

	notifier := &mockNotifier{}
	syncer := &mockAddrSyncer{}
	prov := &mockProvider{name: "test-prov"}

	cfg := Config{
		Enabled:             true,
		Provider:            "test-prov",
		Domain:              "mc.example.com",
		EnableIPv4:          true,
		IntervalMinutes:     5,
		AutoSyncGameAddress: true,
	}

	mgr := NewManager(cfg, detector, notifier, syncer)
	mgr.RegisterProvider(prov)

	// First sync should update provider and syncer
	res, err := mgr.Sync(context.Background())
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	if prov.updateCount != 1 {
		t.Fatalf("expected 1 update, got %d", prov.updateCount)
	}
	if syncer.lastAddress != "mc.example.com" {
		t.Fatalf("expected address mc.example.com, got %s", syncer.lastAddress)
	}
	if notifier.ipChangedCount != 1 {
		t.Fatalf("expected 1 ipChanged notification, got %d", notifier.ipChangedCount)
	}

	// Second sync with SAME IP: should skip provider update
	res2, err := mgr.Sync(context.Background())
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
	if prov.updateCount != 1 {
		t.Fatalf("expected update count to remain 1, got %d", prov.updateCount)
	}
	if res2.Message != "IP unchanged, DNS record is up to date" {
		t.Fatalf("expected unchanged message, got %s", res2.Message)
	}

	// Change IP and re-sync: should update provider again
	currentIP = "198.51.100.2"
	res3, err := mgr.Sync(context.Background())
	if err != nil {
		t.Fatalf("third sync failed: %v", err)
	}
	if !res3.Success {
		t.Fatalf("expected success on third sync")
	}
	if prov.updateCount != 2 {
		t.Fatalf("expected update count 2, got %d", prov.updateCount)
	}
	if notifier.ipChangedCount != 2 {
		t.Fatalf("expected 2 ipChanged notifications, got %d", notifier.ipChangedCount)
	}
}

func TestManager_StateChangeAlerts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("203.0.113.50"))
	}))
	defer ts.Close()

	detector := &IPDetector{
		httpClient: ts.Client(),
		v4URLs:     []string{ts.URL},
	}

	notifier := &mockNotifier{}
	prov := &mockProvider{name: "failing-prov", shouldFail: true}

	cfg := Config{
		Enabled:         true,
		Provider:        "failing-prov",
		Domain:          "alert.example.com",
		EnableIPv4:      true,
		IntervalMinutes: 5,
	}

	mgr := NewManager(cfg, detector, notifier, nil)
	mgr.RegisterProvider(prov)
	mgr.SetRetryDelays([]time.Duration{0})

	// First failure
	_, err := mgr.Sync(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if notifier.failedCount != 1 {
		t.Fatalf("expected 1 failure alert, got %d", notifier.failedCount)
	}

	// Second failure: should NOT send duplicate alert
	_, err = mgr.Sync(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if notifier.failedCount != 1 {
		t.Fatalf("expected failedCount to stay 1, got %d", notifier.failedCount)
	}

	// Now provider recovers!
	prov.shouldFail = false
	res, err := mgr.Sync(context.Background())
	if err != nil {
		t.Fatalf("expected success on recovery, got: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	if notifier.recoveredCount != 1 {
		t.Fatalf("expected 1 recovery alert, got %d", notifier.recoveredCount)
	}
}
