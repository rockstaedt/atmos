package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Session represents a user session stored in the database.
type Session struct {
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// SessionRepository handles session persistence.
type SessionRepository struct {
	db *sql.DB
}

// NewSessionRepository creates a new SessionRepository.
func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create stores a new session in the database.
func (r *SessionRepository) Create(ctx context.Context, token string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (token, expires_at, created_at)
		VALUES (?, ?, ?)
	`, token, expiresAt.UTC(), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// Get retrieves a session by its token.
func (r *SessionRepository) Get(ctx context.Context, token string) (*Session, error) {
	session := &Session{}
	err := r.db.QueryRowContext(ctx, `
		SELECT token, expires_at, created_at
		FROM sessions
		WHERE token = ?
	`, token).Scan(&session.Token, &session.ExpiresAt, &session.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return session, nil
}

// Exists checks if a valid (non-expired) session exists for the given token.
func (r *SessionRepository) Exists(ctx context.Context, token string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions
		WHERE token = ? AND expires_at > ?
	`, token, time.Now().UTC()).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check session: %w", err)
	}
	return count > 0, nil
}

// Delete removes a session by its token.
func (r *SessionRepository) Delete(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM sessions WHERE token = ?
	`, token)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// DeleteExpired removes all expired sessions.
func (r *SessionRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM sessions WHERE expires_at < ?
	`, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to delete expired sessions: %w", err)
	}
	return nil
}
