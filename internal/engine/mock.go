package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// MockEngine is an in-memory test implementation of ServerEngine.
type MockEngine struct {
	mu             sync.RWMutex
	servers        map[string]*models.Server
	statuses       map[string]string
	ringBuffers    map[string]*RingBuffer
	logChans       map[string][]chan string
	circuitBreaker *CrashCircuitBreaker
}

// NewMockEngine initializes a MockEngine.
func NewMockEngine() *MockEngine {
	return &MockEngine{
		servers:        make(map[string]*models.Server),
		statuses:       make(map[string]string),
		ringBuffers:    make(map[string]*RingBuffer),
		logChans:       make(map[string][]chan string),
		circuitBreaker: NewCrashCircuitBreaker(5, 5*time.Minute),
	}
}

func (m *MockEngine) CreateServer(ctx context.Context, server *models.Server, dataDir string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	containerID := fmt.Sprintf("mock-container-%s", server.ID)
	m.servers[server.ID] = server
	m.statuses[server.ID] = models.ServerStatusStopped

	rb := NewRingBuffer(1000)
	rb.Write(fmt.Sprintf("[MOCK] Server %s container initialized", server.Name))
	m.ringBuffers[server.ID] = rb

	return containerID, nil
}

func (m *MockEngine) StartServer(ctx context.Context, server *models.Server) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.circuitBreaker.IsTripped(server.ID) {
		return fmt.Errorf("circuit breaker tripped for server %s", server.ID)
	}

	m.statuses[server.ID] = models.ServerStatusRunning
	if rb, ok := m.ringBuffers[server.ID]; ok {
		rb.Write("[MOCK] Dedicated Bedrock Server started on UDP port " + fmt.Sprint(server.Port))
		rb.Write("[MOCK] Server opened on port " + fmt.Sprint(server.Port))
	}
	return nil
}

func (m *MockEngine) StopServer(ctx context.Context, server *models.Server, timeoutSeconds int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.statuses[server.ID] = models.ServerStatusStopped
	if rb, ok := m.ringBuffers[server.ID]; ok {
		rb.Write("[MOCK] Dedicated Bedrock Server stopped.")
	}
	return nil
}

func (m *MockEngine) RestartServer(ctx context.Context, server *models.Server) error {
	_ = m.StopServer(ctx, server, 5)
	return m.StartServer(ctx, server)
}

func (m *MockEngine) RemoveServer(ctx context.Context, server *models.Server, removeData bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.servers, server.ID)
	delete(m.statuses, server.ID)
	delete(m.ringBuffers, server.ID)
	return nil
}

func (m *MockEngine) GetServerStatus(ctx context.Context, server *models.Server) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if status, ok := m.statuses[server.ID]; ok {
		return status, nil
	}
	return models.ServerStatusStopped, nil
}

func (m *MockEngine) GetContainerStats(ctx context.Context, server *models.Server) (*models.MetricRaw, error) {
	return &models.MetricRaw{
		ServerID:    server.ID,
		Timestamp:   time.Now().UTC(),
		CPUPercent:  14.5,
		RAMBytes:    1024 * 1024 * 450, // ~450MB
		PlayerCount: 2,
	}, nil
}

func (m *MockEngine) SendConsoleCommand(ctx context.Context, server *models.Server, cmd string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rb, ok := m.ringBuffers[server.ID]; ok {
		line := fmt.Sprintf("[MOCK CMD] > %s", cmd)
		rb.Write(line)
		for _, ch := range m.logChans[server.ID] {
			select {
			case ch <- line:
			default:
			}
		}
	}
	return nil
}

func (m *MockEngine) GetRecentLogs(serverID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if rb, ok := m.ringBuffers[serverID]; ok {
		return rb.GetAll()
	}
	return []string{}
}

func (m *MockEngine) SubscribeLogs(serverID string) (<-chan string, func()) {
	ch := make(chan string, 50)

	m.mu.Lock()
	m.logChans[serverID] = append(m.logChans[serverID], ch)
	m.mu.Unlock()

	unsubscribe := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		subs := m.logChans[serverID]
		for i, sub := range subs {
			if sub == ch {
				m.logChans[serverID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, unsubscribe
}
