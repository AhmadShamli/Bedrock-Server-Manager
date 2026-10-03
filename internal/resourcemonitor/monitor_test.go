package resourcemonitor

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// MockCommandSender mocks the Engine command execution.
type MockCommandSender struct {
	mu       sync.Mutex
	commands []string
	failNext bool
}

func (m *MockCommandSender) SendConsoleCommand(ctx context.Context, server *models.Server, cmd string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return errors.New("stdin pipe error")
	}
	m.commands = append(m.commands, cmd)
	return nil
}

func (m *MockCommandSender) GetCommands() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.commands))
	copy(cp, m.commands)
	return cp
}

// MockChatBroadcaster mocks PlayerManager chat feed.
type MockChatBroadcaster struct {
	mu       sync.Mutex
	messages []struct {
		ServerID string
		Gamertag string
		Message  string
	}
}

func (m *MockChatBroadcaster) AddChatMessage(serverID, gamertag, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, struct {
		ServerID string
		Gamertag string
		Message  string
	}{ServerID: serverID, Gamertag: gamertag, Message: message})
}

// MockDBStore mocks database settings and audit logging.
type MockDBStore struct {
	mu        sync.Mutex
	settings  map[string]string
	auditLogs []models.AuditLog
}

func (m *MockDBStore) GetSetting(ctx context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, ok := m.settings[key]
	if !ok {
		return "", errors.New("not found")
	}
	return val, nil
}

func (m *MockDBStore) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if log != nil {
		m.auditLogs = append(m.auditLogs, *log)
	}
	return nil
}

func (m *MockDBStore) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	return &models.User{ID: id, Username: "admin"}, nil
}

// MockWebhookSender mocks Discord webhook dispatch.
type MockWebhookSender struct {
	mu     sync.Mutex
	alerts []struct {
		WebhookURL string
		ServerName string
		ServerID   string
		RAMUsed    int64
		RAMLimit   int64
		RAMPct     float64
		CPUUsed    float64
		CPULimit   float64
		CPUPct     float64
	}
}

func (m *MockWebhookSender) NotifyResourceAlert(
	ctx context.Context,
	webhookURL, serverName, serverID string,
	ramUsed, ramLimit int64, ramPct float64,
	cpuUsed, cpuLimitCores, cpuPct float64,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, struct {
		WebhookURL string
		ServerName string
		ServerID   string
		RAMUsed    int64
		RAMLimit   int64
		RAMPct     float64
		CPUUsed    float64
		CPULimit   float64
		CPUPct     float64
	}{
		WebhookURL: webhookURL,
		ServerName: serverName,
		ServerID:   serverID,
		RAMUsed:    ramUsed,
		RAMLimit:   ramLimit,
		RAMPct:     ramPct,
		CPUUsed:    cpuUsed,
		CPULimit:   cpuLimitCores,
		CPUPct:     cpuPct,
	})
	return nil
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1024 * 1024 * 512, "512.00 MB"},
		{1024 * 1024 * 1024 * 2, "2.00 GB"},
	}

	for _, tt := range tests {
		got := FormatBytes(tt.bytes)
		if got != tt.expected {
			t.Errorf("FormatBytes(%d) = %s; want %s", tt.bytes, got, tt.expected)
		}
	}
}

