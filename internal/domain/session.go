package domain

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"
)

// Session represents an authenticated user session
type Session struct {
	ID             string
	Token          string
	CreatedAt      time.Time
	ExpiresAt      time.Time
	LastAccessedAt time.Time
}

// Session validation errors
var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session has expired")
)

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}

// NewSession creates a new session with a cryptographically secure token
func NewSession(duration time.Duration) (*Session, error) {
	id, err := generateSecureToken(16)
	if err != nil {
		return nil, err
	}

	token, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &Session{
		ID:             id,
		Token:          token,
		CreatedAt:      now,
		ExpiresAt:      now.Add(duration),
		LastAccessedAt: now,
	}, nil
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// SessionRepository defines the interface for session persistence
type SessionRepository interface {
	// Create stores a new session
	Create(ctx context.Context, session *Session) error

	// FindByToken retrieves a session by its token
	FindByToken(ctx context.Context, token string) (*Session, error)

	// UpdateLastAccessed updates the last accessed time for a session
	UpdateLastAccessed(ctx context.Context, id string, accessedAt time.Time) error

	// Delete removes a session
	Delete(ctx context.Context, id string) error

	// DeleteExpired removes all expired sessions
	DeleteExpired(ctx context.Context) error
}
