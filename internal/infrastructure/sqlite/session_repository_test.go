package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/rockstaedt/atmos/internal/domain"
	_ "modernc.org/sqlite"
)

func setupSessionTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db
}

func TestSessionRepository_Create(t *testing.T) {
	db := setupSessionTestDB(t)
	defer db.Close()

	repo := NewSessionRepository(db)
	ctx := context.Background()

	session, err := domain.NewSession(24 * time.Hour)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	// Verify session was saved
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", session.ID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query sessions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 session, got %d", count)
	}
}

func TestSessionRepository_FindByToken(t *testing.T) {
	db := setupSessionTestDB(t)
	defer db.Close()

	repo := NewSessionRepository(db)
	ctx := context.Background()

	session, _ := domain.NewSession(24 * time.Hour)
	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	t.Run("find existing session", func(t *testing.T) {
		found, err := repo.FindByToken(ctx, session.Token)
		if err != nil {
			t.Fatalf("failed to find session: %v", err)
		}
		if found.ID != session.ID {
			t.Errorf("expected session ID %s, got %s", session.ID, found.ID)
		}
	})

	t.Run("session not found", func(t *testing.T) {
		_, err := repo.FindByToken(ctx, "nonexistent-token")
		if !errors.Is(err, domain.ErrSessionNotFound) {
			t.Errorf("expected ErrSessionNotFound, got %v", err)
		}
	})
}

func TestSessionRepository_UpdateLastAccessed(t *testing.T) {
	db := setupSessionTestDB(t)
	defer db.Close()

	repo := NewSessionRepository(db)
	ctx := context.Background()

	session, _ := domain.NewSession(24 * time.Hour)
	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	newTime := time.Now().UTC().Add(time.Hour)
	if err := repo.UpdateLastAccessed(ctx, session.ID, newTime); err != nil {
		t.Fatalf("failed to update last accessed: %v", err)
	}

	// Verify update
	found, err := repo.FindByToken(ctx, session.Token)
	if err != nil {
		t.Fatalf("failed to find session: %v", err)
	}
	if found.LastAccessedAt.Before(session.LastAccessedAt) {
		t.Error("last accessed time should have been updated")
	}
}

func TestSessionRepository_Delete(t *testing.T) {
	db := setupSessionTestDB(t)
	defer db.Close()

	repo := NewSessionRepository(db)
	ctx := context.Background()

	session, _ := domain.NewSession(24 * time.Hour)
	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	if err := repo.Delete(ctx, session.ID); err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}

	_, err := repo.FindByToken(ctx, session.Token)
	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound after delete, got %v", err)
	}
}

func TestSessionRepository_DeleteExpired(t *testing.T) {
	db := setupSessionTestDB(t)
	defer db.Close()

	repo := NewSessionRepository(db)
	ctx := context.Background()

	// Create an expired session
	expiredSession := &domain.Session{
		ID:             "expired-id",
		Token:          "expired-token",
		CreatedAt:      time.Now().UTC().Add(-48 * time.Hour),
		ExpiresAt:      time.Now().UTC().Add(-24 * time.Hour),
		LastAccessedAt: time.Now().UTC().Add(-48 * time.Hour),
	}
	if err := repo.Create(ctx, expiredSession); err != nil {
		t.Fatalf("failed to save expired session: %v", err)
	}

	// Create a valid session
	validSession, _ := domain.NewSession(24 * time.Hour)
	if err := repo.Create(ctx, validSession); err != nil {
		t.Fatalf("failed to save valid session: %v", err)
	}

	if err := repo.DeleteExpired(ctx); err != nil {
		t.Fatalf("failed to delete expired sessions: %v", err)
	}

	// Expired session should be gone
	_, err := repo.FindByToken(ctx, expiredSession.Token)
	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Error("expired session should have been deleted")
	}

	// Valid session should still exist
	_, err = repo.FindByToken(ctx, validSession.Token)
	if err != nil {
		t.Error("valid session should still exist")
	}
}
