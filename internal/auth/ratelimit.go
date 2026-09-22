package auth

import (
	"sync"
	"time"
)

// RateLimitResult conveys rate limiting enforcement state.
type RateLimitResult struct {
	Blocked           bool
	RetryAfterSeconds int
	Level             int // 0 = none, 1 = per-IP, 2 = distributed circuit breaker
	Reason            string
}

// RateLimiter manages Level 1 per-IP rate limiting and Level 2 distributed circuit breaker.
type RateLimiter struct {
	mu sync.RWMutex

	// Level 1: Per-IP tracking
	perIPAttempts map[string][]time.Time
	perIPBlock    map[string]time.Time // blocked until
	maxAttempts   int
	window        time.Duration

	// Level 2: Distributed multi-IP tracking
	failedIPs          map[string]time.Time // ip -> last failed time
	circuitBreakerTrip time.Time            // lockout until
	distThreshold      int
	distWindow         time.Duration
	distLockout        time.Duration

	// Named user targeting: username -> map[ip]time.Time
	userFailedIPs map[string]map[string]time.Time
}

// NewRateLimiter initializes the two-tier rate limiter.
// Recommended defaults: maxAttempts=5, window=5m, distThreshold=10, distWindow=5m, distLockout=15m.
func NewRateLimiter(maxAttempts int, window time.Duration, distThreshold int, distWindow, distLockout time.Duration) *RateLimiter {
	return &RateLimiter{
		perIPAttempts:      make(map[string][]time.Time),
		perIPBlock:         make(map[string]time.Time),
		maxAttempts:        maxAttempts,
		window:             window,
		failedIPs:          make(map[string]time.Time),
		distThreshold:      distThreshold,
		distWindow:         distWindow,
		distLockout:        distLockout,
		userFailedIPs:      make(map[string]map[string]time.Time),
	}
}

// CheckKnockAttempt evaluates whether port-knock access is currently allowed for this IP.
func (rl *RateLimiter) CheckKnockAttempt(ip string, now time.Time) RateLimitResult {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Level 2: Circuit Breaker check
	if now.Before(rl.circuitBreakerTrip) {
		remaining := int(rl.circuitBreakerTrip.Sub(now).Seconds())
		if remaining < 1 {
			remaining = 1
		}
		return RateLimitResult{
			Blocked:           true,
			RetryAfterSeconds: remaining,
			Level:             2,
			Reason:            "Distributed brute-force defense circuit breaker active. New port authorizations paused.",
		}
	}

	// Level 1: Per-IP temporary block check
	if blockedUntil, exists := rl.perIPBlock[ip]; exists {
		if now.Before(blockedUntil) {
			remaining := int(blockedUntil.Sub(now).Seconds())
			if remaining < 1 {
				remaining = 1
			}
			return RateLimitResult{
				Blocked:           true,
				RetryAfterSeconds: remaining,
				Level:             1,
				Reason:            "Too many failed access attempts from your IP address. Please wait before retrying.",
			}
		}
		delete(rl.perIPBlock, ip)
	}

	return RateLimitResult{Blocked: false, Level: 0}
}

// RecordKnockFailure logs a failed port-knock attempt.
// Returns true if circuit breaker was newly tripped.
func (rl *RateLimiter) RecordKnockFailure(ip string, now time.Time) (circuitBreakerTripped bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Level 1 update
	attempts := rl.cleanAttempts(rl.perIPAttempts[ip], now, rl.window)
	attempts = append(attempts, now)
	rl.perIPAttempts[ip] = attempts

	if len(attempts) >= rl.maxAttempts {
		excess := len(attempts) - rl.maxAttempts
		// Progressive backoff: 30s * 2^excess capped at 300s (5m)
		delaySeconds := 30 * (1 << excess)
		if delaySeconds > 300 {
			delaySeconds = 300
		}
		rl.perIPBlock[ip] = now.Add(time.Duration(delaySeconds) * time.Second)
	}

	// Level 2 update
	rl.cleanDistributedIPs(now)
	rl.failedIPs[ip] = now

	if len(rl.failedIPs) >= rl.distThreshold {
		if now.After(rl.circuitBreakerTrip) {
			rl.circuitBreakerTrip = now.Add(rl.distLockout)
			circuitBreakerTripped = true
		}
	}

	return circuitBreakerTripped
}

// RecordKnockSuccess clears per-IP failure tracking for that visitor.
func (rl *RateLimiter) RecordKnockSuccess(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.perIPAttempts, ip)
	delete(rl.perIPBlock, ip)
}

// CheckLogin evaluates rate limiting for manager login attempts.
func (rl *RateLimiter) CheckLogin(ip string, now time.Time) RateLimitResult {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if blockedUntil, exists := rl.perIPBlock[ip]; exists {
		if now.Before(blockedUntil) {
			remaining := int(blockedUntil.Sub(now).Seconds())
			if remaining < 1 {
				remaining = 1
			}
			return RateLimitResult{
				Blocked:           true,
				RetryAfterSeconds: remaining,
				Level:             1,
				Reason:            "Too many failed login attempts from your IP. Temporary backoff applied.",
			}
		}
		delete(rl.perIPBlock, ip)
	}

	return RateLimitResult{Blocked: false, Level: 0}
}

// RecordLoginFailure records a manager login failure.
func (rl *RateLimiter) RecordLoginFailure(ip, username string, now time.Time) (circuitBreakerTripped bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Level 1 update
	attempts := rl.cleanAttempts(rl.perIPAttempts[ip], now, rl.window)
	attempts = append(attempts, now)
	rl.perIPAttempts[ip] = attempts

	if len(attempts) >= rl.maxAttempts {
		excess := len(attempts) - rl.maxAttempts
		delaySeconds := 30 * (1 << excess)
		if delaySeconds > 300 {
			delaySeconds = 300
		}
		rl.perIPBlock[ip] = now.Add(time.Duration(delaySeconds) * time.Second)
	}

	// User-targeted distributed attack tracking
	if username != "" {
		if _, exists := rl.userFailedIPs[username]; !exists {
			rl.userFailedIPs[username] = make(map[string]time.Time)
		}
		rl.userFailedIPs[username][ip] = now

		for userIP, t := range rl.userFailedIPs[username] {
			if now.Sub(t) > rl.distWindow {
				delete(rl.userFailedIPs[username], userIP)
			}
		}
	}

	return false
}

// ResetCircuitBreaker clears active Level 2 lockdown.
func (rl *RateLimiter) ResetCircuitBreaker() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.circuitBreakerTrip = time.Time{}
	rl.failedIPs = make(map[string]time.Time)
	rl.userFailedIPs = make(map[string]map[string]time.Time)
}

// IsCircuitBreakerActive returns whether Level 2 circuit breaker is currently active.
func (rl *RateLimiter) IsCircuitBreakerActive(now time.Time) (bool, time.Duration) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if now.Before(rl.circuitBreakerTrip) {
		return true, rl.circuitBreakerTrip.Sub(now)
	}
	return false, 0
}

func (rl *RateLimiter) cleanAttempts(attempts []time.Time, now time.Time, window time.Duration) []time.Time {
	var valid []time.Time
	for _, t := range attempts {
		if now.Sub(t) <= window {
			valid = append(valid, t)
		}
	}
	return valid
}

func (rl *RateLimiter) cleanDistributedIPs(now time.Time) {
	for ip, t := range rl.failedIPs {
		if now.Sub(t) > rl.distWindow {
			delete(rl.failedIPs, ip)
		}
	}
}
