package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// ServerLookup is an interface for looking up server models by ID.
type ServerLookup interface {
	GetServer(ctx context.Context, id string) (*models.Server, error)
}

// EngineRouter dispatches container orchestration or native process calls
// to the appropriate ServerEngine implementation based on server.Engine.
type EngineRouter struct {
	dockerEngine ServerEngine
	nativeEngine ServerEngine
	lookup       ServerLookup

	mu            sync.RWMutex
	serverEngines map[string]string // serverID -> Engine constant
}

// NewEngineRouter creates a router that directs calls between Docker and Native engines.
func NewEngineRouter(docker ServerEngine, native ServerEngine, lookup ServerLookup) *EngineRouter {
	return &EngineRouter{
		dockerEngine:  docker,
		nativeEngine:  native,
		lookup:        lookup,
		serverEngines: make(map[string]string),
	}
}

// DockerEngine returns the underlying DockerEngine instance.
func (r *EngineRouter) DockerEngine() ServerEngine {
	return r.dockerEngine
}

// NativeEngine returns the underlying NativeEngine instance.
func (r *EngineRouter) NativeEngine() ServerEngine {
	return r.nativeEngine
}

// IsDockerAvailable returns true if the Docker engine is configured and ready.
func (r *EngineRouter) IsDockerAvailable() bool {
	return r.dockerEngine != nil
}

// IsNativeAvailable returns true if the native BDS engine is supported on this platform.
func (r *EngineRouter) IsNativeAvailable() bool {
	return r.nativeEngine != nil
}

func (r *EngineRouter) resolveEngine(server *models.Server) (ServerEngine, error) {
	if server == nil {
		return nil, fmt.Errorf("server cannot be nil")
	}

	engineType := server.Engine
	if engineType == "" {
		engineType = models.EngineDocker
	}

	r.mu.Lock()
	r.serverEngines[server.ID] = engineType
	r.mu.Unlock()

	if engineType == models.EngineNative {
		if r.nativeEngine == nil {
			return nil, fmt.Errorf("native engine is not available on this platform")
		}
		return r.nativeEngine, nil
	}

	if r.dockerEngine == nil {
		return nil, fmt.Errorf("docker engine is not available")
	}
	return r.dockerEngine, nil
}

func (r *EngineRouter) resolveEngineByID(serverID string) (ServerEngine, string) {
	r.mu.RLock()
	engType, ok := r.serverEngines[serverID]
	r.mu.RUnlock()

	if !ok && r.lookup != nil {
		if s, err := r.lookup.GetServer(context.Background(), serverID); err == nil && s != nil {
			engType = s.Engine
			r.mu.Lock()
			r.serverEngines[serverID] = engType
			r.mu.Unlock()
		}
	}

	if engType == models.EngineNative {
		return r.nativeEngine, engType
	}
	if engType == models.EngineDocker {
		return r.dockerEngine, engType
	}
	return nil, engType
}

func (r *EngineRouter) CreateServer(ctx context.Context, server *models.Server, dataDir string) (string, error) {
	eng, err := r.resolveEngine(server)
	if err != nil {
		return "", err
	}
	return eng.CreateServer(ctx, server, dataDir)
}

func (r *EngineRouter) StartServer(ctx context.Context, server *models.Server) error {
	eng, err := r.resolveEngine(server)
	if err != nil {
		return err
	}
	return eng.StartServer(ctx, server)
}

func (r *EngineRouter) StopServer(ctx context.Context, server *models.Server, timeoutSeconds int) error {
	eng, err := r.resolveEngine(server)
	if err != nil {
		return err
	}
	return eng.StopServer(ctx, server, timeoutSeconds)
}

func (r *EngineRouter) RestartServer(ctx context.Context, server *models.Server) error {
	eng, err := r.resolveEngine(server)
	if err != nil {
		return err
	}
	return eng.RestartServer(ctx, server)
}

func (r *EngineRouter) RemoveServer(ctx context.Context, server *models.Server, removeData bool) error {
	eng, err := r.resolveEngine(server)
	if err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.serverEngines, server.ID)
	r.mu.Unlock()
	return eng.RemoveServer(ctx, server, removeData)
}

func (r *EngineRouter) GetServerStatus(ctx context.Context, server *models.Server) (string, error) {
	eng, err := r.resolveEngine(server)
	if err != nil {
		return models.ServerStatusStopped, err
	}
	return eng.GetServerStatus(ctx, server)
}

func (r *EngineRouter) GetContainerStats(ctx context.Context, server *models.Server) (*models.MetricRaw, error) {
	eng, err := r.resolveEngine(server)
	if err != nil {
		return nil, err
	}
	return eng.GetContainerStats(ctx, server)
}

func (r *EngineRouter) SendConsoleCommand(ctx context.Context, server *models.Server, cmd string) error {
	eng, err := r.resolveEngine(server)
	if err != nil {
		return err
	}
	return eng.SendConsoleCommand(ctx, server, cmd)
}

func (r *EngineRouter) GetRecentLogs(serverID string) []string {
	eng, _ := r.resolveEngineByID(serverID)
	if eng != nil {
		return eng.GetRecentLogs(serverID)
	}
	if r.nativeEngine != nil {
		logs := r.nativeEngine.GetRecentLogs(serverID)
		if len(logs) > 0 {
			return logs
		}
	}
	if r.dockerEngine != nil {
		return r.dockerEngine.GetRecentLogs(serverID)
	}
	return []string{}
}

func (r *EngineRouter) SubscribeLogs(serverID string) (<-chan string, func()) {
	eng, _ := r.resolveEngineByID(serverID)
	if eng != nil {
		return eng.SubscribeLogs(serverID)
	}
	if r.nativeEngine != nil {
		return r.nativeEngine.SubscribeLogs(serverID)
	}
	if r.dockerEngine != nil {
		return r.dockerEngine.SubscribeLogs(serverID)
	}
	ch := make(chan string)
	close(ch)
	return ch, func() {}
}

func (r *EngineRouter) AttachLogCapture(serverID, containerID string) {
	eng, _ := r.resolveEngineByID(serverID)
	if eng != nil {
		eng.AttachLogCapture(serverID, containerID)
		return
	}
	if r.nativeEngine != nil {
		r.nativeEngine.AttachLogCapture(serverID, containerID)
	}
	if r.dockerEngine != nil {
		r.dockerEngine.AttachLogCapture(serverID, containerID)
	}
}

func (r *EngineRouter) SetLogListener(listener func(serverID, line string)) {
	if r.dockerEngine != nil {
		r.dockerEngine.SetLogListener(listener)
	}
	if r.nativeEngine != nil {
		r.nativeEngine.SetLogListener(listener)
	}
}

func (r *EngineRouter) ListNetworks(ctx context.Context) ([]string, error) {
	if r.dockerEngine != nil {
		return r.dockerEngine.ListNetworks(ctx)
	}
	return []string{}, nil
}
