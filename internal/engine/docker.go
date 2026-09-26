package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const DefaultImage = "itzg/minecraft-bedrock-server:latest"

// DockerEngine orchestrates Minecraft Bedrock instances via Docker Engine API.
type DockerEngine struct {
	cli            *dockerclient.Client
	circuitBreaker *CrashCircuitBreaker

	mu          sync.RWMutex
	ringBuffers map[string]*RingBuffer
	logChans    map[string][]chan string
	logCancels  map[string]context.CancelFunc
	logListener func(serverID, line string)
}

// NewDockerEngine initializes a DockerEngine connected to the local socket or DOCKER_HOST.
func NewDockerEngine(dockerHost string) (*DockerEngine, error) {
	opts := []dockerclient.Opt{
		dockerclient.WithAPIVersionNegotiation(),
	}
	if dockerHost != "" {
		opts = append(opts, dockerclient.WithHost(dockerHost))
	} else {
		opts = append(opts, dockerclient.FromEnv)
	}

	cli, err := dockerclient.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &DockerEngine{
		cli:            cli,
		circuitBreaker: NewCrashCircuitBreaker(5, 5*time.Minute),
		ringBuffers:    make(map[string]*RingBuffer),
		logChans:       make(map[string][]chan string),
		logCancels:     make(map[string]context.CancelFunc),
	}, nil
}

// EnsureImage pulls the bedrock server image if not locally present.
func (e *DockerEngine) EnsureImage(ctx context.Context, imageName string) error {
	if imageName == "" {
		imageName = DefaultImage
	}
	reader, err := e.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// CreateServer provisions a new Docker container with strict CPU, RAM, and volume limits.
func (e *DockerEngine) CreateServer(ctx context.Context, server *models.Server, dataDir string) (string, error) {
	memBytes, err := ParseMemoryBytes(server.MemoryLimit)
	if err != nil {
		return "", err
	}

	nanoCPUs := int64(server.CPULimit * 1e9)
	if nanoCPUs <= 0 {
		nanoCPUs = 2 * 1e9 // Default 2 CPUs
	}

	// Prepare host server data directory
	hostServerDir, err := filepath.Abs(filepath.Join(dataDir, "servers", server.ID))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(hostServerDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create server directory: %w", err)
	}

	imageTag := server.Version
	if imageTag == "" || imageTag == "latest" {
		imageTag = DefaultImage
	} else {
		imageTag = "itzg/minecraft-bedrock-server:" + imageTag
	}

	containerName := fmt.Sprintf("bsm-%s", server.ID)

	envVars := []string{
		"EULA=TRUE",
		fmt.Sprintf("SERVER_NAME=%s", server.Name),
		fmt.Sprintf("GAMEMODE=%s", server.Mode),
		fmt.Sprintf("DIFFICULTY=%s", server.Difficulty),
		"SERVER_PORT=19132",
		"SERVER_PORT_V6=19133",
	}
	if server.Seed != "" {
		envVars = append(envVars, fmt.Sprintf("LEVEL_SEED=%s", server.Seed))
	}

	config := &container.Config{
		Image:     imageTag,
		Env:       envVars,
		Tty:       true,
		OpenStdin: true,
		StdinOnce: false,
		ExposedPorts: nat.PortSet{
			"19132/udp": struct{}{},
			"19133/udp": struct{}{},
		},
	}

	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/data", hostServerDir),
		},
		PortBindings: nat.PortMap{
			"19132/udp": []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(server.Port)},
			},
			"19133/udp": []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(server.PortV6)},
			},
		},
		Resources: container.Resources{
			Memory:     memBytes,
			MemorySwap: memBytes, // Disable swap beyond assigned memory
			NanoCPUs:   nanoCPUs,
		},
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	resp, err := e.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container '%s': %w", containerName, err)
	}

	return resp.ID, nil
}

// StartServer launches the container and initiates console log capture.
func (e *DockerEngine) StartServer(ctx context.Context, server *models.Server) error {
	if e.circuitBreaker.IsTripped(server.ID) {
		return fmt.Errorf("circuit breaker tripped for server %s (too many recent crashes)", server.ID)
	}

	containerID := server.ContainerID
	if containerID == "" {
		containerID = fmt.Sprintf("bsm-%s", server.ID)
	}

	if err := e.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Reset crash circuit breaker on manual start
	e.circuitBreaker.Reset(server.ID)

	// Attach log reader loop
	go e.captureContainerLogs(server.ID, containerID)

	return nil
}

// StopServer halts the container gracefully with fallback timeout.
func (e *DockerEngine) StopServer(ctx context.Context, server *models.Server, timeoutSeconds int) error {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 15
	}
	containerID := server.ContainerID
	if containerID == "" {
		containerID = fmt.Sprintf("bsm-%s", server.ID)
	}

	stopTimeout := timeoutSeconds
	return e.cli.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &stopTimeout,
	})
}

// RestartServer restarts the container.
func (e *DockerEngine) RestartServer(ctx context.Context, server *models.Server) error {
	containerID := server.ContainerID
	if containerID == "" {
		containerID = fmt.Sprintf("bsm-%s", server.ID)
	}
	stopTimeout := 15
	if err := e.cli.ContainerRestart(ctx, containerID, container.StopOptions{
		Timeout: &stopTimeout,
	}); err != nil {
		return err
	}
	go e.captureContainerLogs(server.ID, containerID)
	return nil
}

