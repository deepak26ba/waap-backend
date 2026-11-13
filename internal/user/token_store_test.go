package user

import (
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
)

func TestMemoryOTPStore(t *testing.T) {
	store := NewMemoryOTPStore(100 * time.Millisecond)
	defer store.Close()

	uID1 := uuid.Must(uuid.NewV4())
	uID2 := uuid.Must(uuid.NewV4())
	sharedOTP := "shared-otp-hash"

	// 1. Two users with identical OTP hash do NOT overwrite each other
	store.Store(uID1, sharedOTP, 1*time.Minute)
	store.Store(uID2, sharedOTP, 1*time.Minute)

	if err := store.Validate(uID1, sharedOTP); err != nil {
		t.Fatalf("expected valid OTP for user 1, got %v", err)
	}
	if err := store.Validate(uID2, sharedOTP); err != nil {
		t.Fatalf("expected valid OTP for user 2, got %v", err)
	}

	// 2. Consume user 1 only; user 2 remains unaffected
	if err := store.Consume(uID1); err != nil {
		t.Fatalf("unexpected error consuming user 1 OTP: %v", err)
	}
	if err := store.Validate(uID1, sharedOTP); err != ErrOTPAlreadyUsed {
		t.Fatalf("expected ErrOTPAlreadyUsed for user 1, got %v", err)
	}
	if err := store.Validate(uID2, sharedOTP); err != nil {
		t.Fatalf("expected user 2 OTP to remain valid, got %v", err)
	}

	// 3. Unknown hash or unknown user
	if err := store.Validate(uID2, "wrong-hash"); err != ErrOTPNotFound {
		t.Fatalf("expected ErrOTPNotFound for wrong hash, got %v", err)
	}
	unknownUser := uuid.Must(uuid.NewV4())
	if err := store.Validate(unknownUser, sharedOTP); err != ErrOTPNotFound {
		t.Fatalf("expected ErrOTPNotFound for unknown user, got %v", err)
	}

	// 4. Expired OTP
	expiredUser := uuid.Must(uuid.NewV4())
	store.Store(expiredUser, "exp-hash", -1*time.Second)
	if err := store.Validate(expiredUser, "exp-hash"); err != ErrOTPExpired {
		t.Fatalf("expected ErrOTPExpired, got %v", err)
	}
	// Once checked, it was purged
	if err := store.Validate(expiredUser, "exp-hash"); err != ErrOTPNotFound {
		t.Fatalf("expected ErrOTPNotFound after purge, got %v", err)
	}

	// 5. DeleteByUserID
	store.DeleteByUserID(uID2)
	if err := store.Validate(uID2, sharedOTP); err != ErrOTPNotFound {
		t.Fatalf("expected ErrOTPNotFound after DeleteByUserID, got %v", err)
	}

	// 6. Automatic background cleanup
	cleanupUser := uuid.Must(uuid.NewV4())
	store.Store(cleanupUser, "cleanup-hash", 50*time.Millisecond)
	time.Sleep(150 * time.Millisecond)

	store.mu.RLock()
	_, exists := store.otps[cleanupUser]
	store.mu.RUnlock()
	if exists {
		t.Error("expected expired OTP to be cleaned up from memory by background cleaner")
	}
}
