package domain

import (
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	duration := 24 * time.Hour
	session, err := NewSession(duration)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if session.ID == "" {
		t.Error("session ID should not be empty")
	}

	if session.Token == "" {
		t.Error("session token should not be empty")
	}

	if len(session.Token) < 32 {
		t.Error("session token should be at least 32 characters")
	}

	if session.CreatedAt.IsZero() {
		t.Error("session CreatedAt should be set")
	}

	expectedExpiry := session.CreatedAt.Add(duration)
	if !session.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected ExpiresAt %v, got %v", expectedExpiry, session.ExpiresAt)
	}
}

func TestNewSessionGeneratesUniqueTokens(t *testing.T) {
	sessions := make(map[string]bool)
	for i := 0; i < 100; i++ {
		session, err := NewSession(time.Hour)
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}
		if sessions[session.Token] {
			t.Error("generated duplicate token")
		}
		sessions[session.Token] = true
	}
}

func TestSessionIsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "not expired",
			expiresAt: time.Now().UTC().Add(time.Hour),
			expected:  false,
		},
		{
			name:      "expired",
			expiresAt: time.Now().UTC().Add(-time.Hour),
			expected:  true,
		},
		{
			name:      "just expired",
			expiresAt: time.Now().UTC().Add(-time.Second),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				ExpiresAt: tt.expiresAt,
			}
			if got := session.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
