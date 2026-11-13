package user

import (
	"errors"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
)

// maxOTPAttempts is the maximum number of failed verification attempts permitted per account before entering a cooling period.
const maxOTPAttempts = 5

// DefaultCoolingPeriod is the default lockout duration after exceeding failed verification attempts.
const DefaultCoolingPeriod = 30 * time.Minute

var (
	ErrOTPNotFound      = errors.New("otp not found")
	ErrOTPExpired       = errors.New("otp expired")
	ErrOTPAlreadyUsed   = errors.New("otp already used")
	ErrOTPCoolingPeriod = errors.New("cooling period active: too many failed attempts, please try again after 30 minutes")

	// Backwards-compatibility aliases
	ErrTokenNotFound    = ErrOTPNotFound
	ErrTokenExpired     = ErrOTPExpired
	ErrTokenAlreadyUsed = ErrOTPAlreadyUsed
)

type otpItem struct {
	otpHash   string
	expiresAt time.Time
	used      bool
}

// OTPStore defines the contract for storing, validating, and consuming verification OTPs in memory,
// scoped by user identity to prevent collision between identical OTP codes across accounts.
type OTPStore interface {
	Store(userID uuid.UUID, otpHash string, ttl time.Duration)
	Validate(userID uuid.UUID, otpHash string) error
	Consume(userID uuid.UUID) error
	DeleteByUserID(userID uuid.UUID)
	IsLocked(userID uuid.UUID) bool
	Close()
}

// TokenStore is an alias for OTPStore.
type TokenStore = OTPStore

// MemoryOTPStore is a thread-safe in-memory store for verification OTPs keyed by user ID.
type MemoryOTPStore struct {
	mu                    sync.RWMutex
	otps                  map[uuid.UUID]otpItem
	accountFailures       map[uuid.UUID]int
	lockedUntil           map[uuid.UUID]time.Time
	coolingPeriodDuration time.Duration
	stopCh                chan struct{}
}

// NewMemoryOTPStore constructs a MemoryOTPStore and starts background expiration cleanup.
func NewMemoryOTPStore(cleanupInterval time.Duration) *MemoryOTPStore {
	if cleanupInterval <= 0 {
		cleanupInterval = 1 * time.Minute
	}
	s := &MemoryOTPStore{
		otps:                  make(map[uuid.UUID]otpItem),
		accountFailures:       make(map[uuid.UUID]int),
		lockedUntil:           make(map[uuid.UUID]time.Time),
		coolingPeriodDuration: DefaultCoolingPeriod,
		stopCh:                make(chan struct{}),
	}

	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.cleanupExpired()
			case <-s.stopCh:
				return
			}
		}
	}()

	return s
}

// NewMemoryTokenStore is an alias for NewMemoryOTPStore.
func NewMemoryTokenStore(cleanupInterval time.Duration) *MemoryOTPStore {
	return NewMemoryOTPStore(cleanupInterval)
}

// SetCoolingPeriod allows configuring the cooling period duration (useful in tests).
func (s *MemoryOTPStore) SetCoolingPeriod(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.coolingPeriodDuration = d
}

// Store records an OTP hash for a specific user ID with expiration duration.
func (s *MemoryOTPStore) Store(userID uuid.UUID, otpHash string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.otps[userID] = otpItem{
		otpHash:   otpHash,
		expiresAt: time.Now().Add(ttl),
		used:      false,
	}
}

// Validate checks if the user has an active, unconsumed OTP matching the provided hash without consuming it.
// It verifies the OTP hash first (CWE-204 protection) and enforces a 30-minute cooling period on 5 failed attempts (CWE-307 protection).
func (s *MemoryOTPStore) Validate(userID uuid.UUID, otpHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Check if user is currently in a 30-minute cooling period
	if lockUntil, locked := s.lockedUntil[userID]; locked {
		if time.Now().Before(lockUntil) {
			return ErrOTPCoolingPeriod
		}
		// Cooling period expired: reset lock and failure count
		delete(s.lockedUntil, userID)
		delete(s.accountFailures, userID)
	}

	item, exists := s.otps[userID]
	if !exists {
		return ErrOTPNotFound
	}

	// 2. Verify hash first: wrong OTP guesses increment failure count (CWE-307)
	// and never reveal whether the token is already used or expired (CWE-204).
	if item.otpHash != otpHash {
		s.accountFailures[userID]++
		if s.accountFailures[userID] >= maxOTPAttempts {
			delete(s.otps, userID)
			s.lockedUntil[userID] = time.Now().Add(s.coolingPeriodDuration)
			return ErrOTPCoolingPeriod
		}
		return ErrOTPNotFound
	}

	// 3. Hash matches: check if already consumed
	if item.used {
		return ErrOTPAlreadyUsed
	}

	// 4. Hash matches: check expiration
	if time.Now().After(item.expiresAt) {
		delete(s.otps, userID)
		delete(s.accountFailures, userID)
		return ErrOTPExpired
	}

	return nil
}

// Consume marks the user's OTP as consumed, enforcing single-use and clearing failure count.
func (s *MemoryOTPStore) Consume(userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.otps[userID]
	if !exists {
		return ErrOTPNotFound
	}

	item.used = true
	s.otps[userID] = item
	delete(s.accountFailures, userID)
	delete(s.lockedUntil, userID)
	return nil
}

// IsLocked reports whether the account is currently locked out in a cooling period.
func (s *MemoryOTPStore) IsLocked(userID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockUntil, locked := s.lockedUntil[userID]
	if !locked {
		return false
	}

	if time.Now().Before(lockUntil) {
		return true
	}

	// Cooling period has expired: clear lock and failures
	delete(s.lockedUntil, userID)
	delete(s.accountFailures, userID)
	return false
}

// DeleteByUserID purges the active OTP associated with a given user ID, preserving account failures and locks across resends.
func (s *MemoryOTPStore) DeleteByUserID(userID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.otps, userID)
}

// cleanupExpired removes all OTPs whose expiration timestamp has passed and cleans up orphaned failure/lock counts.
func (s *MemoryOTPStore) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.otps {
		if now.After(v.expiresAt) {
			delete(s.otps, k)
			if lockUntil, locked := s.lockedUntil[k]; !locked || now.After(lockUntil) {
				delete(s.accountFailures, k)
				delete(s.lockedUntil, k)
			}
		}
	}
	for k, lockUntil := range s.lockedUntil {
		if now.After(lockUntil) {
			delete(s.lockedUntil, k)
			delete(s.accountFailures, k)
		}
	}
}

// Close stops the periodic cleanup goroutine.
func (s *MemoryOTPStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}
