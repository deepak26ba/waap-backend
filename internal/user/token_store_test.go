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

// TestMemoryOTPStore_MaxAttemptsInvalidatesOTPAndEnforcesCoolingPeriod tests that:
// 1. 5 failed attempts invalidates the OTP.
// 2. A 30-minute cooling period is activated, rejecting all subsequent validation attempts with ErrOTPCoolingPeriod.
func TestMemoryOTPStore_MaxAttemptsInvalidatesOTPAndEnforcesCoolingPeriod(t *testing.T) {
	store := NewMemoryOTPStore(1 * time.Minute)
	defer store.Close()

	uID := uuid.Must(uuid.NewV4())
	realOTP := "real-otp-hash"
	store.Store(uID, realOTP, 5*time.Minute)

	// Attempts 1 to 4: rejected with ErrOTPNotFound
	for i := 1; i <= 4; i++ {
		if err := store.Validate(uID, "wrong-hash"); err != ErrOTPNotFound {
			t.Fatalf("attempt %d: expected ErrOTPNotFound, got %v", i, err)
		}
	}

	// Stored OTP is still present before 5th failure
	store.mu.RLock()
	_, exists := store.otps[uID]
	store.mu.RUnlock()
	if !exists {
		t.Fatal("expected OTP to still exist before 5th failure")
	}

	// 5th attempt: exceeds maxOTPAttempts (5) -> triggers ErrOTPCoolingPeriod and invalidates OTP
	if err := store.Validate(uID, "wrong-hash"); err != ErrOTPCoolingPeriod {
		t.Fatalf("5th attempt: expected ErrOTPCoolingPeriod, got %v", err)
	}

	store.mu.RLock()
	_, exists = store.otps[uID]
	store.mu.RUnlock()
	if exists {
		t.Fatal("expected OTP entry to be purged after 5 failed attempts")
	}

	// Account is reported as locked
	if !store.IsLocked(uID) {
		t.Fatal("expected IsLocked to return true during cooling period")
	}

	// Any subsequent attempt during the cooling period returns ErrOTPCoolingPeriod
	if err := store.Validate(uID, realOTP); err != ErrOTPCoolingPeriod {
		t.Fatalf("expected ErrOTPCoolingPeriod during active cooling period, got %v", err)
	}
}

// TestMemoryOTPStore_AccountFailuresPreservedAcrossResends ensures that calling DeleteByUserID
// and Store (resend flow) cannot be used to bypass the cooling period trigger.
func TestMemoryOTPStore_AccountFailuresPreservedAcrossResends(t *testing.T) {
	store := NewMemoryOTPStore(1 * time.Minute)
	defer store.Close()

	uID := uuid.Must(uuid.NewV4())
	otp1 := "otp-1"
	otp2 := "otp-2"

	store.Store(uID, otp1, 5*time.Minute)

	// 2 failed attempts on otp1
	for i := 1; i <= 2; i++ {
		_ = store.Validate(uID, "wrong-hash")
	}

	// Resend flow: delete previous OTP and store new one
	store.DeleteByUserID(uID)
	store.Store(uID, otp2, 5*time.Minute)

	// 2 more failed attempts on otp2 (total 4)
	for i := 1; i <= 2; i++ {
		_ = store.Validate(uID, "wrong-hash")
	}

	// OTP2 still exists at 4 failures
	store.mu.RLock()
	_, exists := store.otps[uID]
	store.mu.RUnlock()
	if !exists {
		t.Fatal("expected OTP2 to still exist at 4 cumulative failures")
	}

	// 5th cumulative failed attempt triggers cooling period
	if err := store.Validate(uID, "wrong-hash"); err != ErrOTPCoolingPeriod {
		t.Fatalf("expected ErrOTPCoolingPeriod on 5th cumulative attempt, got %v", err)
	}

	store.mu.RLock()
	_, exists = store.otps[uID]
	store.mu.RUnlock()
	if exists {
		t.Fatal("expected OTP2 to be invalidated after 5 cumulative failed attempts across resends")
	}

	if !store.IsLocked(uID) {
		t.Fatal("expected user to be locked in cooling period after 5 cumulative failures")
	}
}

