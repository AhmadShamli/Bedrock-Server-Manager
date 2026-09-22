package firewall

import (
	"context"
	"fmt"
	"sync"
)

// MockFirewallDriver provides an in-memory firewall driver for non-root testing and simulation.
type MockFirewallDriver struct {
	mu          sync.RWMutex
	rules       map[string]ActiveRule
	allowCalls  int
	revokeCalls int
	flushCalls  int
	failNext    error
}

// NewMockFirewallDriver initializes a new in-memory mock firewall driver.
func NewMockFirewallDriver() *MockFirewallDriver {
	return &MockFirewallDriver{
		rules: make(map[string]ActiveRule),
	}
}

func (m *MockFirewallDriver) Name() string {
	return "mock"
}

func (m *MockFirewallDriver) Detect(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockFirewallDriver) Validate(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.failNext
}

func (m *MockFirewallDriver) SetFailNext(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failNext = err
}

func ruleKey(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
}

func (m *MockFirewallDriver) AllowPort(ctx context.Context, ip string, port int, comment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failNext != nil {
		err := m.failNext
		m.failNext = nil
		return err
	}

	m.allowCalls++
	k := ruleKey(ip, port)
	m.rules[k] = ActiveRule{
		Backend: "mock",
		IP:      ip,
		Port:    port,
		Comment: comment,
	}
	return nil
}

func (m *MockFirewallDriver) RevokePort(ctx context.Context, ip string, port int, comment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failNext != nil {
		err := m.failNext
		m.failNext = nil
		return err
	}

	m.revokeCalls++
	k := ruleKey(ip, port)
	delete(m.rules, k)
	return nil
}

func (m *MockFirewallDriver) FlushRules(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failNext != nil {
		err := m.failNext
		m.failNext = nil
		return err
	}

	m.flushCalls++
	m.rules = make(map[string]ActiveRule)
	return nil
}

func (m *MockFirewallDriver) ListActiveRules(ctx context.Context) ([]ActiveRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []ActiveRule
	for _, r := range m.rules {
		result = append(result, r)
	}
	return result, nil
}

func (m *MockFirewallDriver) HasRule(ip string, port int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := ruleKey(ip, port)
	_, exists := m.rules[k]
	return exists
}

func (m *MockFirewallDriver) TotalRules() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rules)
}
