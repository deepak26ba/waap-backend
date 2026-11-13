package user

import (
	"errors"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
)

var (
	ErrOTPNotFound    = errors.New("otp not found")
	ErrOTPExpired     = errors.New("otp expired")
	ErrOTPAlreadyUsed = errors.New("otp already used")

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
	Close()
}

// TokenStore is an alias for OTPStore.
type TokenStore = OTPStore

// MemoryOTPStore is a thread-safe in-memory store for verification OTPs keyed by user ID.
type MemoryOTPStore struct {
	mu     sync.RWMutex
	otps   map[uuid.UUID]otpItem
	stopCh chan struct{}
}

// MemoryTokenStore is an alias for MemoryOTPStore.
type MemoryTokenStore = MemoryOTPStore

// NewMemoryOTPStore constructs a MemoryOTPStore and starts background expiration cleanup.
func NewMemoryOTPStore(cleanupInterval time.Duration) *MemoryOTPStore {
	if cleanupInterval <= 0 {
		cleanupInterval = 1 * time.Minute
	}
	s := &MemoryOTPStore{
		otps:   make(map[uuid.UUID]otpItem),
		stopCh: make(chan struct{}),
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
func (s *MemoryOTPStore) Validate(userID uuid.UUID, otpHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.otps[userID]
	if !exists {
		return ErrOTPNotFound
	}

	if item.used {
		return ErrOTPAlreadyUsed
	}

	if time.Now().After(item.expiresAt) {
		delete(s.otps, userID)
		return ErrOTPExpired
	}

	if item.otpHash != otpHash {
		return ErrOTPNotFound
	}

	return nil
}

// Consume marks the user's OTP as consumed, enforcing single-use.
func (s *MemoryOTPStore) Consume(userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.otps[userID]
	if !exists {
		return ErrOTPNotFound
	}

	item.used = true
	s.otps[userID] = item
	return nil
}

// DeleteByUserID purges the active OTP associated with a given user ID.
func (s *MemoryOTPStore) DeleteByUserID(userID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.otps, userID)
}

// cleanupExpired removes all OTPs whose expiration timestamp has passed.
func (s *MemoryOTPStore) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.otps {
		if now.After(v.expiresAt) {
			delete(s.otps, k)
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
