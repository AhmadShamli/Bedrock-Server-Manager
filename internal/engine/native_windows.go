//go:build windows

package engine

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/bds"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/configfile"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"golang.org/x/sys/windows"
)

const (
	JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE   = 0x00002000
	JOB_OBJECT_LIMIT_JOB_MEMORY          = 0x00000200
	JobObjectExtendedLimitInformation    = 9
	JobObjectCpuRateControlInformation   = 15
	JOB_OBJECT_CPU_RATE_CONTROL_ENABLE   = 0x00000001
	JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP = 0x00000004

	CREATE_NO_WINDOW = 0x08000000
)

var (
	modKernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procCreateJobObjectW         = modKernel32.NewProc("CreateJobObjectW")
	procAssignProcessToJobObject = modKernel32.NewProc("AssignProcessToJobObject")
	procSetInformationJobObject  = modKernel32.NewProc("SetInformationJobObject")

	modPsapi                 = windows.NewLazySystemDLL("psapi.dll")
	procGetProcessMemoryInfo = modPsapi.NewProc("GetProcessMemoryInfo")
)

type ioCounters struct {
	readOperationCount  uint64
	writeOperationCount uint64
	otherOperationCount uint64
	readTransferCount   uint64
	writeTransferCount  uint64
	otherTransferCount  uint64
}

type jobObjectBasicLimitInformation struct {
	perProcessUserTimeLimit int64
	perJobUserTimeLimit     int64
	limitFlags              uint32
	minimumWorkingSetSize   uintptr
	maximumWorkingSetSize   uintptr
	activeProcessLimit      uint32
	affinity                uintptr
	priorityClass           uint32
	schedulingClass         uint32
}

type jobObjectExtendedLimitInformation struct {
	basicLimitInformation jobObjectBasicLimitInformation
	ioInfo                ioCounters
	processMemoryLimit    uintptr
	jobMemoryLimit        uintptr
	peakProcessMemoryUsed uintptr
	peakJobMemoryUsed     uintptr
}

type jobObjectCpuRateControlInformation struct {
	controlFlags uint32
	cpuRate      uint32
}

type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

type nativeCPUSample struct {
	timestamp   time.Time
	kernelTicks uint64
	userTicks   uint64
}

type nativeProcess struct {
	serverID      string
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	jobHandle     windows.Handle
	processID     int
	startedAt     time.Time
	stopRequested bool
	exitedChan    chan struct{}
	exitCode      int
	crashHistory  []time.Time
}

// NativeEngine orchestrates Minecraft Bedrock instances directly as Windows child processes.
type NativeEngine struct {
	dataDir        string
	bdsCache       *bds.CacheManager
	bdsResolver    *bds.Resolver
	db             bds.SettingGetter
	circuitBreaker *CrashCircuitBreaker

	mu             sync.RWMutex
	processes      map[string]*nativeProcess
	ringBuffers    map[string]*RingBuffer
	logChans       map[string][]chan string
	logListener    func(serverID, line string)

	prevCPUMu      sync.Mutex
	prevCPUSamples map[string]nativeCPUSample
}

// NewNativeEngine initializes the NativeEngine for Windows.
func NewNativeEngine(dataDir string, bdsCache *bds.CacheManager, bdsResolver *bds.Resolver, db bds.SettingGetter) (*NativeEngine, error) {
	return &NativeEngine{
		dataDir:        dataDir,
		bdsCache:       bdsCache,
		bdsResolver:    bdsResolver,
		db:             db,
		circuitBreaker: NewCrashCircuitBreaker(5, 5*time.Minute),
		processes:      make(map[string]*nativeProcess),
		ringBuffers:    make(map[string]*RingBuffer),
		logChans:       make(map[string][]chan string),
		prevCPUSamples: make(map[string]nativeCPUSample),
	}, nil
}

// CheckVCRuntime checks for required Visual C++ 2015-2022 x64 runtime libraries.
func CheckVCRuntime() error {
	sysRoot := os.Getenv("SystemRoot")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	sys32 := filepath.Join(sysRoot, "System32")
	msvcp := filepath.Join(sys32, "msvcp140.dll")
	vcrun := filepath.Join(sys32, "vcruntime140.dll")

	_, errM := os.Stat(msvcp)
	_, errV := os.Stat(vcrun)
	if errM != nil || errV != nil {
		return fmt.Errorf("missing Microsoft Visual C++ 2015-2022 Redistributable (x64). Please install it from https://aka.ms/vs/17/release/vc_redist.x64.exe to run Bedrock Dedicated Server")
	}
	return nil
}

