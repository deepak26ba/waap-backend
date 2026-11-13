package user

import (
	"context"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"waap-backend/pkg/db"
)

type mockDBQuerier struct {
	db.Querier
	usersByEmail map[string]db.User
	usersByID    map[uuid.UUID]db.User
	tokensByHash map[string]db.EmailVerificationToken
	tokensByID   map[int64]db.EmailVerificationToken
	tokenSeq     int64
}

func newMockDBQuerier() *mockDBQuerier {
	return &mockDBQuerier{
		usersByEmail: make(map[string]db.User),
		usersByID:    make(map[uuid.UUID]db.User),
		tokensByHash: make(map[string]db.EmailVerificationToken),
		tokensByID:   make(map[int64]db.EmailVerificationToken),
	}
}

func (m *mockDBQuerier) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	newID := uuid.Must(uuid.NewV4())
	u := db.User{
		ID:            newID,
		Email:         arg.Email,
		PasswordHash:  arg.PasswordHash,
		FullName:      arg.FullName,
		Role:          arg.Role,
		PhoneNumber:   arg.PhoneNumber,
		IsActive:      true,
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.usersByEmail[u.Email] = u
	m.usersByID[u.ID] = u
	return u, nil
}

func (m *mockDBQuerier) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	u, ok := m.usersByEmail[email]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *mockDBQuerier) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *mockDBQuerier) UpdateUser(ctx context.Context, arg db.UpdateUserParams) (db.User, error) {
	u := m.usersByID[arg.ID]
	if arg.FullName != "" {
		u.FullName = arg.FullName
	}
	m.usersByID[arg.ID] = u
	m.usersByEmail[u.Email] = u
	return u, nil
}

func (m *mockDBQuerier) CreateEmailVerificationToken(ctx context.Context, arg db.CreateEmailVerificationTokenParams) (db.EmailVerificationToken, error) {
	m.tokenSeq++
	tok := db.EmailVerificationToken{
		ID:        m.tokenSeq,
		UserID:    arg.UserID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.tokensByHash[arg.TokenHash] = tok
	m.tokensByID[tok.ID] = tok
	return tok, nil
}

func (m *mockDBQuerier) GetEmailVerificationTokenByHash(ctx context.Context, tokenHash string) (db.EmailVerificationToken, error) {
	tok, ok := m.tokensByHash[tokenHash]
	if !ok {
		return db.EmailVerificationToken{}, pgx.ErrNoRows
	}
	return tok, nil
}

func (m *mockDBQuerier) MarkEmailVerificationTokenUsed(ctx context.Context, id int64) (db.EmailVerificationToken, error) {
	tok, ok := m.tokensByID[id]
	if !ok {
		return db.EmailVerificationToken{}, pgx.ErrNoRows
	}
	tok.UsedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.tokensByID[id] = tok
	m.tokensByHash[tok.TokenHash] = tok
	return tok, nil
}

func (m *mockDBQuerier) MarkUserEmailVerified(ctx context.Context, id uuid.UUID) (db.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	u.EmailVerified = true
	m.usersByID[id] = u
	m.usersByEmail[u.Email] = u
	return u, nil
}

func TestUserRepository(t *testing.T) {
	mock := newMockDBQuerier()
	repo := NewRepository(nil, mock)
	ctx := context.Background()

	created, err := repo.Create(ctx, db.CreateUserParams{
		Email:        "repo@example.com",
		PasswordHash: "hashed",
		FullName:     "Repo User",
		Role:         "customer",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	byEmail, err := repo.GetByEmail(ctx, "repo@example.com")
	if err != nil {
		t.Fatalf("failed to get user by email: %v", err)
	}
	if byEmail.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, byEmail.ID)
	}

	byID, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get user by ID: %v", err)
	}
	if byID.Email != "repo@example.com" {
		t.Errorf("expected email repo@example.com, got %s", byID.Email)
	}

	// Test Email Verification Token persistence and verification
	tok, err := repo.CreateVerificationToken(ctx, db.CreateEmailVerificationTokenParams{
		UserID:    created.ID,
		TokenHash: "dummyhash123",
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("failed to create verification token: %v", err)
	}

	fetchedTok, err := repo.GetVerificationTokenByHash(ctx, "dummyhash123")
	if err != nil {
		t.Fatalf("failed to fetch verification token: %v", err)
	}
	if fetchedTok.ID != tok.ID {
		t.Errorf("expected token ID %d, got %d", tok.ID, fetchedTok.ID)
	}

	if err := repo.VerifyEmailTx(ctx, tok.ID, created.ID); err != nil {
		t.Fatalf("VerifyEmailTx failed: %v", err)
	}

	verifiedUser, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get verified user: %v", err)
	}
	if !verifiedUser.EmailVerified {
		t.Error("expected user email_verified to be true")
	}

	verifiedTok, err := repo.GetVerificationTokenByHash(ctx, "dummyhash123")
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}
	if !verifiedTok.UsedAt.Valid {
		t.Error("expected token used_at to be valid")
	}
}