// TestMemoryOTPStore_CoolingPeriodExpiration verifies that after the cooling period elapses,
// the account lockout automatically clears and the user can be issued a new OTP and verify.
func TestMemoryOTPStore_CoolingPeriodExpiration(t *testing.T) {
	store := NewMemoryOTPStore(1 * time.Minute)
	defer store.Close()

	// Set short cooling period for test
	store.SetCoolingPeriod(50 * time.Millisecond)

	uID := uuid.Must(uuid.NewV4())
	store.Store(uID, "otp-hash", 5*time.Minute)

	// Trigger cooling period with 5 failed attempts
	for i := 1; i <= 4; i++ {
		_ = store.Validate(uID, "wrong-hash")
	}
	if err := store.Validate(uID, "wrong-hash"); err != ErrOTPCoolingPeriod {
		t.Fatalf("expected ErrOTPCoolingPeriod on 5th failure, got %v", err)
	}

	if !store.IsLocked(uID) {
		t.Fatal("expected user to be locked initially")
	}

	// Wait for cooling period to expire
	time.Sleep(70 * time.Millisecond)

	if store.IsLocked(uID) {
		t.Fatal("expected IsLocked to return false after cooling period expired")
	}

	// User can now receive a fresh OTP and successfully verify
	freshOTP := "fresh-otp-hash"
	store.Store(uID, freshOTP, 5*time.Minute)

	if err := store.Validate(uID, freshOTP); err != nil {
		t.Fatalf("expected successful validation after cooling period elapsed, got %v", err)
	}
}

// TestMemoryOTPStore_InformationDisclosureProtection tests CWE-204 mitigation:
// submitting an invalid hash against an already used or expired OTP returns ErrOTPNotFound,
// never revealing the internal lifecycle state of the account's OTP.
func TestMemoryOTPStore_InformationDisclosureProtection(t *testing.T) {
	store := NewMemoryOTPStore(1 * time.Minute)
	defer store.Close()

	uID := uuid.Must(uuid.NewV4())
	realOTP := "real-otp-hash"
	store.Store(uID, realOTP, 5*time.Minute)

	// Mark as consumed
	if err := store.Consume(uID); err != nil {
		t.Fatalf("failed to consume: %v", err)
	}

	// Probing with wrong hash on consumed OTP must return ErrOTPNotFound (not ErrOTPAlreadyUsed)
	if err := store.Validate(uID, "guessed-hash"); err != ErrOTPNotFound {
		t.Fatalf("expected ErrOTPNotFound for wrong hash on consumed OTP (CWE-204), got %v", err)
	}

	// Correct hash on consumed OTP returns ErrOTPAlreadyUsed
	if err := store.Validate(uID, realOTP); err != ErrOTPAlreadyUsed {
		t.Fatalf("expected ErrOTPAlreadyUsed for correct hash, got %v", err)
	}

	// Expired OTP test: wrong hash must return ErrOTPNotFound (not ErrOTPExpired)
	uIDExp := uuid.Must(uuid.NewV4())
	expOTP := "exp-otp-hash"
	store.Store(uIDExp, expOTP, -1*time.Second)

	if err := store.Validate(uIDExp, "guessed-hash"); err != ErrOTPNotFound {
		t.Fatalf("expected ErrOTPNotFound for wrong hash on expired OTP (CWE-204), got %v", err)
	}

	// Correct hash on expired OTP returns ErrOTPExpired
	if err := store.Validate(uIDExp, expOTP); err != ErrOTPExpired {
		t.Fatalf("expected ErrOTPExpired for correct hash, got %v", err)
	}
}
