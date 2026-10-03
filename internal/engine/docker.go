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

// containerCPUSample holds previous sample ticks for delta calculation.
type containerCPUSample struct {
	containerUsage uint64
	systemUsage    uint64
	timestamp      time.Time
}

// DockerEngine orchestrates Minecraft Bedrock instances via Docker Engine API.
type DockerEngine struct {
	cli            *dockerclient.Client
	circuitBreaker *CrashCircuitBreaker

	mu             sync.RWMutex
	ringBuffers    map[string]*RingBuffer
	logChans       map[string][]chan string
	logCancels     map[string]context.CancelFunc
	logListener    func(serverID, line string)

	prevCPUMu      sync.Mutex
	prevCPUSamples map[string]containerCPUSample
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
		prevCPUSamples: make(map[string]containerCPUSample),
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

	networkMode := strings.TrimSpace(server.NetworkMode)
	if networkMode == "" {
		networkMode = "bridge"
	}
	isHostNet := networkMode == "host"

	serverPort := "19132"
	serverPortV6 := "19133"
	if isHostNet {
		serverPort = strconv.Itoa(server.Port)
		if server.PortV6 > 0 {
			serverPortV6 = strconv.Itoa(server.PortV6)
		}
	}

	envVars := []string{
		"EULA=TRUE",
		fmt.Sprintf("SERVER_NAME=%s", server.Name),
		fmt.Sprintf("GAMEMODE=%s", server.Mode),
		fmt.Sprintf("DIFFICULTY=%s", server.Difficulty),
		fmt.Sprintf("SERVER_PORT=%s", serverPort),
		fmt.Sprintf("SERVER_PORT_V6=%s", serverPortV6),
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
	}
	if !isHostNet {
		config.ExposedPorts = nat.PortSet{
			"19132/udp": struct{}{},
			"19133/udp": struct{}{},
		}
	}

	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkMode),
		Binds: []string{
			fmt.Sprintf("%s:/data", hostServerDir),
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
	if !isHostNet {
		hostConfig.PortBindings = nat.PortMap{
			"19132/udp": []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(server.Port)},
			},
			"19133/udp": []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(server.PortV6)},
			},
		}
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
	canonicalName := fmt.Sprintf("bsm-%s", server.ID)
	if containerID == "" {
		containerID = canonicalName
	}

	inspect, err := e.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if dockerclient.IsErrNotFound(err) && containerID != canonicalName {
			// ContainerID might be outdated, try inspecting canonical name
			inspect, err = e.cli.ContainerInspect(ctx, canonicalName)
			if err == nil {
				server.ContainerID = inspect.ID
			}
		}
		if err != nil {
			if dockerclient.IsErrNotFound(err) {
				return models.ServerStatusStopped, nil
			}
			return "", err
		}
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
	canonicalName := fmt.Sprintf("bsm-%s", server.ID)
	if containerID == "" {
		containerID = canonicalName
	}

	statsResp, err := e.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		if dockerclient.IsErrNotFound(err) && containerID != canonicalName {
			containerID = canonicalName
			statsResp, err = e.cli.ContainerStats(ctx, containerID, false)
		}
		if err != nil {
			return nil, err
		}
	}
	defer statsResp.Body.Close()

	var stats types.StatsJSON
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	now := time.Now()
	curContainerUsage := stats.CPUStats.CPUUsage.TotalUsage
	curSystemUsage := stats.CPUStats.SystemUsage

	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0.0 {
		onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0.0 {
		onlineCPUs = 1.0
	}

	e.prevCPUMu.Lock()
	prev, hasPrev := e.prevCPUSamples[server.ID]
	e.prevCPUSamples[server.ID] = containerCPUSample{
		containerUsage: curContainerUsage,
		systemUsage:    curSystemUsage,
		timestamp:      now,
	}
	e.prevCPUMu.Unlock()

	var cpuPercent float64

	// Strategy 1: Check if Docker daemon returned valid non-zero PreCPUStats
	if stats.PreCPUStats.CPUUsage.TotalUsage > 0 && stats.PreCPUStats.SystemUsage > 0 && stats.CPUStats.SystemUsage > stats.PreCPUStats.SystemUsage {
		cpuDelta := float64(curContainerUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
		systemDelta := float64(curSystemUsage - stats.PreCPUStats.SystemUsage)
		if systemDelta > 0.0 && cpuDelta >= 0.0 {
			cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
		}
	} else if hasPrev && curContainerUsage >= prev.containerUsage {
		// Strategy 2: Use in-memory delta from previous sample
		cpuDelta := float64(curContainerUsage - prev.containerUsage)
		var systemDelta float64
		if curSystemUsage > prev.systemUsage {
			systemDelta = float64(curSystemUsage - prev.systemUsage)
		} else {
			elapsedNs := float64(now.Sub(prev.timestamp).Nanoseconds())
			if elapsedNs > 0 {
				systemDelta = elapsedNs * onlineCPUs
			}
		}
		if systemDelta > 0.0 && cpuDelta >= 0.0 {
			cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
		}
	}

	ramBytes := int64(stats.MemoryStats.Usage)
	// Subtract inactive_file or cache if present (matching standard docker stats)
	if v, ok := stats.MemoryStats.Stats["inactive_file"]; ok && int64(v) < ramBytes {
		ramBytes -= int64(v)
	} else if v, ok := stats.MemoryStats.Stats["total_inactive_file"]; ok && int64(v) < ramBytes {
		ramBytes -= int64(v)
	} else if v, ok := stats.MemoryStats.Stats["cache"]; ok && int64(v) < ramBytes {
		ramBytes -= int64(v)
	}
	if ramBytes < 0 {
		ramBytes = 0
	}

	return &models.MetricRaw{
		ServerID:   server.ID,
		Timestamp:  now.UTC(),
		CPUPercent: cpuPercent,
		RAMBytes:   ramBytes,
	}, nil
}

// cleanDockerLogLine strips Docker multiplex framing headers and unprintable characters.
func cleanDockerLogLine(raw []byte) string {
	// Docker demux header: byte 1 (stdout) or 2 (stderr), 3 zeroes, 4 length bytes
	if len(raw) >= 8 && (raw[0] == 1 || raw[0] == 2) && raw[1] == 0 && raw[2] == 0 && raw[3] == 0 {
		raw = raw[8:]
	}
	s := strings.TrimRight(string(raw), "\r\n")
	return strings.ReplaceAll(s, "\x00", "")
}

// SendConsoleCommand pipes a command into the Bedrock server stdin.
func (e *DockerEngine) SendConsoleCommand(ctx context.Context, server *models.Server, cmd string) error {
	containerID := server.ContainerID
	if containerID == "" {
		containerID = fmt.Sprintf("bsm-%s", server.ID)
	}

	// Immediately echo the command into the ring buffer and active subscribers
	echoLine := fmt.Sprintf("> %s", cmd)
	e.mu.Lock()
	if rb, ok := e.ringBuffers[server.ID]; ok {
		rb.Write(echoLine)
	}
	for _, ch := range e.logChans[server.ID] {
		select {
		case ch <- echoLine:
		default:
		}
	}
	if e.logListener != nil {
		e.logListener(server.ID, echoLine)
	}
	e.mu.Unlock()

	// 2. Primary Method: Attach directly to container standard input (PTY / stdin stream)
	// Because containers are created with OpenStdin: true and Tty: true, writing to
	// ContainerAttach stream writes directly to the Bedrock console, identical to docker attach
	// and Portainer console.
	var attachErr error
	hijacked, err := e.cli.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
	})
	if err == nil {
		defer hijacked.Close()
		_, writeErr := fmt.Fprintf(hijacked.Conn, "%s\n", cmd)
		if writeErr == nil {
			return nil
		}
		attachErr = writeErr
	} else {
		attachErr = err
	}

	// 3. Fallback Method: Container Exec as root
	// If ContainerAttach failed or container does not have stdin attached, run send-command or write to /proc
	shScript := `export PATH="/usr/local/bin:/usr/bin:/bin:$PATH"
if command -v send-command >/dev/null 2>&1; then
  send-command "$@" && exit 0
elif [ -x /usr/local/bin/send-command ]; then
  /usr/local/bin/send-command "$@" && exit 0
fi

# Locate the bedrock process in /proc
for proc in $(find /proc -mindepth 2 -maxdepth 2 -name exe \( -lname '*/bedrock_server*' -o -lname '*/box64*' \) -printf '%h\n' 2>/dev/null); do
  if [ -e "$proc/fd/0" ]; then
    printf '%s\n' "$*" > "$proc/fd/0" 2>/dev/null && exit 0
  fi
done

for pid in $(pidof bedrock_server 2>/dev/null) $(pgrep -f bedrock_server 2>/dev/null) 1; do
  if [ -e "/proc/$pid/fd/0" ]; then
    printf '%s\n' "$*" > "/proc/$pid/fd/0" 2>/dev/null && exit 0
  fi
done
exit 1`

	execConfig := types.ExecConfig{
		User:         "root",
		Privileged:   true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          []string{"sh", "-c", shScript, "--", cmd},
	}

	execID, err := e.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		if attachErr != nil {
			return fmt.Errorf("failed delivery via attach (%v) and exec create (%w)", attachErr, err)
		}
		return fmt.Errorf("failed to create exec: %w", err)
	}

	if err := e.cli.ContainerExecStart(ctx, execID.ID, types.ExecStartCheck{Tty: true}); err != nil {
		if attachErr != nil {
			return fmt.Errorf("failed delivery via attach (%v) and exec start (%w)", attachErr, err)
		}
		return fmt.Errorf("failed to start exec: %w", err)
	}

	return nil
}

