package engine

import (
	"testing"
	"time"
)

func TestComputeNativeProcessCPUPercent_InitialSample(t *testing.T) {
	now := time.Now()
	cpu, sample, update := computeNativeProcessCPUPercent(1234, 1000, 2000, now, nativeCPUSample{}, false, 4)
	if cpu != 0.0 {
		t.Fatalf("expected initial CPU percent to be 0.0, got %f", cpu)
	}
	if !update {
		t.Fatalf("expected updateSample to be true for initial sample")
	}
	if sample.processID != 1234 || sample.kernelTicks != 1000 || sample.userTicks != 2000 {
		t.Fatalf("unexpected sample fields: %+v", sample)
	}
}

func TestComputeNativeProcessCPUPercent_ProcessRestart_PIDMismatch(t *testing.T) {
	t0 := time.Now()
	oldSample := nativeCPUSample{
		processID:      1000,
		timestamp:      t0,
		kernelTicks:    50_000_000, // Process ran for a long time previously
		userTicks:      90_000_000,
		lastCPUPercent: 75.0,
	}

	// New process just started with PID 2000 and has only accumulated 500 ticks
	t1 := t0.Add(1 * time.Second)
	cpu, sample, update := computeNativeProcessCPUPercent(2000, 200, 300, t1, oldSample, true, 4)
	if cpu != 0.0 {
		t.Fatalf("expected restarted process with new PID to report 0.0%% CPU, got %f", cpu)
	}
	if !update {
		t.Fatalf("expected updateSample to be true")
	}
	if sample.processID != 2000 {
		t.Fatalf("expected new sample processID to be 2000, got %d", sample.processID)
	}
	if sample.kernelTicks != 200 || sample.userTicks != 300 {
		t.Fatalf("unexpected sample ticks: %+v", sample)
	}
}

func TestComputeNativeProcessCPUPercent_UnderflowProtection(t *testing.T) {
	t0 := time.Now()
	prev := nativeCPUSample{
		processID:      1234,
		timestamp:      t0,
		kernelTicks:    5000,
		userTicks:      5000,
		lastCPUPercent: 10.0,
	}

	// Ticks went backwards (anomaly)
	t1 := t0.Add(1 * time.Second)
	cpu, sample, update := computeNativeProcessCPUPercent(1234, 4000, 4000, t1, prev, true, 4)
	if cpu != 0.0 {
		t.Fatalf("expected 0.0%% CPU on tick decrease, got %f", cpu)
	}
	if !update {
		t.Fatalf("expected updateSample to be true")
	}
	if cpu > 1000.0 {
		t.Fatalf("detected integer underflow! CPU is %f", cpu)
	}
	_ = sample
}

func TestComputeNativeProcessCPUPercent_ThrottledCall(t *testing.T) {
	t0 := time.Now()
	prev := nativeCPUSample{
		processID:      1234,
		timestamp:      t0,
		kernelTicks:    1000,
		userTicks:      1000,
		lastCPUPercent: 42.5,
	}

	// Called only 100ms later (< 250ms threshold)
	t1 := t0.Add(100 * time.Millisecond)
	cpu, sample, update := computeNativeProcessCPUPercent(1234, 2000, 2000, t1, prev, true, 4)
	if cpu != 42.5 {
		t.Fatalf("expected throttled call to return previous CPU percent 42.5, got %f", cpu)
	}
	if update {
		t.Fatalf("expected updateSample to be false for throttled call")
	}
	if sample.timestamp != t0 {
		t.Fatalf("expected baseline timestamp to remain unchanged")
	}
}

func TestComputeNativeProcessCPUPercent_AccurateCalculation(t *testing.T) {
	t0 := time.Now()
	prev := nativeCPUSample{
		processID:      1234,
		timestamp:      t0,
		kernelTicks:    10_000_000,
		userTicks:      20_000_000,
		lastCPUPercent: 0.0,
	}

	// 1 second elapsed (1e9 ns).
	// Process consumed:
	//   kernel: 1_000_000 ticks (100ms = 0.1s)
	//   user:   3_000_000 ticks (300ms = 0.3s)
	// Total: 4_000_000 ticks = 400,000,000 ns = 0.4s CPU time -> 40.0% CPU
	t1 := t0.Add(1 * time.Second)
	cpu, sample, update := computeNativeProcessCPUPercent(1234, 11_000_000, 23_000_000, t1, prev, true, 4)
	if !update {
		t.Fatalf("expected updateSample to be true")
	}
	if cpu < 39.9 || cpu > 40.1 {
		t.Fatalf("expected approximately 40.0%% CPU, got %f", cpu)
	}
	if sample.lastCPUPercent != cpu {
		t.Fatalf("expected sample.lastCPUPercent to match cpu, got %f", sample.lastCPUPercent)
	}
}

func TestComputeNativeProcessCPUPercent_ClampedToHostCores(t *testing.T) {
	t0 := time.Now()
	prev := nativeCPUSample{
		processID:      1234,
		timestamp:      t0,
		kernelTicks:    0,
		userTicks:      0,
		lastCPUPercent: 0.0,
	}

	// 1 second elapsed, but delta ticks simulate 10 cores busy (10,000,000,000 ns) on a 2-core system
	t1 := t0.Add(1 * time.Second)
	cpu, _, _ := computeNativeProcessCPUPercent(1234, 50_000_000, 50_000_000, t1, prev, true, 2)
	if cpu != 200.0 {
		t.Fatalf("expected CPU to be clamped to 200.0%% (2 cores * 100), got %f", cpu)
	}
}
