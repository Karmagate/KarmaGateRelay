package main

import (
	"testing"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(10) // 10 req/sec

	// First request should be allowed
	if !rl.Allow("1.2.3.4") {
		t.Error("first request should be allowed")
	}

	// Different IP should also be allowed
	if !rl.Allow("5.6.7.8") {
		t.Error("different IP should be allowed")
	}
}

func TestRateLimiter_Burst(t *testing.T) {
	rl := NewRateLimiter(5) // 5 req/sec, burst = 10

	ip := "10.0.0.1"

	// Should allow burst
	allowed := 0
	for i := 0; i < 20; i++ {
		if rl.Allow(ip) {
			allowed++
		}
	}

	// Should allow at least the burst amount but not all
	if allowed < 5 {
		t.Errorf("expected at least 5 allowed in burst, got %d", allowed)
	}
	if allowed >= 20 {
		t.Error("rate limiter should have blocked some requests")
	}
}
