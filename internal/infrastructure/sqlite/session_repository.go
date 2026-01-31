package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rockstaedt/atmos/internal/domain"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (id, token, created_at, expires_at, last_accessed_at)
		VALUES (?, ?, ?, ?, ?)
	`, session.ID, session.Token, session.CreatedAt.UTC(), session.ExpiresAt.UTC(), session.LastAccessedAt.UTC())
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

func (r *SessionRepository) FindByToken(ctx context.Context, token string) (*domain.Session, error) {
	session := &domain.Session{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, token, created_at, expires_at, last_accessed_at
		FROM sessions
		WHERE token = ?
	`, token).Scan(&session.ID, &session.Token, &session.CreatedAt, &session.ExpiresAt, &session.LastAccessedAt)

	if err == sql.ErrNoRows {
		return nil, domain.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find session: %w", err)
	}

	return session, nil
}

func (r *SessionRepository) UpdateLastAccessed(ctx context.Context, id string, accessedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET last_accessed_at = ? WHERE id = ?
	`, accessedAt.UTC(), id)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrSessionNotFound
	}

	return nil
}

func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to delete expired sessions: %w", err)
	}
	return nil
}
