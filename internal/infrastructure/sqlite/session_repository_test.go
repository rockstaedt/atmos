package sqlite

import (
	"context"
	"testing"
	"time"
)

func TestSessionRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewSessionRepository(db)
	ctx := context.Background()

	token := "test-token-123"
	expiresAt := time.Now().Add(24 * time.Hour).UTC()

	err := repo.Create(ctx, token, expiresAt)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify session was created
	session, err := repo.Get(ctx, token)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	switch {
	case session == nil:
		t.Fatal("Expected session to be created")
	case session.Token != token:
		t.Errorf("Expected token %q, got %q", token, session.Token)
	case session.ExpiresAt.Unix() != expiresAt.Unix():
		t.Errorf("Expected expires_at %v, got %v", expiresAt, session.ExpiresAt)
	}
}

func TestSessionRepository_Get_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewSessionRepository(db)
	ctx := context.Background()

	session, err := repo.Get(ctx, "non-existent-token")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if session != nil {
		t.Error("Expected nil session for non-existent token")
	}
}

func TestSessionRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewSessionRepository(db)
	ctx := context.Background()

	token := "test-token-456"
	expiresAt := time.Now().Add(24 * time.Hour).UTC()

	err := repo.Create(ctx, token, expiresAt)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = repo.Delete(ctx, token)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify session was deleted
	session, err := repo.Get(ctx, token)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if session != nil {
		t.Error("Expected session to be deleted")
	}
}

func TestSessionRepository_DeleteExpired(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewSessionRepository(db)
	ctx := context.Background()

	// Create an expired session
	expiredToken := "expired-token"
	expiredAt := time.Now().Add(-1 * time.Hour).UTC()
	err := repo.Create(ctx, expiredToken, expiredAt)
	if err != nil {
		t.Fatalf("Create expired session failed: %v", err)
	}

	// Create a valid session
	validToken := "valid-token"
	validAt := time.Now().Add(24 * time.Hour).UTC()
	err = repo.Create(ctx, validToken, validAt)
	if err != nil {
		t.Fatalf("Create valid session failed: %v", err)
	}

	// Delete expired sessions
	err = repo.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired failed: %v", err)
	}

	// Verify expired session was deleted
	expiredSession, err := repo.Get(ctx, expiredToken)
	if err != nil {
		t.Fatalf("Get expired session failed: %v", err)
	}
	if expiredSession != nil {
		t.Error("Expected expired session to be deleted")
	}

	// Verify valid session still exists
	validSession, err := repo.Get(ctx, validToken)
	if err != nil {
		t.Fatalf("Get valid session failed: %v", err)
	}
	if validSession == nil {
		t.Error("Expected valid session to still exist")
	}
}

func TestSessionRepository_Exists(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewSessionRepository(db)
	ctx := context.Background()

	// Create a valid session
	validToken := "valid-exists-token"
	validAt := time.Now().Add(24 * time.Hour).UTC()
	err := repo.Create(ctx, validToken, validAt)
	if err != nil {
		t.Fatalf("Create valid session failed: %v", err)
	}

	// Create an expired session
	expiredToken := "expired-exists-token"
	expiredAt := time.Now().Add(-1 * time.Hour).UTC()
	err = repo.Create(ctx, expiredToken, expiredAt)
	if err != nil {
		t.Fatalf("Create expired session failed: %v", err)
	}

	t.Run("valid session exists", func(t *testing.T) {
		exists, err := repo.Exists(ctx, validToken)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("Expected valid session to exist")
		}
	})

	t.Run("expired session does not exist", func(t *testing.T) {
		exists, err := repo.Exists(ctx, expiredToken)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("Expected expired session to not exist")
		}
	})

	t.Run("non-existent token does not exist", func(t *testing.T) {
		exists, err := repo.Exists(ctx, "non-existent-token")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("Expected non-existent token to not exist")
		}
	})
}