func TestResourceMonitor_NormalUsage(t *testing.T) {
	mockEng := &MockCommandSender{}
	mockChat := &MockChatBroadcaster{}
	mockDB := &MockDBStore{settings: make(map[string]string)}
	mockWH := &MockWebhookSender{}

	mon := NewResourceMonitor(mockEng, mockChat, mockDB, mockWH)

	server := &models.Server{
		ID:          "srv-test-1",
		Name:        "Survival World",
		Status:      models.ServerStatusRunning,
		MemoryLimit: "2G",
		CPULimit:    2.0,
	}

	// 50% RAM (1GB of 2GB), 50% CPU (100% of 200%) -> should NOT trigger alert
	status, alerted, err := mon.CheckServer(context.Background(), server, 100.0, 1024*1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alerted {
		t.Errorf("expected no alert, but got alerted=true")
	}
	if status.IsNearlyFull {
		t.Errorf("expected IsNearlyFull=false, got true")
	}
	if len(mockEng.GetCommands()) > 0 {
		t.Errorf("expected no commands sent, got %d", len(mockEng.GetCommands()))
	}
}

func TestResourceMonitor_RAMNearlyFull(t *testing.T) {
	mockEng := &MockCommandSender{}
	mockChat := &MockChatBroadcaster{}
	mockDB := &MockDBStore{
		settings: map[string]string{
			"discord_webhook_url": "https://discord.com/api/webhooks/123/xyz",
		},
	}
	mockWH := &MockWebhookSender{}

	mon := NewResourceMonitor(mockEng, mockChat, mockDB, mockWH)

	server := &models.Server{
		ID:          "srv-test-ram",
		Name:        "Faction Server",
		Status:      models.ServerStatusRunning,
		MemoryLimit: "2G", // 2147483648 bytes
		CPULimit:    2.0,  // 200% capacity
	}

	// 90% RAM (1932735283 bytes), 20% CPU (40.0%) -> triggers alert (RAM >= 85%)
	status, alerted, err := mon.CheckServer(context.Background(), server, 40.0, 1932735283)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !alerted {
		t.Fatalf("expected alerted=true, got false")
	}
	if !status.IsNearlyFull {
		t.Errorf("expected IsNearlyFull=true")
	}
	if status.TriggeredResource != "ram" {
		t.Errorf("expected TriggeredResource='ram', got '%s'", status.TriggeredResource)
	}

	// Verify command sent via tellraw @a
	cmds := mockEng.GetCommands()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 console command, got %d", len(cmds))
	}
	if !strings.HasPrefix(cmds[0], "tellraw @a ") {
		t.Errorf("expected command starting with 'tellraw @a ', got: %s", cmds[0])
	}
	if !strings.Contains(cmds[0], "RAM") || !strings.Contains(cmds[0], "CPU") {
		t.Errorf("expected tellraw command to mention both RAM and CPU, got: %s", cmds[0])
	}

	// Verify chat message
	if len(mockChat.messages) != 1 {
		t.Fatalf("expected 1 chat message, got %d", len(mockChat.messages))
	}
	chatMsg := mockChat.messages[0].Message
	if !strings.Contains(chatMsg, "RAM: 1.80 GB / 2.00 GB") {
		t.Errorf("expected chat message to contain 'RAM: 1.80 GB / 2.00 GB', got: %s", chatMsg)
	}
	if !strings.Contains(chatMsg, "CPU:") {
		t.Errorf("expected chat message to contain CPU usage vs allocated, got: %s", chatMsg)
	}

	// Verify webhook alert
	if len(mockWH.alerts) != 1 {
		t.Fatalf("expected 1 webhook alert, got %d", len(mockWH.alerts))
	}
	if mockWH.alerts[0].ServerID != "srv-test-ram" {
		t.Errorf("expected webhook ServerID 'srv-test-ram', got %s", mockWH.alerts[0].ServerID)
	}

	// Verify audit log
	if len(mockDB.auditLogs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(mockDB.auditLogs))
	}
	if mockDB.auditLogs[0].Action != "resource_alert" {
		t.Errorf("expected audit action 'resource_alert', got %s", mockDB.auditLogs[0].Action)
	}
}

