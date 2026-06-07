package proxy

import (
	"sort"
	"strings"
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

	// waiting counts callers currently blocked in Wait for this key. It feeds
	// the observability snapshot so operators can see live queue depth.
	waiting int
	// grantedTotal / timeoutTotal are cumulative counters since process start.
	grantedTotal int64
	timeoutTotal int64
}

// BucketStat is a point-in-time view of one bucket's queue state, returned by
// Snapshot for the admin observability endpoint.
type BucketStat struct {
	Key          string  `json:"key"`
	Region       string  `json:"region"`
	AccountID    string  `json:"accountId"`
	Email        string  `json:"email,omitempty"`
	Waiting      int     `json:"waiting"`
	Tokens       float64 `json:"tokens"`
	Burst        float64 `json:"burst"`
	RPM          int     `json:"rpm"`
	GrantedTotal int64   `json:"grantedTotal"`
	TimeoutTotal int64   `json:"timeoutTotal"`
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
	granted, _, _ := rl.WaitInfo(key, rpm, maxWait)
	return granted
}

// WaitInfo is Wait with observability: it also returns how long the caller was
// queued (waited) and the queue depth observed at entry (peers, excluding self).
// The call site uses these to emit a "queued" log so operators can see live
// contention without scraping the snapshot endpoint.
func (rl *rateLimiter) WaitInfo(key string, rpm int, maxWait time.Duration) (granted bool, waited time.Duration, queueDepth int) {
	if rpm <= 0 {
		return true, 0, 0
	}

	rate := float64(rpm) / 60.0 // tokens per second
	start := time.Now()
	deadline := start.Add(maxWait)

	// Resolve/create the bucket once, then register this caller as waiting so
	// the snapshot reflects live queue depth. We deregister on every exit path.
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
	// Queue depth observed at entry = peers already waiting (excludes self).
	queueDepth = bucket.waiting
	bucket.waiting++
	rl.mu.Unlock()

	defer func() {
		rl.mu.Lock()
		bucket.waiting--
		rl.mu.Unlock()
	}()

	for {
		rl.mu.Lock()
		bucket.burst = float64(rpm)
		bucket.refillRate = rate
		now := time.Now()
		wait, ok := bucket.reserve(now)
		if ok {
			bucket.grantedTotal++
		}
		rl.mu.Unlock()

		if ok {
			return true, time.Since(start), queueDepth
		}

		// No token now; would we exceed the cap by waiting for the next one?
		failFast := false
		if maxWait <= 0 {
			failFast = true
		} else {
			remaining := time.Until(deadline)
			if remaining <= 0 || wait > remaining {
				failFast = true
			}
		}
		if failFast {
			rl.mu.Lock()
			bucket.timeoutTotal++
			rl.mu.Unlock()
			return false, time.Since(start), queueDepth
		}
		if wait <= 0 {
			wait = 10 * time.Millisecond
		}
		time.Sleep(wait)
	}
}

// Snapshot returns a point-in-time view of every active bucket, used by the
// admin observability endpoint. Buckets with no waiters and a full token count
// are omitted so the panel only surfaces keys under contention. The current
// rpm is passed in so each row shows the live setting.
func (rl *rateLimiter) Snapshot(rpm int) []BucketStat {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	out := make([]BucketStat, 0, len(rl.buckets))
	for key, b := range rl.buckets {
		b.refillLocked(now)
		// Skip idle, fully-replenished buckets with no history to keep the
		// panel focused on contention.
		if b.waiting == 0 && b.tokens >= b.burst && b.grantedTotal == 0 && b.timeoutTotal == 0 {
			continue
		}
		accountID, region := key, ""
		if i := strings.LastIndex(key, ":"); i >= 0 {
			accountID = key[:i]
			region = key[i+1:]
		}
		out = append(out, BucketStat{
			Key:          key,
			AccountID:    accountID,
			Region:       region,
			Waiting:      b.waiting,
			Tokens:       b.tokens,
			Burst:        b.burst,
			RPM:          rpm,
			GrantedTotal: b.grantedTotal,
			TimeoutTotal: b.timeoutTotal,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Waiting != out[j].Waiting {
			return out[i].Waiting > out[j].Waiting
		}
		return out[i].Key < out[j].Key
	})
	return out
}