// GetRecentLogs retrieves lines from the server ring buffer, or queries Docker if empty.
func (e *DockerEngine) GetRecentLogs(serverID string) []string {
	e.mu.RLock()
	rb, exists := e.ringBuffers[serverID]
	e.mu.RUnlock()

	if exists && rb.Count() > 0 {
		return rb.GetAll()
	}

	// Ring buffer is empty (e.g. BSM was just started). Attempt one-shot fetch from Docker.
	containerID := fmt.Sprintf("bsm-%s", serverID)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	reader, err := e.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "200",
	})
	if err != nil {
		if exists {
			return rb.GetAll()
		}
		return []string{}
	}
	defer reader.Close()

	var lines []string
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		clean := cleanDockerLogLine(scanner.Bytes())
		if clean != "" {
			lines = append(lines, clean)
		}
	}

	if len(lines) > 0 {
		e.mu.Lock()
		if !exists {
			rb = NewRingBuffer(1000)
			e.ringBuffers[serverID] = rb
		}
		for _, l := range lines {
			rb.Write(l)
		}
		e.mu.Unlock()
	}

	return lines
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

// AttachLogCapture initiates log capture if not already running for this server.
func (e *DockerEngine) AttachLogCapture(serverID, containerID string) {
	if containerID == "" {
		containerID = fmt.Sprintf("bsm-%s", serverID)
	}

	e.mu.Lock()
	if _, running := e.logCancels[serverID]; running {
		e.mu.Unlock()
		return // Log capture already running
	}
	e.mu.Unlock()

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
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := cleanDockerLogLine(scanner.Bytes())
		if line == "" {
			continue
		}
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

// ListNetworks queries the Docker daemon for available networks.
func (e *DockerEngine) ListNetworks(ctx context.Context) ([]string, error) {
	networks, err := e.cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return []string{"bridge", "host"}, nil
	}
	var names []string
	hasBridge := false
	hasHost := false
	for _, n := range networks {
		if n.Name == "none" {
			continue
		}
		if n.Name == "bridge" {
			hasBridge = true
		}
		if n.Name == "host" {
			hasHost = true
		}
		names = append(names, n.Name)
	}
	if !hasBridge {
		names = append([]string{"bridge"}, names...)
	}
	if !hasHost {
		names = append(names, "host")
	}
	return names, nil
}