// RemoveServer stops and removes the container.
func (e *DockerEngine) RemoveServer(ctx context.Context, server *models.Server, removeData bool) error {
	containerID := server.ContainerID
	if containerID == "" {
		containerID = fmt.Sprintf("bsm-%s", server.ID)
	}

	_ = e.cli.ContainerStop(ctx, containerID, container.StopOptions{})
	return e.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
}

// GetServerStatus returns current Docker container status (running, stopped, crashed).
func (e *DockerEngine) GetServerStatus(ctx context.Context, server *models.Server) (string, error) {
	if e.circuitBreaker.IsTripped(server.ID) {
		return models.ServerStatusCrashed, nil
	}

	containerID := server.ContainerID
	if containerID == "" {
		containerID = fmt.Sprintf("bsm-%s", server.ID)
	}

	inspect, err := e.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if dockerclient.IsErrNotFound(err) {
			return models.ServerStatusStopped, nil
		}
		return "", err
	}

	if inspect.State.Running {
		return models.ServerStatusRunning, nil
	}
	if inspect.State.Restarting {
		return models.ServerStatusStarting, nil
	}
	if inspect.State.ExitCode != 0 {
		if e.circuitBreaker.RecordCrash(server.ID, time.Now()) {
			return models.ServerStatusCrashed, nil
		}
	}

	return models.ServerStatusStopped, nil
}

// GetContainerStats samples real-time CPU % and RAM usage from Docker.
func (e *DockerEngine) GetContainerStats(ctx context.Context, server *models.Server) (*models.MetricRaw, error) {
	containerID := server.ContainerID
	if containerID == "" {
		containerID = fmt.Sprintf("bsm-%s", server.ID)
	}

	statsResp, err := e.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, err
	}
	defer statsResp.Body.Close()

	var stats types.StatsJSON
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	// Calculate CPU percentage
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
		if onlineCPUs == 0.0 {
			onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
		}
		if onlineCPUs == 0.0 {
			onlineCPUs = 1.0
		}
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	ramBytes := int64(stats.MemoryStats.Usage)

	return &models.MetricRaw{
		ServerID:   server.ID,
		Timestamp:  time.Now().UTC(),
		CPUPercent: cpuPercent,
		RAMBytes:   ramBytes,
	}, nil
}

// SendConsoleCommand pipes a command into the Bedrock server stdin.
func (e *DockerEngine) SendConsoleCommand(ctx context.Context, server *models.Server, cmd string) error {
	containerID := server.ContainerID
	if containerID == "" {
		containerID = fmt.Sprintf("bsm-%s", server.ID)
	}

	escapedCmd := strings.ReplaceAll(cmd, "'", `'\''`)
	execConfig := types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: false,
		AttachStderr: false,
		Tty:          true,
		Cmd:          []string{"sh", "-c", fmt.Sprintf("echo '%s' > /proc/1/fd/0", escapedCmd)},
	}

	execID, err := e.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return err
	}

	return e.cli.ContainerExecStart(ctx, execID.ID, types.ExecStartCheck{
		Tty: true,
	})
}

// GetRecentLogs retrieves lines from the server ring buffer.
func (e *DockerEngine) GetRecentLogs(serverID string) []string {
	e.mu.RLock()
	rb, exists := e.ringBuffers[serverID]
	e.mu.RUnlock()

	if !exists {
		return []string{}
	}
	return rb.GetAll()
}

// SubscribeLogs registers a subscriber channel for real-time log events.
func (e *DockerEngine) SubscribeLogs(serverID string) (<-chan string, func()) {
	ch := make(chan string, 100)

	e.mu.Lock()
	e.logChans[serverID] = append(e.logChans[serverID], ch)
	e.mu.Unlock()

	unsubscribe := func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		subscribers := e.logChans[serverID]
		for i, subscriber := range subscribers {
			if subscriber == ch {
				e.logChans[serverID] = append(subscribers[:i], subscribers[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, unsubscribe
}

func (e *DockerEngine) AttachLogCapture(serverID, containerID string) {
	if containerID == "" {
		containerID = fmt.Sprintf("bsm-%s", serverID)
	}
	go e.captureContainerLogs(serverID, containerID)
}

func (e *DockerEngine) SetLogListener(listener func(serverID, line string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logListener = listener
}

func (e *DockerEngine) captureContainerLogs(serverID, containerID string) {
	e.mu.Lock()
	if oldCancel, exists := e.logCancels[serverID]; exists && oldCancel != nil {
		oldCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.logCancels[serverID] = cancel

	rb, exists := e.ringBuffers[serverID]
	if !exists {
		rb = NewRingBuffer(1000)
		e.ringBuffers[serverID] = rb
	}
	e.mu.Unlock()

	defer func() {
		cancel()
		e.mu.Lock()
		delete(e.logCancels, serverID)
		e.mu.Unlock()
	}()

	reader, err := e.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "200",
	})
	if err != nil {
		log.Printf("[DockerEngine] Failed to stream logs for %s: %v", serverID, err)
		return
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		rb.Write(line)

		// Broadcast to subscribers
		e.mu.RLock()
		subscribers := e.logChans[serverID]
		for _, ch := range subscribers {
			select {
			case ch <- line:
			default:
			}
		}
		listener := e.logListener
		e.mu.RUnlock()

		if listener != nil {
			listener(serverID, line)
		}
	}
}
