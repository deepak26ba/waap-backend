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
	userID    uuid.UUID
	expiresAt time.Time
	used      bool
}

// OTPStore defines the contract for storing and validating verification OTPs in memory.
type OTPStore interface {
	Store(otpHash string, userID uuid.UUID, ttl time.Duration)
	ValidateAndConsume(otpHash string) (uuid.UUID, error)
	Delete(otpHash string)
	DeleteByUserID(userID uuid.UUID)
	Close()
}

// TokenStore is an alias for OTPStore.
type TokenStore = OTPStore

// MemoryOTPStore is a thread-safe in-memory store for verification OTPs that deletes expired entries.
type MemoryOTPStore struct {
	mu     sync.RWMutex
	otps   map[string]otpItem
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
		otps:   make(map[string]otpItem),
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

// Store records an OTP hash, associating it with the user ID and expiration duration.
func (s *MemoryOTPStore) Store(otpHash string, userID uuid.UUID, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.otps[otpHash] = otpItem{
		userID:    userID,
		expiresAt: time.Now().Add(ttl),
		used:      false,
	}
}

// ValidateAndConsume validates the OTP, marks it consumed to prevent reuse, and purges if expired.
func (s *MemoryOTPStore) ValidateAndConsume(otpHash string) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.otps[otpHash]
	if !exists {
		return uuid.Nil, ErrOTPNotFound
	}

	if item.used {
		return uuid.Nil, ErrOTPAlreadyUsed
	}

	if time.Now().After(item.expiresAt) {
		delete(s.otps, otpHash)
		return uuid.Nil, ErrOTPExpired
	}

	// Mark as consumed to enforce single-use
	item.used = true
	s.otps[otpHash] = item

	return item.userID, nil
}

// Delete removes an OTP from memory immediately.
func (s *MemoryOTPStore) Delete(otpHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.otps, otpHash)
}

// DeleteByUserID purges all OTPs associated with a given user ID.
func (s *MemoryOTPStore) DeleteByUserID(userID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.otps {
		if v.userID == userID {
			delete(s.otps, k)
		}
	}
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
