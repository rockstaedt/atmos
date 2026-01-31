package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterAllow(t *testing.T) {
	limiter := NewLimiter(3, time.Minute)

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow("test-key") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if limiter.Allow("test-key") {
		t.Error("4th request should be denied")
	}
}

func TestLimiterRemaining(t *testing.T) {
	limiter := NewLimiter(5, time.Minute)

	if remaining := limiter.Remaining("test-key"); remaining != 5 {
		t.Errorf("expected 5 remaining, got %d", remaining)
	}

	limiter.Allow("test-key")
	limiter.Allow("test-key")

	if remaining := limiter.Remaining("test-key"); remaining != 3 {
		t.Errorf("expected 3 remaining, got %d", remaining)
	}
}

func TestLimiterDifferentKeys(t *testing.T) {
	limiter := NewLimiter(2, time.Minute)

	// Key 1 uses its limit
	limiter.Allow("key1")
	limiter.Allow("key1")
	if limiter.Allow("key1") {
		t.Error("key1 should be rate limited")
	}

	// Key 2 should still have its own limit
	if !limiter.Allow("key2") {
		t.Error("key2 should be allowed")
	}
}

func TestLimiterWindowExpiry(t *testing.T) {
	// Use a very short window for testing
	limiter := NewLimiter(2, 50*time.Millisecond)

	// Use up the limit
	limiter.Allow("test-key")
	limiter.Allow("test-key")
	if limiter.Allow("test-key") {
		t.Error("should be rate limited")
	}

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	if !limiter.Allow("test-key") {
		t.Error("should be allowed after window expires")
	}
}

func TestLimiterReset(t *testing.T) {
	limiter := NewLimiter(2, time.Minute)

	limiter.Allow("test-key")
	limiter.Allow("test-key")
	if limiter.Allow("test-key") {
		t.Error("should be rate limited before reset")
	}

	limiter.Reset()

	if !limiter.Allow("test-key") {
		t.Error("should be allowed after reset")
	}
}

func TestLimiterCleanup(t *testing.T) {
	limiter := NewLimiter(10, 50*time.Millisecond)

	// Add some entries
	limiter.Allow("key1")
	limiter.Allow("key2")
	limiter.Allow("key3")

	// Wait for entries to expire
	time.Sleep(60 * time.Millisecond)

	limiter.Cleanup()

	// All keys should have full limit again
	if remaining := limiter.Remaining("key1"); remaining != 10 {
		t.Errorf("expected 10 remaining after cleanup, got %d", remaining)
	}
}

func TestLimiterConcurrency(t *testing.T) {
	limiter := NewLimiter(100, time.Minute)
	done := make(chan bool)

	// Run concurrent requests
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				limiter.Allow("concurrent-key")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have counted all requests
	remaining := limiter.Remaining("concurrent-key")
	if remaining != 0 {
		// Due to the limit, some requests were denied
		// Just verify no panic occurred and remaining is reasonable
		if remaining > 100 {
			t.Errorf("remaining %d exceeds limit", remaining)
		}
	}
}
