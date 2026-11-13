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
}

func newMockDBQuerier() *mockDBQuerier {
	return &mockDBQuerier{
		usersByEmail: make(map[string]db.User),
		usersByID:    make(map[uuid.UUID]db.User),
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
	repo := NewRepository(mock)
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

	// Verify marking user email verified persists in repository
	updated, err := repo.MarkUserEmailVerified(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to mark email verified: %v", err)
	}
	if !updated.EmailVerified {
		t.Error("expected updated user to have email_verified = true")
	}

	verifiedUser, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to fetch user after verification: %v", err)
	}
	if !verifiedUser.EmailVerified {
		t.Error("expected persisted user to have email_verified = true")
	}
}
