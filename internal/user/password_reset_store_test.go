package user

import (
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
)

func TestMemoryPasswordResetStore(t *testing.T) {
	store := NewMemoryPasswordResetStore(50 * time.Millisecond)
	defer store.Close()

	uID := uuid.Must(uuid.NewV4())
	tokenHash1 := "hash-1"

	// 1. Store and Validate
	store.Store(tokenHash1, uID, 1*time.Minute)
	resolvedUID, err := store.Validate(tokenHash1)
	if err != nil {
		t.Fatalf("expected valid token, got %v", err)
	}
	if resolvedUID != uID {
		t.Fatalf("expected user ID %v, got %v", uID, resolvedUID)
	}

	// 2. Consume and Single-Use enforcement
	if err := store.Consume(tokenHash1); err != nil {
		t.Fatalf("unexpected error consuming token: %v", err)
	}
	_, err = store.Validate(tokenHash1)
	if err != ErrResetTokenAlreadyUsed {
		t.Fatalf("expected ErrResetTokenAlreadyUsed, got %v", err)
	}

	// 3. Unknown token
	_, err = store.Validate("non-existent-hash")
	if err != ErrResetTokenNotFound {
		t.Fatalf("expected ErrResetTokenNotFound, got %v", err)
	}

	// 4. Expired token
	tokenHashExpired := "expired-hash"
	store.Store(tokenHashExpired, uID, -1*time.Second)
	_, err = store.Validate(tokenHashExpired)
	if err != ErrResetTokenExpired {
		t.Fatalf("expected ErrResetTokenExpired, got %v", err)
	}

	// Once checked, it was purged
	_, err = store.Validate(tokenHashExpired)
	if err != ErrResetTokenNotFound {
		t.Fatalf("expected ErrResetTokenNotFound after purge, got %v", err)
	}

	// 5. Storing a new token for the same user invalidates the previous token
	tokenHashA := "hash-a"
	tokenHashB := "hash-b"
	store.Store(tokenHashA, uID, 5*time.Minute)
	store.Store(tokenHashB, uID, 5*time.Minute)

	_, err = store.Validate(tokenHashA)
	if err != ErrResetTokenNotFound {
		t.Fatalf("expected tokenHashA to be replaced by tokenHashB, got %v", err)
	}
	_, err = store.Validate(tokenHashB)
	if err != nil {
		t.Fatalf("expected tokenHashB to be valid, got %v", err)
	}

	// 6. Session Revocation
	now := time.Now()
	tokenIssuedBefore := now.Add(-5 * time.Second)
	tokenIssuedAfter := now.Add(5 * time.Second)

	if store.IsSessionRevoked(uID, tokenIssuedBefore) {
		t.Error("expected session not to be revoked prior to RevokeSessions")
	}

	store.RevokeSessions(uID)

	if !store.IsSessionRevoked(uID, tokenIssuedBefore) {
		t.Error("expected session issued before reset to be revoked")
	}
	if store.IsSessionRevoked(uID, tokenIssuedAfter) {
		t.Error("expected session issued after reset to remain valid")
	}
}
