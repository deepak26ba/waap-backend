package user

import (
	"errors"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
)

var (
	ErrResetTokenNotFound    = errors.New("reset token not found")
	ErrResetTokenExpired     = errors.New("reset token expired")
	ErrResetTokenAlreadyUsed = errors.New("reset token already used")
)

type resetTokenItem struct {
	tokenHash string
	userID    uuid.UUID
	expiresAt time.Time
	used      bool
}

// PasswordResetStore defines the contract for storing, validating, and consuming password reset tokens
// and tracking session revocation after a successful password reset.
type PasswordResetStore interface {
	Store(tokenHash string, userID uuid.UUID, ttl time.Duration)
	Validate(tokenHash string) (uuid.UUID, error)
	Reserve(tokenHash string) (uuid.UUID, error)
	Release(tokenHash string)
	Consume(tokenHash string) error
	DeleteByUserID(userID uuid.UUID)
	RevokeSessions(userID uuid.UUID)
	IsSessionRevoked(userID uuid.UUID, issuedAt time.Time, issuedAtNano ...int64) bool
	Close()
}

// MemoryPasswordResetStore is a thread-safe in-memory store for password reset tokens and session revocations.
type MemoryPasswordResetStore struct {
	mu               sync.RWMutex
	tokens           map[string]resetTokenItem
	revocationCutoff map[uuid.UUID]time.Time
	stopCh           chan struct{}
}

// NewMemoryPasswordResetStore constructs a MemoryPasswordResetStore and starts background expiration cleanup.
func NewMemoryPasswordResetStore(cleanupInterval time.Duration) *MemoryPasswordResetStore {
	if cleanupInterval <= 0 {
		cleanupInterval = 1 * time.Minute
	}
	s := &MemoryPasswordResetStore{
		tokens:           make(map[string]resetTokenItem),
		revocationCutoff: make(map[uuid.UUID]time.Time),
		stopCh:           make(chan struct{}),
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

// Store records a password reset token hash for a specific user ID with expiration duration.
func (s *MemoryPasswordResetStore) Store(tokenHash string, userID uuid.UUID, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Invalidate any older reset tokens for this user
	for k, v := range s.tokens {
		if v.userID == userID {
			delete(s.tokens, k)
		}
	}

	s.tokens[tokenHash] = resetTokenItem{
		tokenHash: tokenHash,
		userID:    userID,
		expiresAt: time.Now().Add(ttl),
		used:      false,
	}
}

// Validate checks if a token hash exists, is unconsumed, and is not expired without consuming it.
func (s *MemoryPasswordResetStore) Validate(tokenHash string) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.tokens[tokenHash]
	if !exists {
		return uuid.Nil, ErrResetTokenNotFound
	}

	if item.used {
		return uuid.Nil, ErrResetTokenAlreadyUsed
	}

	if time.Now().After(item.expiresAt) {
		delete(s.tokens, tokenHash)
		return uuid.Nil, ErrResetTokenExpired
	}

	return item.userID, nil
}

// Reserve atomically marks the token as used/in-flight and returns the user ID.
// If the token is already used, expired, or non-existent, it returns the appropriate error.
func (s *MemoryPasswordResetStore) Reserve(tokenHash string) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.tokens[tokenHash]
	if !exists {
		return uuid.Nil, ErrResetTokenNotFound
	}

	if item.used {
		return uuid.Nil, ErrResetTokenAlreadyUsed
	}

	if time.Now().After(item.expiresAt) {
		delete(s.tokens, tokenHash)
		return uuid.Nil, ErrResetTokenExpired
	}

	item.used = true
	s.tokens[tokenHash] = item
	return item.userID, nil
}

// Release unmarks the token as used if an error occurred during subsequent operations before completion.
func (s *MemoryPasswordResetStore) Release(tokenHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.tokens[tokenHash]
	if exists && item.used {
		item.used = false
		s.tokens[tokenHash] = item
	}
}

// Consume marks the reset token as consumed, enforcing single-use.
func (s *MemoryPasswordResetStore) Consume(tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.tokens[tokenHash]
	if !exists {
		return ErrResetTokenNotFound
	}

	item.used = true
	s.tokens[tokenHash] = item
	return nil
}

// DeleteByUserID purges all active reset tokens for a given user ID.
func (s *MemoryPasswordResetStore) DeleteByUserID(userID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, v := range s.tokens {
		if v.userID == userID {
			delete(s.tokens, k)
		}
	}
}

// RevokeSessions marks all sessions and refresh tokens issued prior to this moment as revoked.
func (s *MemoryPasswordResetStore) RevokeSessions(userID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revocationCutoff[userID] = time.Now()
}

// IsSessionRevoked checks whether a token issued at the specified time was revoked due to password reset.
func (s *MemoryPasswordResetStore) IsSessionRevoked(userID uuid.UUID, issuedAt time.Time, issuedAtNano ...int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff, ok := s.revocationCutoff[userID]
	if !ok {
		return false
	}

	if len(issuedAtNano) > 0 && issuedAtNano[0] > 0 {
		return issuedAtNano[0] <= cutoff.UnixNano()
	}

	// Any token issued before or at the revocation cutoff is invalid
	return !issuedAt.After(cutoff)
}

// cleanupExpired removes all tokens whose expiration timestamp has passed.
func (s *MemoryPasswordResetStore) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.tokens {
		if now.After(v.expiresAt) {
			delete(s.tokens, k)
		}
	}
}

// Close stops the periodic cleanup goroutine.
func (s *MemoryPasswordResetStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}
