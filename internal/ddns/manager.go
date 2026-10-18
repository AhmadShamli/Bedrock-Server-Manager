package ddns

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// NotificationHandler delivers state change alerts to external channels (e.g. Discord).
type NotificationHandler interface {
	OnIPChanged(ctx context.Context, domain, provider, ipv4, ipv6 string)
	OnSyncFailed(ctx context.Context, domain, provider string, syncErr error)
	OnSyncRecovered(ctx context.Context, domain, provider, ipv4, ipv6 string)
}

// AddressSyncHandler propagates the updated hostname or IP to system settings.
type AddressSyncHandler interface {
	UpdateGameServerAddress(ctx context.Context, newAddress string) error
}

// Manager coordinates the DDNS background scheduler, change detection, and provider updates.
type Manager struct {
	mu        sync.RWMutex
	config    Config
	status    Status
	detector  *IPDetector
	providers map[string]Provider

	lastIPv4     string
	lastIPv6     string
	lastSyncTime time.Time
	hasFailed    bool
	retryDelays  []time.Duration

	notifier   NotificationHandler
	addrSyncer AddressSyncHandler

	stopCh  chan struct{}
	running bool
}

// NewManager creates a DDNS coordinator.
func NewManager(cfg Config, detector *IPDetector, notifier NotificationHandler, addrSyncer AddressSyncHandler) *Manager {
	if detector == nil {
		detector = NewIPDetector(6 * time.Second)
	}

	m := &Manager{
		config:      cfg,
		detector:    detector,
		providers:   make(map[string]Provider),
		notifier:    notifier,
		addrSyncer:  addrSyncer,
		retryDelays: []time.Duration{0, 5 * time.Second, 15 * time.Second},
		stopCh:      make(chan struct{}),
		status: Status{
			Enabled:             cfg.Enabled,
			Provider:            cfg.Provider,
			Domain:              cfg.Domain,
			Status:              StatusIdle,
			IntervalMinutes:     cfg.IntervalMinutes,
			AutoSyncGameAddress: cfg.AutoSyncGameAddress,
		},
	}

	// Register default providers
	m.RegisterProvider(NewCloudflareProvider(nil))
	m.RegisterProvider(NewDuckDNSProvider(nil))
	m.RegisterProvider(NewNoIPProvider(nil))
	m.RegisterProvider(NewWebhookProvider(nil))

	return m
}

// RegisterProvider attaches a provider adapter to the manager.
func (m *Manager) RegisterProvider(p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.Name()] = p
}

// SetRetryDelays customizes the retry backoff intervals (useful for unit tests).
func (m *Manager) SetRetryDelays(delays []time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryDelays = delays
}

// Start launches the background polling worker.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	go m.workerLoop()
}

// Stop terminates the background worker loop.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopCh)
	m.mu.Unlock()
}

// UpdateConfig updates the running configuration dynamically.
func (m *Manager) UpdateConfig(newCfg Config) {
	m.mu.Lock()
	m.config = newCfg
	m.status.Enabled = newCfg.Enabled
	m.status.Provider = newCfg.Provider
	m.status.Domain = newCfg.Domain
	m.status.IntervalMinutes = newCfg.IntervalMinutes
	m.status.AutoSyncGameAddress = newCfg.AutoSyncGameAddress
	m.mu.Unlock()
}

// GetConfig returns a snapshot of the current configuration.
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// GetStatus returns the current runtime diagnostics snapshot.
func (m *Manager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) workerLoop() {
	// Run initial synchronization immediately on boot
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	_, _ = m.Sync(ctx)
	cancel()

	for {
		m.mu.RLock()
		intervalMin := m.config.IntervalMinutes
		if intervalMin <= 0 {
			intervalMin = 5
		}
		m.mu.RUnlock()

		select {
		case <-m.stopCh:
			return
		case <-time.After(time.Duration(intervalMin) * time.Minute):
			syncCtx, syncCancel := context.WithTimeout(context.Background(), 45*time.Second)
			_, _ = m.Sync(syncCtx)
			syncCancel()
		}
	}
}

