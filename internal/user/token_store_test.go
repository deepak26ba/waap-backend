package user

import (
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
)

func TestMemoryTokenStore(t *testing.T) {
	store := NewMemoryTokenStore(100 * time.Millisecond)
	defer store.Close()

	uID := uuid.Must(uuid.NewV4())
	tokHash := "test-hash-123"

	// 1. Store and consume
	store.Store(tokHash, uID, 1*time.Minute)

	gotID, err := store.ValidateAndConsume(tokHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotID != uID {
		t.Fatalf("expected user ID %s, got %s", uID, gotID)
	}

	// 2. Token reuse
	_, err = store.ValidateAndConsume(tokHash)
	if err != ErrTokenAlreadyUsed {
		t.Fatalf("expected ErrTokenAlreadyUsed, got %v", err)
	}

	// 3. Unknown token
	_, err = store.ValidateAndConsume("non-existent-hash")
	if err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}

	// 4. Expired token upon access
	expiredHash := "expired-hash-456"
	store.Store(expiredHash, uID, -1*time.Second)

	_, err = store.ValidateAndConsume(expiredHash)
	if err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}

	// Confirm it was deleted on access
	_, err = store.ValidateAndConsume(expiredHash)
	if err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound after expired token purged, got %v", err)
	}

	// 5. Automatic background cleanup
	cleanupHash := "cleanup-hash-789"
	store.Store(cleanupHash, uID, 50*time.Millisecond)

	time.Sleep(150 * time.Millisecond)

	store.mu.RLock()
	_, exists := store.otps[cleanupHash]
	store.mu.RUnlock()

	if exists {
		t.Error("expected expired token to be cleaned up from memory by background cleaner")
	}

	// 6. Manual delete
	delHash := "delete-hash"
	store.Store(delHash, uID, 1*time.Minute)
	store.Delete(delHash)
	_, err = store.ValidateAndConsume(delHash)
	if err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound after manual delete, got %v", err)
	}

	// 7. DeleteByUserID
	userTok1 := "user-tok-1"
	userTok2 := "user-tok-2"
	store.Store(userTok1, uID, 1*time.Minute)
	store.Store(userTok2, uID, 1*time.Minute)
	store.DeleteByUserID(uID)

	if _, err := store.ValidateAndConsume(userTok1); err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound for userTok1, got %v", err)
	}
	if _, err := store.ValidateAndConsume(userTok2); err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound for userTok2, got %v", err)
	}
}