func TestResourceMonitor_CPUNearlyFull(t *testing.T) {
	mockEng := &MockCommandSender{}
	mockChat := &MockChatBroadcaster{}
	mockDB := &MockDBStore{settings: make(map[string]string)}
	mockWH := &MockWebhookSender{}

	mon := NewResourceMonitor(mockEng, mockChat, mockDB, mockWH)

	server := &models.Server{
		ID:          "srv-test-cpu",
		Name:        "Creative Server",
		Status:      models.ServerStatusRunning,
		MemoryLimit: "2G",
		CPULimit:    2.0, // 200% capacity
	}

	// 180.0% CPU out of 200.0% (90% of allocated CPU cores) -> triggers alert
	status, alerted, err := mon.CheckServer(context.Background(), server, 180.0, 500*1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !alerted {
		t.Fatalf("expected alerted=true, got false")
	}
	if status.TriggeredResource != "cpu" {
		t.Errorf("expected TriggeredResource='cpu', got '%s'", status.TriggeredResource)
	}
	if status.CPUPercent < 89.9 || status.CPUPercent > 90.1 {
		t.Errorf("expected CPUPercent ~90%%, got %f", status.CPUPercent)
	}
}

func TestResourceMonitor_Cooldown1Minute(t *testing.T) {
	mockEng := &MockCommandSender{}
	mockChat := &MockChatBroadcaster{}
	mockDB := &MockDBStore{settings: make(map[string]string)}
	mockWH := &MockWebhookSender{}

	// Cooldown 1 minute
	mon := NewResourceMonitor(mockEng, mockChat, mockDB, mockWH, Config{
		ThresholdPercent: 85.0,
		AlertInterval:    1 * time.Minute,
		Enabled:          true,
	})

	server := &models.Server{
		ID:          "srv-test-cooldown",
		Name:        "Survival Cooldown",
		Status:      models.ServerStatusRunning,
		MemoryLimit: "2G",
		CPULimit:    2.0,
	}

	var twoGB float64 = 2 * 1024 * 1024 * 1024

	// 1. First sample at 95% RAM -> must trigger alert
	_, alerted, err := mon.CheckServer(context.Background(), server, 50.0, int64(twoGB*0.95))
	if err != nil || !alerted {
		t.Fatalf("expected first check to alert, alerted=%v, err=%v", alerted, err)
	}
	if len(mockEng.GetCommands()) != 1 {
		t.Fatalf("expected 1 command, got %d", len(mockEng.GetCommands()))
	}

	// 2. Second sample 2 seconds later (still 95% RAM) -> MUST NOT trigger alert (cooldown active)
	_, alerted2, err2 := mon.CheckServer(context.Background(), server, 50.0, int64(twoGB*0.95))
	if err2 != nil || alerted2 {
		t.Fatalf("expected second check NOT to alert within 1m, alerted=%v, err=%v", alerted2, err2)
	}
	if len(mockEng.GetCommands()) != 1 {
		t.Fatalf("expected still only 1 command, got %d", len(mockEng.GetCommands()))
	}

	// 3. Fast-forward last alert by 61 seconds (simulating 1 minute elapsed)
	mon.mu.Lock()
	mon.lastAlert[server.ID] = time.Now().Add(-61 * time.Second)
	mon.mu.Unlock()

	// 4. Third sample after 1 minute elapsed -> MUST trigger alert again!
	_, alerted3, err3 := mon.CheckServer(context.Background(), server, 50.0, int64(twoGB*0.95))
	if err3 != nil || !alerted3 {
		t.Fatalf("expected third check to alert after 1m cooldown, alerted=%v, err=%v", alerted3, err3)
	}
	if len(mockEng.GetCommands()) != 2 {
		t.Fatalf("expected 2 commands total, got %d", len(mockEng.GetCommands()))
	}
}

func TestResourceMonitor_TellrawFallbackToSay(t *testing.T) {
	mockEng := &MockCommandSender{failNext: true} // Will fail the first command (tellraw)
	mockChat := &MockChatBroadcaster{}
	mockDB := &MockDBStore{settings: make(map[string]string)}
	mockWH := &MockWebhookSender{}

	mon := NewResourceMonitor(mockEng, mockChat, mockDB, mockWH)

	server := &models.Server{
		ID:          "srv-fallback",
		Name:        "Fallback Server",
		Status:      models.ServerStatusRunning,
		MemoryLimit: "2G",
		CPULimit:    2.0,
	}

	var twoGB float64 = 2 * 1024 * 1024 * 1024

	// High RAM (90%)
	_, alerted, _ := mon.CheckServer(context.Background(), server, 50.0, int64(twoGB*0.90))
	if !alerted {
		t.Fatalf("expected alerted=true")
	}

	// The first attempt (tellraw) failed, so fallback 'say' should have been issued
	cmds := mockEng.GetCommands()
	if len(cmds) != 1 {
		t.Fatalf("expected fallback command to be recorded, got %d", len(cmds))
	}
	if !strings.HasPrefix(cmds[0], "say ") {
		t.Errorf("expected fallback command starting with 'say ', got: %s", cmds[0])
	}
}

func TestResourceMonitor_DynamicConfigFromDB(t *testing.T) {
	mockEng := &MockCommandSender{}
	mockChat := &MockChatBroadcaster{}
	mockDB := &MockDBStore{
		settings: map[string]string{
			"resource_alert_threshold":        "90", // custom 90% threshold
			"resource_alert_interval_seconds": "30", // custom 30s interval
		},
	}
	mockWH := &MockWebhookSender{}

	mon := NewResourceMonitor(mockEng, mockChat, mockDB, mockWH)

	server := &models.Server{
		ID:          "srv-dyn",
		Name:        "Dynamic Server",
		Status:      models.ServerStatusRunning,
		MemoryLimit: "2G",
		CPULimit:    2.0,
	}

	var twoGB float64 = 2 * 1024 * 1024 * 1024

	// 86% RAM: exceeds default 85%, but is BELOW custom 90% threshold -> should NOT alert
	status, alerted, _ := mon.CheckServer(context.Background(), server, 50.0, int64(twoGB*0.86))
	if alerted {
		t.Errorf("expected no alert under custom 90%% threshold, got alerted=true")
	}
	if status.IsNearlyFull {
		t.Errorf("expected IsNearlyFull=false for 86%% with 90%% threshold")
	}

	// 92% RAM: exceeds custom 90% threshold -> MUST alert
	status2, alerted2, _ := mon.CheckServer(context.Background(), server, 50.0, int64(twoGB*0.92))
	if !alerted2 {
		t.Errorf("expected alert for 92%% with 90%% threshold, got alerted=false")
	}
	if !status2.IsNearlyFull {
		t.Errorf("expected IsNearlyFull=true for 92%%")
	}
}
