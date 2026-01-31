package ratelimit

import (
	"sync"
	"time"
)

// Limiter implements a sliding window rate limiter
type Limiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewLimiter creates a new rate limiter with the specified limit and window
func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request from the given key should be allowed
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// Clean up old entries
	requests := l.requests[key]
	valid := requests[:0]
	for _, t := range requests {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	// Check if under limit
	if len(valid) >= l.limit {
		l.requests[key] = valid
		return false
	}

	// Add new request
	l.requests[key] = append(valid, now)
	return true
}

// Remaining returns the number of remaining requests for a key
func (l *Limiter) Remaining(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	requests := l.requests[key]
	count := 0
	for _, t := range requests {
		if t.After(cutoff) {
			count++
		}
	}

	remaining := l.limit - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Reset clears all rate limit data (useful for testing)
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requests = make(map[string][]time.Time)
}

// Cleanup removes expired entries to prevent memory growth
func (l *Limiter) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	for key, requests := range l.requests {
		valid := requests[:0]
		for _, t := range requests {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(l.requests, key)
		} else {
			l.requests[key] = valid
		}
	}
}

// StartCleanup starts a background goroutine that periodically cleans up expired entries
func (l *Limiter) StartCleanup(interval time.Duration, done <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				l.Cleanup()
			case <-done:
				return
			}
		}
	}()
}