func (e *NativeEngine) getServerDir(serverID string) string {
	return filepath.Join(e.dataDir, "servers", serverID)
}

func (e *NativeEngine) CreateServer(ctx context.Context, server *models.Server, dataDir string) (string, error) {
	if err := CheckVCRuntime(); err != nil {
		return "", err
	}

	serverDir := e.getServerDir(server.ID)
	if err := os.MkdirAll(serverDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create server directory: %w", err)
	}

	// Verify EULA acceptance
	eulaAccepted, _ := bds.IsEULAAccepted(ctx, e.db)
	if !eulaAccepted {
		return "", bds.ErrEULANotAccepted
	}

	// Resolve requested BDS version
	verSelector := server.BDSVersion
	if verSelector == "" {
		verSelector = "latest"
	}
	resolved, err := e.bdsResolver.ResolveVersion(ctx, verSelector, false)
	if err != nil {
		return "", fmt.Errorf("failed to resolve BDS version: %w", err)
	}

	// Download and extract into cache
	cacheDir, err := e.bdsCache.EnsureVersion(ctx, resolved, true)
	if err != nil {
		return "", fmt.Errorf("failed to prepare BDS version %s: %w", resolved.Version, err)
	}

	// Install binaries into instance directory
	if err := bds.InstallServerFiles(serverDir, cacheDir, false); err != nil {
		return "", fmt.Errorf("failed to install server binaries: %w", err)
	}

	server.InstalledBDSVersion = resolved.Version

	// Generate baseline server.properties
	propsPath := filepath.Join(serverDir, "server.properties")
	if _, err := os.Stat(propsPath); os.IsNotExist(err) {
		props, keys := configfile.GenerateDefaultProperties(server)
		_ = configfile.WriteProperties(propsPath, props, keys)
	}

	e.mu.Lock()
	if _, exists := e.ringBuffers[server.ID]; !exists {
		e.ringBuffers[server.ID] = NewRingBuffer(1000)
	}
	e.mu.Unlock()

	return fmt.Sprintf("native-%s", server.ID), nil
}

func (e *NativeEngine) StartServer(ctx context.Context, server *models.Server) error {
	if err := CheckVCRuntime(); err != nil {
		return err
	}

	if e.circuitBreaker.IsTripped(server.ID) {
		return fmt.Errorf("circuit breaker tripped for server %s (repeated crashes)", server.ID)
	}

	e.mu.Lock()
	if proc, exists := e.processes[server.ID]; exists && proc.cmd != nil && proc.cmd.Process != nil {
		e.mu.Unlock()
		return nil // Already running
	}
	e.mu.Unlock()

	serverDir := e.getServerDir(server.ID)
	exePath := filepath.Join(serverDir, "bedrock_server.exe")

	// If binary does not exist, provision it
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		if _, err := e.CreateServer(ctx, server, e.dataDir); err != nil {
			return err
		}
	} else if server.UpdatePolicy == models.UpdatePolicyAuto {
		// Auto update check on start (Decision 4 & 16)
		resolved, err := e.bdsResolver.ResolveVersion(ctx, server.BDSVersion, false)
		if err == nil && resolved.Version != "" && resolved.Version != server.InstalledBDSVersion {
			log.Printf("[BDS] Auto-update found newer version %s for server %s (current: %s)", resolved.Version, server.ID, server.InstalledBDSVersion)
			cacheDir, err := e.bdsCache.EnsureVersion(ctx, resolved, true)
			if err == nil {
				if err := bds.InstallServerFiles(serverDir, cacheDir, true); err == nil {
					server.PreviousBDSVersion = server.InstalledBDSVersion
					server.InstalledBDSVersion = resolved.Version
				}
			}
		}
	}

	// Update server.properties with any model changes (ports, seed, etc.)
	propsPath := filepath.Join(serverDir, "server.properties")
	existingProps, existingKeys, _ := configfile.ReadProperties(propsPath)
	mergedProps, mergedKeys := configfile.MergeDefaultProperties(existingProps, existingKeys, server)
	_ = configfile.WriteProperties(propsPath, mergedProps, mergedKeys)

	// Configure Job Object with RAM and CPU caps
	memBytes, _ := ParseMemoryBytes(server.MemoryLimit)
	hJob, err := createWindowsJob(memBytes, server.CPULimit)
	if err != nil {
		log.Printf("[WARN] [NativeEngine] Failed to create Job Object (%v), running without Job limits", err)
	}

	cmd := exec.Command(exePath)
	cmd.Dir = serverDir
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW,
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		if hJob != 0 {
			windows.CloseHandle(hJob)
		}
		return fmt.Errorf("failed to open stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		if hJob != 0 {
			windows.CloseHandle(hJob)
		}
		return fmt.Errorf("failed to open stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		if hJob != 0 {
			windows.CloseHandle(hJob)
		}
		return fmt.Errorf("failed to open stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		if hJob != 0 {
			windows.CloseHandle(hJob)
		}
		return fmt.Errorf("failed to start bedrock_server.exe: %w", err)
	}

	pid := cmd.Process.Pid
	if hJob != 0 {
		hProcess, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_INFORMATION, false, uint32(pid))
		if err == nil {
			procAssignProcessToJobObject.Call(uintptr(hJob), uintptr(hProcess))
			windows.CloseHandle(hProcess)
		}
	}

	proc := &nativeProcess{
		serverID:   server.ID,
		cmd:        cmd,
		stdin:      stdinPipe,
		jobHandle:  hJob,
		processID:  pid,
		startedAt:  time.Now().UTC(),
		exitedChan: make(chan struct{}),
	}

	e.mu.Lock()
	e.processes[server.ID] = proc
	rb, exists := e.ringBuffers[server.ID]
	if !exists {
		rb = NewRingBuffer(1000)
		e.ringBuffers[server.ID] = rb
	}
	e.mu.Unlock()

	// Console log file rotation
	logDir := filepath.Join(serverDir, "logs")
	_ = os.MkdirAll(logDir, 0750)
	logFile, _ := os.OpenFile(
		filepath.Join(logDir, fmt.Sprintf("console-%s.log", time.Now().Format("2006-01-02"))),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0640,
	)

	// Stream stdout & stderr lines
	go e.pipeScanner(server.ID, stdoutPipe, rb, logFile)
	go e.pipeScanner(server.ID, stderrPipe, rb, logFile)

	// Supervisor goroutine
	go e.supervise(server, proc, logFile)

	return nil
}

