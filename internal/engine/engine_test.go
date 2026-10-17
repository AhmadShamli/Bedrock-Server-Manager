package engine

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/docker/docker/api/types"
)

func TestParseMemoryBytes(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		err      bool
	}{
		{"512M", 512 * 1024 * 1024, false},
		{"1G", 1024 * 1024 * 1024, false},
		{"2G", 2 * 1024 * 1024 * 1024, false},
		{"4GB", 4 * 1024 * 1024 * 1024, false},
		{"", 2 * 1024 * 1024 * 1024, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		bytes, err := ParseMemoryBytes(tt.input)
		if (err != nil) != tt.err {
			t.Errorf("input %s: unexpected err state %v", tt.input, err)
		}
		if !tt.err && bytes != tt.expected {
			t.Errorf("input %s: expected %d, got %d", tt.input, tt.expected, bytes)
		}
	}
}

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Write("line 1")
	rb.Write("line 2")

	all := rb.GetAll()
	if len(all) != 2 || all[0] != "line 1" || all[1] != "line 2" {
		t.Fatalf("unexpected lines: %v", all)
	}

	// Overwrite ring buffer
	rb.Write("line 3")
	rb.Write("line 4") // Should overwrite line 1

	all = rb.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(all))
	}
	if all[0] != "line 2" || all[1] != "line 3" || all[2] != "line 4" {
		t.Fatalf("unexpected wrapped lines: %v", all)
	}
}

func TestCrashCircuitBreaker(t *testing.T) {
	cb := NewCrashCircuitBreaker(3, 5*time.Minute)
	now := time.Now()

	if cb.IsTripped("srv-1") {
		t.Errorf("expected not tripped initially")
	}

	cb.RecordCrash("srv-1", now)
	cb.RecordCrash("srv-1", now.Add(10*time.Second))
	if cb.IsTripped("srv-1") {
		t.Errorf("expected not tripped after 2 crashes")
	}

	tripped := cb.RecordCrash("srv-1", now.Add(20*time.Second))
	if !tripped || !cb.IsTripped("srv-1") {
		t.Errorf("expected circuit breaker to trip on 3rd crash")
	}

	// Reset
	cb.Reset("srv-1")
	if cb.IsTripped("srv-1") {
		t.Errorf("expected circuit breaker to be reset")
	}
}

func TestMockEngine(t *testing.T) {
	ctx := context.Background()
	mock := NewMockEngine()

	srv := &models.Server{
		ID:          "mock-1",
		Name:        "Test Realm",
		Port:        19132,
		PortV6:      19133,
		MemoryLimit: "2G",
		CPULimit:    2.0,
	}

	cid, err := mock.CreateServer(ctx, srv, "data")
	if err != nil || cid == "" {
		t.Fatalf("CreateServer failed: %v", err)
	}

	srv.ContainerID = cid
	if err := mock.StartServer(ctx, srv); err != nil {
		t.Fatalf("StartServer failed: %v", err)
	}

	status, err := mock.GetServerStatus(ctx, srv)
	if err != nil || status != models.ServerStatusRunning {
		t.Errorf("expected running status, got %s", status)
	}

	// Logs & Command
	ch, unsub := mock.SubscribeLogs(srv.ID)
	defer unsub()

	if err := mock.SendConsoleCommand(ctx, srv, "say Hello"); err != nil {
		t.Fatalf("SendConsoleCommand failed: %v", err)
	}

	select {
	case line := <-ch:
		if line != "[MOCK CMD] > say Hello" {
			t.Errorf("unexpected command line: %s", line)
		}
	case <-time.After(1 * time.Second):
		t.Errorf("timed out waiting for log broadcast")
	}

	if err := mock.StopServer(ctx, srv, 5); err != nil {
		t.Fatalf("StopServer failed: %v", err)
	}
	status, _ = mock.GetServerStatus(ctx, srv)
	if status != models.ServerStatusStopped {
		t.Errorf("expected stopped status, got %s", status)
	}
}