// Sync performs a full detection and update pass if IP changed or safety force-sync triggered.
func (m *Manager) Sync(ctx context.Context) (*SyncResult, error) {
	m.mu.Lock()
	cfg := m.config
	if !cfg.Enabled {
		m.status.Status = StatusIdle
		m.mu.Unlock()
		return nil, nil
	}

	m.status.Status = StatusSyncing
	m.mu.Unlock()

	// 1. Detect public IPs
	ipv4, ipv6, err := m.detector.DetectIPs(ctx, cfg.EnableIPv4, cfg.EnableIPv6)
	if err != nil {
		m.recordFailure(ctx, cfg, err)
		return nil, fmt.Errorf("ip detection failed: %w", err)
	}

	m.mu.Lock()
	m.status.CurrentIPv4 = ipv4
	m.status.CurrentIPv6 = ipv6

	ipChanged := (cfg.EnableIPv4 && ipv4 != "" && ipv4 != m.lastIPv4) ||
		(cfg.EnableIPv6 && ipv6 != "" && ipv6 != m.lastIPv6)

	timeSinceLastSync := time.Since(m.lastSyncTime)
	forceSync := m.lastSyncTime.IsZero() || timeSinceLastSync >= 24*time.Hour

	provider, exists := m.providers[cfg.Provider]
	m.mu.Unlock()

	if !exists {
		err := fmt.Errorf("unsupported DDNS provider: '%s'", cfg.Provider)
		m.recordFailure(ctx, cfg, err)
		return nil, err
	}

	// If IP hasn't changed and not force-sync, record success and return
	if !ipChanged && !forceSync {
		m.mu.Lock()
		m.status.Status = StatusSuccess
		m.status.LastError = ""
		m.mu.Unlock()
		return &SyncResult{
			Success:   true,
			IPv4:      ipv4,
			IPv6:      ipv6,
			Domain:    cfg.Domain,
			Message:   "IP unchanged, DNS record is up to date",
			Timestamp: time.Now().UTC(),
		}, nil
	}

	// 2. Execute update with progressive retry
	var syncRes *SyncResult
	var lastErr error
	m.mu.RLock()
	backoffs := m.retryDelays
	m.mu.RUnlock()
	if len(backoffs) == 0 {
		backoffs = []time.Duration{0}
	}

	for i, delay := range backoffs {
		if delay > 0 {
			select {
			case <-ctx.Done():
				m.recordFailure(ctx, cfg, ctx.Err())
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		syncRes, lastErr = provider.Update(ctx, &cfg, ipv4, ipv6)
		if lastErr == nil && syncRes != nil && syncRes.Success {
			break
		}
		log.Printf("[DDNS] Provider %s update attempt %d failed: %v", cfg.Provider, i+1, lastErr)
	}

	if lastErr != nil {
		m.recordFailure(ctx, cfg, lastErr)
		return nil, lastErr
	}

	// 3. Success! Update state and propagate
	m.recordSuccess(ctx, cfg, ipv4, ipv6, ipChanged)
	return syncRes, nil
}

// Test tests credentials and connectivity against the target provider without modifying persistent status.
func (m *Manager) Test(ctx context.Context, testCfg *Config) (*SyncResult, error) {
	if testCfg == nil {
		return nil, errors.New("configuration is required")
	}

	m.mu.RLock()
	provider, exists := m.providers[testCfg.Provider]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unsupported DDNS provider: '%s'", testCfg.Provider)
	}

	ipv4, ipv6, err := m.detector.DetectIPs(ctx, testCfg.EnableIPv4, testCfg.EnableIPv6)
	if err != nil {
		return nil, fmt.Errorf("ip detection failed: %w", err)
	}

	return provider.Test(ctx, testCfg, ipv4, ipv6)
}

func (m *Manager) recordSuccess(ctx context.Context, cfg Config, ipv4, ipv6 string, ipChanged bool) {
	m.mu.Lock()
	now := time.Now().UTC()
	m.lastIPv4 = ipv4
	m.lastIPv6 = ipv6
	m.lastSyncTime = now
	m.status.Status = StatusSuccess
	m.status.LastError = ""
	m.status.LastSyncTime = &now
	interval := cfg.IntervalMinutes
	if interval <= 0 {
		interval = 5
	}
	next := now.Add(time.Duration(interval) * time.Minute)
	m.status.NextSyncTime = &next

	wasFailed := m.hasFailed
	m.hasFailed = false
	notifier := m.notifier
	addrSyncer := m.addrSyncer
	m.mu.Unlock()

	// State-change notification: recovered from failure
	if wasFailed && notifier != nil {
		notifier.OnSyncRecovered(ctx, cfg.Domain, cfg.Provider, ipv4, ipv6)
	}

	// IP changed notification
	if ipChanged && notifier != nil {
		notifier.OnIPChanged(ctx, cfg.Domain, cfg.Provider, ipv4, ipv6)
	}

	// Propagate to game server address if enabled
	if cfg.AutoSyncGameAddress && addrSyncer != nil {
		newAddr := cfg.Domain
		if newAddr == "" {
			newAddr = ipv4
		}
		if newAddr != "" {
			_ = addrSyncer.UpdateGameServerAddress(ctx, newAddr)
		}
	}
}

func (m *Manager) recordFailure(ctx context.Context, cfg Config, err error) {
	m.mu.Lock()
	m.status.Status = StatusError
	m.status.LastError = err.Error()

	firstFailure := !m.hasFailed
	m.hasFailed = true
	notifier := m.notifier
	m.mu.Unlock()

	// State-change notification: first persistent failure only
	if firstFailure && notifier != nil {
		notifier.OnSyncFailed(ctx, cfg.Domain, cfg.Provider, err)
	}
}