func (e *NativeEngine) pipeScanner(serverID string, r io.Reader, rb *RingBuffer, logFile *os.File) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		rb.Write(line)

		if logFile != nil {
			_, _ = fmt.Fprintln(logFile, line)
		}

		e.mu.RLock()
		subs := e.logChans[serverID]
		listener := e.logListener
		e.mu.RUnlock()

		for _, ch := range subs {
			select {
			case ch <- line:
			default:
			}
		}

		if listener != nil {
			listener(serverID, line)
		}
	}
}

func (e *NativeEngine) supervise(server *models.Server, proc *nativeProcess, logFile *os.File) {
	waitErr := proc.cmd.Wait()
	close(proc.exitedChan)

	if logFile != nil {
		_ = logFile.Close()
	}

	if proc.jobHandle != 0 {
		windows.CloseHandle(proc.jobHandle)
	}

	e.mu.Lock()
	delete(e.processes, server.ID)
	e.mu.Unlock()

	if proc.stopRequested {
		return // Clean expected shutdown
	}

	now := time.Now().UTC()
	log.Printf("[WARN] [NativeEngine] Server %s exited unexpectedly (%v)", server.ID, waitErr)
	e.circuitBreaker.RecordCrash(server.ID, now)

	proc.crashHistory = append(proc.crashHistory, now)

	// Check if 3 crashes within 60s (Decision 17: automatic rollback to previous version)
	recentCount := 0
	for _, t := range proc.crashHistory {
		if now.Sub(t) <= 60*time.Second {
			recentCount++
		}
	}

	if recentCount >= 3 && server.PreviousBDSVersion != "" {
		log.Printf("[BDS] Server %s crashed %d times in 60s. Triggering automatic rollback to version %s", server.ID, recentCount, server.PreviousBDSVersion)
		prevDir := e.bdsCache.VersionDir(server.PreviousBDSVersion)
		if e.bdsCache.IsVersionCached(server.PreviousBDSVersion) {
			_ = bds.RollbackServerFiles(e.getServerDir(server.ID), prevDir)
			server.InstalledBDSVersion = server.PreviousBDSVersion
			server.UpdatePolicy = models.UpdatePolicyManual // Pin to prevent auto-reupdating
		}
	}

	if !e.circuitBreaker.IsTripped(server.ID) {
		// Backoff restart: 2s
		time.Sleep(2 * time.Second)
		_ = e.StartServer(context.Background(), server)
	}
}

