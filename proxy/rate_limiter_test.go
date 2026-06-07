package proxy

import (
	"sync"
	"testing"
	"time"
)

// rpm<=0 disables limiting: every call returns immediately and is granted.
func TestRateLimiterDisabledWhenRPMZero(t *testing.T) {
	rl := &rateLimiter{buckets: make(map[string]*tokenBucket)}
	for i := 0; i < 100; i++ {
		if !rl.Wait("k", 0, time.Second) {
			t.Fatalf("rpm=0 must always grant; call %d was denied", i)
		}
	}
}

// A fresh bucket starts full, so the first burst up to rpm is granted instantly.
func TestRateLimiterFirstBurstIsImmediate(t *testing.T) {
	rl := &rateLimiter{buckets: make(map[string]*tokenBucket)}
	start := time.Now()
	for i := 0; i < 10; i++ {
		if !rl.Wait("k", 10, time.Second) {
			t.Fatalf("burst within capacity must be granted; call %d denied", i)
		}
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("initial burst should be near-instant, took %v", elapsed)
	}
}

// Once the bucket is drained, the next call with a tiny wait cap fails fast
// rather than blocking — this is what lets the caller fall through to failover.
func TestRateLimiterFailsFastPastCap(t *testing.T) {
	rl := &rateLimiter{buckets: make(map[string]*tokenBucket)}
	// Drain the full bucket (rpm=2 → capacity 2).
	if !rl.Wait("k", 2, 0) {
		t.Fatal("first token should be granted")
	}
	if !rl.Wait("k", 2, 0) {
		t.Fatal("second token should be granted")
	}
	// Bucket empty now; refill is 2/60 per sec → next token ~30s away.
	// With a 10ms cap we must fail fast, not wait.
	start := time.Now()
	if rl.Wait("k", 2, 10*time.Millisecond) {
		t.Fatal("drained bucket past cap must fail fast")
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("fail-fast should not block, took %v", elapsed)
	}
}

// A drained bucket grants the next request once enough time passes that a
// token refills, as long as the wait cap covers the refill interval.
func TestRateLimiterGrantsAfterRefill(t *testing.T) {
	rl := &rateLimiter{buckets: make(map[string]*tokenBucket)}
	// rpm=600 → 10 tokens/sec → 1 token every 100ms.
	for i := 0; i < 600; i++ {
		rl.Wait("k", 600, 0) // drain
	}
	// Next token ~100ms out; a 2s cap easily covers it.
	start := time.Now()
	if !rl.Wait("k", 600, 2*time.Second) {
		t.Fatal("should grant after a short refill wait")
	}
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Fatalf("expected to wait for refill, returned too fast: %v", elapsed)
	}
	if elapsed > 1*time.Second {
		t.Fatalf("refill wait far longer than expected: %v", elapsed)
	}
}

// Different keys hold independent buckets — draining one must not affect another.
func TestRateLimiterKeysAreIndependent(t *testing.T) {
	rl := &rateLimiter{buckets: make(map[string]*tokenBucket)}
	if !rl.Wait("a", 1, 0) {
		t.Fatal("key a first token should be granted")
	}
	// a is now drained, but b is untouched.
	if !rl.Wait("b", 1, 0) {
		t.Fatal("key b must have its own full bucket")
	}
}

// Concurrent callers on one key must never over-grant beyond the bucket
// capacity in a tight window (mutex correctness / no token double-spend).
func TestRateLimiterConcurrentNoOverGrant(t *testing.T) {
	rl := &rateLimiter{buckets: make(map[string]*tokenBucket)}
	const rpm = 20
	var granted int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 0 cap → only succeeds if a token is immediately available.
			if rl.Wait("k", rpm, 0) {
				mu.Lock()
				granted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	// Capacity is rpm; with a near-instant window no more than a couple of
	// refilled tokens can appear, so granted must stay close to capacity.
	if granted > rpm+2 {
		t.Fatalf("over-granted: got %d, capacity %d", granted, rpm)
	}
	if granted < 1 {
		t.Fatal("expected at least the initial bucket to grant some tokens")
	}
}
