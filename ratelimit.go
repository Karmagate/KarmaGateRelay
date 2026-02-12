package main

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiterEntry
	rate     float64
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(rps float64) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		rate:     rps,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	entry, ok := rl.limiters[ip]
	if !ok {
		entry = &rateLimiterEntry{
			limiter: rate.NewLimiter(rate.Limit(rl.rate), int(rl.rate)*2),
		}
		rl.limiters[ip] = entry
	}
	entry.lastSeen = time.Now()
	rl.mu.Unlock()

	return entry.limiter.Allow()
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for ip, entry := range rl.limiters {
			if entry.lastSeen.Before(cutoff) {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}