func (e *NativeEngine) StopServer(ctx context.Context, server *models.Server, timeoutSeconds int) error {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 15
	}

	e.mu.Lock()
	proc, exists := e.processes[server.ID]
	e.mu.Unlock()

	if !exists || proc == nil {
		return nil
	}

	proc.stopRequested = true

	// Send graceful stop command to stdin
	if proc.stdin != nil {
		_, _ = fmt.Fprintln(proc.stdin, "stop")
	}

	select {
	case <-proc.exitedChan:
		return nil
	case <-time.After(time.Duration(timeoutSeconds) * time.Second):
		log.Printf("[WARN] [NativeEngine] Server %s did not terminate in %ds, killing process", server.ID, timeoutSeconds)
		if proc.cmd != nil && proc.cmd.Process != nil {
			_ = proc.cmd.Process.Kill()
		}
		return nil
	}
}

func (e *NativeEngine) RestartServer(ctx context.Context, server *models.Server) error {
	_ = e.StopServer(ctx, server, 15)
	return e.StartServer(ctx, server)
}

func (e *NativeEngine) RemoveServer(ctx context.Context, server *models.Server, removeData bool) error {
	_ = e.StopServer(ctx, server, 5)

	if removeData {
		serverDir := e.getServerDir(server.ID)
		_ = os.RemoveAll(serverDir)
	}

	e.mu.Lock()
	delete(e.processes, server.ID)
	delete(e.ringBuffers, server.ID)
	e.mu.Unlock()

	return nil
}

func (e *NativeEngine) GetServerStatus(ctx context.Context, server *models.Server) (string, error) {
	e.mu.RLock()
	proc, exists := e.processes[server.ID]
	e.mu.RUnlock()

	if exists && proc != nil && proc.cmd != nil && proc.cmd.Process != nil {
		select {
		case <-proc.exitedChan:
			if e.circuitBreaker.IsTripped(server.ID) {
				return models.ServerStatusCrashed, nil
			}
			return models.ServerStatusStopped, nil
		default:
			return models.ServerStatusRunning, nil
		}
	}

	if e.circuitBreaker.IsTripped(server.ID) {
		return models.ServerStatusCrashed, nil
	}
	return models.ServerStatusStopped, nil
}

func (e *NativeEngine) GetContainerStats(ctx context.Context, server *models.Server) (*models.MetricRaw, error) {
	e.mu.RLock()
	proc, exists := e.processes[server.ID]
	e.mu.RUnlock()

	if !exists || proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
		return nil, nil
	}

	hProcess, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(proc.processID))
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(hProcess)

	// Memory usage
	var pmc processMemoryCounters
	pmc.cb = uint32(unsafe.Sizeof(pmc))
	r1, _, _ := procGetProcessMemoryInfo.Call(
		uintptr(hProcess),
		uintptr(unsafe.Pointer(&pmc)),
		uintptr(pmc.cb),
	)
	var ramBytes int64
	if r1 != 0 {
		ramBytes = int64(pmc.workingSetSize)
	}

	// CPU calculation
	var creationTime, exitTime, kernelTime, userTime windows.Filetime
	var cpuPercent float64

	if err := windows.GetProcessTimes(hProcess, &creationTime, &exitTime, &kernelTime, &userTime); err == nil {
		curKernel := (uint64(kernelTime.HighDateTime) << 32) | uint64(kernelTime.LowDateTime)
		curUser := (uint64(userTime.HighDateTime) << 32) | uint64(userTime.LowDateTime)
		now := time.Now().UTC()

		e.prevCPUMu.Lock()
		prev, ok := e.prevCPUSamples[server.ID]
		e.prevCPUSamples[server.ID] = nativeCPUSample{
			timestamp:   now,
			kernelTicks: curKernel,
			userTicks:   curUser,
		}
		e.prevCPUMu.Unlock()

		if ok && now.After(prev.timestamp) {
			wallMs := now.Sub(prev.timestamp).Milliseconds()
			if wallMs > 0 {
				totalProcessTicksDelta := (curKernel - prev.kernelTicks) + (curUser - prev.userTicks)
				// 1 tick = 100ns = 0.0001ms
				processMs := float64(totalProcessTicksDelta) / 10000.0
				cpuPercent = (processMs / float64(wallMs)) * 100.0
				if math.IsNaN(cpuPercent) || math.IsInf(cpuPercent, 0) || cpuPercent < 0 {
					cpuPercent = 0
				}
			}
		}
	}

	return &models.MetricRaw{
		ServerID:   server.ID,
		Timestamp:  time.Now().UTC(),
		CPUPercent: cpuPercent,
		RAMBytes:   ramBytes,
	}, nil
}

