package user

import (
	"errors"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
)

var (
	ErrTokenNotFound    = errors.New("token not found")
	ErrTokenExpired     = errors.New("token expired")
	ErrTokenAlreadyUsed = errors.New("token already used")
)

type tokenItem struct {
	userID    uuid.UUID
	expiresAt time.Time
	used      bool
}

// TokenStore defines the contract for storing and validating verification tokens in memory.
type TokenStore interface {
	Store(tokenHash string, userID uuid.UUID, ttl time.Duration)
	ValidateAndConsume(tokenHash string) (uuid.UUID, error)
	Delete(tokenHash string)
	DeleteByUserID(userID uuid.UUID)
	Close()
}

// MemoryTokenStore is a thread-safe in-memory store for verification tokens that deletes expired entries.
type MemoryTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]tokenItem
	stopCh chan struct{}
}

// NewMemoryTokenStore constructs a MemoryTokenStore and starts background expiration cleanup.
func NewMemoryTokenStore(cleanupInterval time.Duration) *MemoryTokenStore {
	if cleanupInterval <= 0 {
		cleanupInterval = 1 * time.Minute
	}
	s := &MemoryTokenStore{
		tokens: make(map[string]tokenItem),
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

// Store records a token hash, associating it with the user ID and expiration duration.
func (s *MemoryTokenStore) Store(tokenHash string, userID uuid.UUID, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[tokenHash] = tokenItem{
		userID:    userID,
		expiresAt: time.Now().Add(ttl),
		used:      false,
	}
}

// ValidateAndConsume validates the token, marks it consumed to prevent reuse, and purges if expired.
func (s *MemoryTokenStore) ValidateAndConsume(tokenHash string) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.tokens[tokenHash]
	if !exists {
		return uuid.Nil, ErrTokenNotFound
	}

	if item.used {
		return uuid.Nil, ErrTokenAlreadyUsed
	}

	if time.Now().After(item.expiresAt) {
		delete(s.tokens, tokenHash)
		return uuid.Nil, ErrTokenExpired
	}

	// Mark as consumed to enforce single-use
	item.used = true
	s.tokens[tokenHash] = item

	return item.userID, nil
}

// Delete removes a token from memory immediately.
func (s *MemoryTokenStore) Delete(tokenHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, tokenHash)
}

// DeleteByUserID purges all tokens associated with a given user ID.
func (s *MemoryTokenStore) DeleteByUserID(userID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.tokens {
		if v.userID == userID {
			delete(s.tokens, k)
		}
	}
}

// cleanupExpired removes all tokens whose expiration timestamp has passed.
func (s *MemoryTokenStore) cleanupExpired() {
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
func (s *MemoryTokenStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}
