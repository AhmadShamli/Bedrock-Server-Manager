package engine

import (
	"sync"
	"time"
)

// CrashCircuitBreaker tracks unexpected server crashes to prevent infinite crash-loops.
type CrashCircuitBreaker struct {
	mu           sync.Mutex
	maxCrashes   int
	window       time.Duration
	crashHistory map[string][]time.Time
	tripped      map[string]bool
}

// NewCrashCircuitBreaker creates a circuit breaker (default: 5 crashes in 5 minutes).
func NewCrashCircuitBreaker(maxCrashes int, window time.Duration) *CrashCircuitBreaker {
	if maxCrashes <= 0 {
		maxCrashes = 5
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &CrashCircuitBreaker{
		maxCrashes:   maxCrashes,
		window:       window,
		crashHistory: make(map[string][]time.Time),
		tripped:      make(map[string]bool),
	}
}

// RecordCrash logs an unexpected server crash and checks if the circuit breaker trips.
func (cb *CrashCircuitBreaker) RecordCrash(serverID string, now time.Time) (tripped bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	var valid []time.Time
	for _, t := range cb.crashHistory[serverID] {
		if now.Sub(t) <= cb.window {
			valid = append(valid, t)
		}
	}
	valid = append(valid, now)
	cb.crashHistory[serverID] = valid

	if len(valid) >= cb.maxCrashes {
		cb.tripped[serverID] = true
		return true
	}

	return cb.tripped[serverID]
}

// IsTripped checks if the circuit breaker is active for the given server.
func (cb *CrashCircuitBreaker) IsTripped(serverID string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.tripped[serverID]
}

// Reset clears the crash history and untrips the circuit breaker for a server.
func (cb *CrashCircuitBreaker) Reset(serverID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.crashHistory, serverID)
	delete(cb.tripped, serverID)
}