func (e *NativeEngine) SendConsoleCommand(ctx context.Context, server *models.Server, cmd string) error {
	e.mu.RLock()
	proc, exists := e.processes[server.ID]
	e.mu.RUnlock()

	if !exists || proc == nil || proc.stdin == nil {
		return fmt.Errorf("server %s is not running", server.ID)
	}

	cmd = strings.TrimSpace(cmd)
	_, err := fmt.Fprintln(proc.stdin, cmd)
	return err
}

func (e *NativeEngine) GetRecentLogs(serverID string) []string {
	e.mu.RLock()
	rb, exists := e.ringBuffers[serverID]
	e.mu.RUnlock()

	if exists && rb != nil {
		return rb.GetAll()
	}
	return []string{}
}

func (e *NativeEngine) SubscribeLogs(serverID string) (<-chan string, func()) {
	ch := make(chan string, 100)

	e.mu.Lock()
	e.logChans[serverID] = append(e.logChans[serverID], ch)
	e.mu.Unlock()

	unsubscribe := func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		subs := e.logChans[serverID]
		for i, sub := range subs {
			if sub == ch {
				e.logChans[serverID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, unsubscribe
}

func (e *NativeEngine) AttachLogCapture(serverID, containerID string) {}

func (e *NativeEngine) SetLogListener(listener func(serverID, line string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logListener = listener
}

func (e *NativeEngine) ListNetworks(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

// StopAllGracefully concurrently stops all running native BDS servers within timeout (Decision 33).
func (e *NativeEngine) StopAllGracefully(timeoutSeconds int) {
	e.mu.RLock()
	var running []*nativeProcess
	for _, p := range e.processes {
		if p != nil {
			running = append(running, p)
		}
	}
	e.mu.RUnlock()

	if len(running) == 0 {
		return
	}

	log.Printf("[NativeEngine] Gracefully stopping %d native Bedrock instances...", len(running))
	var wg sync.WaitGroup
	for _, p := range running {
		wg.Add(1)
		go func(proc *nativeProcess) {
			defer wg.Done()
			proc.stopRequested = true
			if proc.stdin != nil {
				_, _ = fmt.Fprintln(proc.stdin, "stop")
			}
			select {
			case <-proc.exitedChan:
			case <-time.After(time.Duration(timeoutSeconds) * time.Second):
				if proc.cmd != nil && proc.cmd.Process != nil {
					_ = proc.cmd.Process.Kill()
				}
			}
		}(p)
	}
	wg.Wait()
	log.Println("[NativeEngine] All native Bedrock instances stopped.")
}

func createWindowsJob(memLimitBytes int64, cpuLimit float64) (windows.Handle, error) {
	r1, _, err := procCreateJobObjectW.Call(0, 0)
	if r1 == 0 {
		return 0, err
	}
	hJob := windows.Handle(r1)

	var extInfo jobObjectExtendedLimitInformation
	extInfo.basicLimitInformation.limitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if memLimitBytes > 0 {
		extInfo.basicLimitInformation.limitFlags |= JOB_OBJECT_LIMIT_JOB_MEMORY
		extInfo.jobMemoryLimit = uintptr(memLimitBytes)
	}

	rSet, _, errSet := procSetInformationJobObject.Call(
		uintptr(hJob),
		uintptr(JobObjectExtendedLimitInformation),
		uintptr(unsafe.Pointer(&extInfo)),
		uintptr(unsafe.Sizeof(extInfo)),
	)
	if rSet == 0 {
		windows.CloseHandle(hJob)
		return 0, errSet
	}

	if cpuLimit > 0 {
		var cpuInfo jobObjectCpuRateControlInformation
		cpuInfo.controlFlags = JOB_OBJECT_CPU_RATE_CONTROL_ENABLE | JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP
		rate := uint32(cpuLimit * 10000)
		if rate > 10000 {
			rate = 10000
		}
		cpuInfo.cpuRate = rate
		_, _, _ = procSetInformationJobObject.Call(
			uintptr(hJob),
			uintptr(JobObjectCpuRateControlInformation),
			uintptr(unsafe.Pointer(&cpuInfo)),
			uintptr(unsafe.Sizeof(cpuInfo)),
		)
	}

	return hJob, nil
}