func TestBootManager(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenManagerDB(":memory:")
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}
	defer db.Close()

	mock := NewMockEngine()

	// Server 1: autostart = true
	s1 := &models.Server{
		ID:              "autostart-srv",
		Name:            "Auto Realm",
		Port:            19132,
		PortV6:          19133,
		AutostartOnBoot: true,
	}
	_ = db.CreateServer(ctx, s1)
	cid1, _ := mock.CreateServer(ctx, s1, "data")
	s1.ContainerID = cid1
	_ = db.UpdateServerStatus(ctx, s1.ID, models.ServerStatusStopped, cid1)

	// Server 2: autostart = false
	s2 := &models.Server{
		ID:              "manual-srv",
		Name:            "Manual Realm",
		Port:            19134,
		PortV6:          19135,
		AutostartOnBoot: false,
	}
	_ = db.CreateServer(ctx, s2)
	cid2, _ := mock.CreateServer(ctx, s2, "data")
	s2.ContainerID = cid2
	_ = db.UpdateServerStatus(ctx, s2.ID, models.ServerStatusStopped, cid2)

	bm := NewBootManager(db, mock)
	started, err := bm.AutostartServers(ctx)
	if err != nil {
		t.Fatalf("AutostartServers failed: %v", err)
	}

	if len(started) != 1 || started[0] != "autostart-srv" {
		t.Errorf("expected only 'autostart-srv' to start, got %v", started)
	}

	status1, _ := mock.GetServerStatus(ctx, s1)
	if status1 != models.ServerStatusRunning {
		t.Errorf("expected s1 running, got %s", status1)
	}

	status2, _ := mock.GetServerStatus(ctx, s2)
	if status2 != models.ServerStatusStopped {
		t.Errorf("expected s2 stopped, got %s", status2)
	}
}

func TestComputeContainerCPUPercent(t *testing.T) {
	now := time.Now()

	// Case 1: Container just restarted — PreCPUStats has higher TotalUsage than current cgroup TotalUsage.
	// Previously this caused a uint64 underflow producing ~1,853,944,127,530.3% CPU!
	var underflowStats types.StatsJSON
	underflowStats.ID = "cid-1"
	underflowStats.CPUStats.OnlineCPUs = 4
	underflowStats.CPUStats.CPUUsage.TotalUsage = 10_000_000        // 10ms in new container lifecycle
	underflowStats.PreCPUStats.CPUUsage.TotalUsage = 50_000_000_000 // 50s from previous run
	underflowStats.CPUStats.SystemUsage = 1_000_000_000_000
	underflowStats.PreCPUStats.SystemUsage = 999_000_000_000

	cpu, sample, update := computeContainerCPUPercent(underflowStats, containerCPUSample{}, false, now)
	if !update {
		t.Fatalf("expected initial sample to be stored")
	}
	if cpu < 0 || cpu > 400.0 || math.IsNaN(cpu) || math.IsInf(cpu, 0) {
		t.Fatalf("expected safe fallback CPU on counter reset (got %f%%)", cpu)
	}
	if cpu != 0.0 {
		t.Fatalf("expected 0.0%% on initial baseline after counter reset, got %f%%", cpu)
	}

	// Case 2: Normal Strategy 1 calculation (valid PreCPUStats and CPUStats)
	var normalStats types.StatsJSON
	normalStats.ID = "cid-1"
	normalStats.CPUStats.OnlineCPUs = 4
	normalStats.PreCPUStats.CPUUsage.TotalUsage = 100_000_000
	normalStats.CPUStats.CPUUsage.TotalUsage = 150_000_000 // +50ms container CPU
	normalStats.PreCPUStats.SystemUsage = 1_000_000_000
	normalStats.CPUStats.SystemUsage = 5_000_000_000 // +4s system usage across 4 CPUs (1s wall clock)

	t2 := now.Add(2 * time.Second)
	cpu, sample, update = computeContainerCPUPercent(normalStats, sample, true, t2)
	if !update {
		t.Fatalf("expected sample update on normal calculation")
	}
	// (50,000,000 / 4,000,000,000) * 4 * 100 = 5.0%
	if math.Abs(cpu-5.0) > 0.001 {
		t.Fatalf("expected 5.0%% CPU, got %f%%", cpu)
	}

	// Case 3: Sub-250ms concurrent poll when PreCPUStats is zero should reuse cached percentage
	// without overwriting baseline or dividing by tiny elapsed time.
	var zeroPreStats types.StatsJSON
	zeroPreStats.ID = "cid-1"
	zeroPreStats.CPUStats.OnlineCPUs = 4
	zeroPreStats.CPUStats.CPUUsage.TotalUsage = 151_000_000
	t3 := t2.Add(10 * time.Millisecond)
	cpuConcurrent, _, updateConcurrent := computeContainerCPUPercent(zeroPreStats, sample, true, t3)
	if updateConcurrent {
		t.Fatalf("expected sub-250ms concurrent poll not to overwrite baseline sample")
	}
	if math.Abs(cpuConcurrent-5.0) > 0.001 {
		t.Fatalf("expected cached 5.0%% CPU on rapid concurrent poll, got %f%%", cpuConcurrent)
	}
}

