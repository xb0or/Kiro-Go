package proxy

import (
	"sync"
	"time"
)

// rateLimiter implements a per-key token-bucket limiter used to smooth request
// bursts against an organization's shared RPM (requests-per-minute) ceiling.
//
// idc (enterprise) accounts in the same AWS organization share a single RPM
// quota; bursting past it returns HTTP 429 and interrupts the coding session.
// Instead of firing and retrying on 429, callers wait here until a token is
// available — turning "immediate failure" into "slightly delayed success".
//
// The wait is capped: if no token frees up within maxWait the call fails fast
// so it can fall through to account/region failover rather than blocking past
// the client's request timeout.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

// tokenBucket is a classic token bucket. tokens are replenished continuously at
// refillRate tokens/second up to a capacity of burst. Access is guarded by the
// parent rateLimiter mutex (buckets are never touched without holding it).
type tokenBucket struct {
	tokens     float64
	burst      float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

var globalRateLimiter = &rateLimiter{buckets: make(map[string]*tokenBucket)}

// refillLocked advances the bucket to now, accruing tokens since lastRefill.
// Caller must hold rateLimiter.mu.
func (b *tokenBucket) refillLocked(now time.Time) {
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.lastRefill = now
}

// reserve advances the bucket and, if a whole token is available, consumes it
// and returns (0, true). Otherwise it returns the duration until the next token
// becomes available and false (nothing is consumed).
// Caller must hold rateLimiter.mu.
func (b *tokenBucket) reserve(now time.Time) (time.Duration, bool) {
	b.refillLocked(now)
	if b.tokens >= 1 {
		b.tokens -= 1
		return 0, true
	}
	if b.refillRate <= 0 {
		return 0, false
	}
	needed := 1 - b.tokens
	wait := time.Duration(needed / b.refillRate * float64(time.Second))
	return wait, false
}

// Wait blocks until a token is available for key under the given rpm budget, or
// until maxWait elapses. It returns true when a token was granted and the
// caller may proceed, false when the wait cap was hit (caller should fail fast).
//
// rpm <= 0 disables limiting (always returns true immediately), preserving the
// pre-feature behavior and giving operators a kill switch.
func (rl *rateLimiter) Wait(key string, rpm int, maxWait time.Duration) bool {
	if rpm <= 0 {
		return true
	}

	rate := float64(rpm) / 60.0 // tokens per second
	deadline := time.Now().Add(maxWait)

	for {
		rl.mu.Lock()
		bucket, ok := rl.buckets[key]
		if !ok {
			// New bucket starts full so the first request never waits.
			bucket = &tokenBucket{
				tokens:     float64(rpm),
				burst:      float64(rpm),
				refillRate: rate,
				lastRefill: time.Now(),
			}
			rl.buckets[key] = bucket
		} else {
			// Keep the bucket in sync with the current rpm setting so config
			// changes take effect without a restart.
			bucket.burst = float64(rpm)
			bucket.refillRate = rate
		}

		now := time.Now()
		wait, granted := bucket.reserve(now)
		rl.mu.Unlock()

		if granted {
			return true
		}

		// No token now; would we exceed the cap by waiting for the next one?
		if maxWait <= 0 {
			return false
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		if wait > remaining {
			return false
		}
		if wait <= 0 {
			wait = 10 * time.Millisecond
		}
		time.Sleep(wait)
	}
}
