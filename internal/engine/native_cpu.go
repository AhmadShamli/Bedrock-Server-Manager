package engine

import (
	"math"
	"runtime"
	"time"
)

// nativeCPUSample preserves kernel/user tick baselines for Windows child processes.
type nativeCPUSample struct {
	processID      int
	timestamp      time.Time
	kernelTicks    uint64
	userTicks      uint64
	lastCPUPercent float64
}

// computeNativeProcessCPUPercent calculates real-time CPU percentage for a native process
// based on Windows GetProcessTimes kernel and user tick deltas across time intervals.
func computeNativeProcessCPUPercent(
	procID int,
	curKernel, curUser uint64,
	now time.Time,
	prev nativeCPUSample,
	hasPrev bool,
	numCPU int,
) (float64, nativeCPUSample, bool) {
	if numCPU <= 0 {
		numCPU = runtime.NumCPU()
	}
	if numCPU <= 0 {
		numCPU = 1
	}

	// 1. If no previous sample exists or the sample belongs to an older/different process PID,
	// establish this as the baseline sample and report 0% CPU for the initial interval.
	if !hasPrev || prev.processID != procID {
		initial := nativeCPUSample{
			processID:      procID,
			timestamp:      now,
			kernelTicks:    curKernel,
			userTicks:      curUser,
			lastCPUPercent: 0.0,
		}
		return 0.0, initial, true
	}

	// 2. Throttle excessively frequent samples (e.g. concurrent HTTP API and background telemetry pollers)
	elapsed := now.Sub(prev.timestamp)
	if elapsed < 250*time.Millisecond {
		// Return last computed percent and keep original baseline sample
		return prev.lastCPUPercent, prev, false
	}

	// 3. Compute CPU delta with strict underflow protection
	var cpuPercent float64
	if curKernel >= prev.kernelTicks && curUser >= prev.userTicks {
		totalProcessTicksDelta := (curKernel - prev.kernelTicks) + (curUser - prev.userTicks)
		// Windows FILETIME ticks are 100ns units: 1 tick = 100ns
		processNs := float64(totalProcessTicksDelta) * 100.0
		elapsedNs := float64(elapsed.Nanoseconds())
		if elapsedNs > 0 {
			cpuPercent = (processNs / elapsedNs) * 100.0
		}
	}

	// 4. Sanitize and clamp within physically possible bounds
	maxValidPercent := float64(numCPU) * 100.0
	if math.IsNaN(cpuPercent) || math.IsInf(cpuPercent, 0) || cpuPercent < 0.0 {
		cpuPercent = 0.0
	} else if cpuPercent > maxValidPercent {
		cpuPercent = maxValidPercent
	}

	nextSample := nativeCPUSample{
		processID:      procID,
		timestamp:      now,
		kernelTicks:    curKernel,
		userTicks:      curUser,
		lastCPUPercent: cpuPercent,
	}

	return cpuPercent, nextSample, true
}
