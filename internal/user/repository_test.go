package user

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"waap-backend/pkg/db"
)

type mockDBQuerier struct {
	db.Querier
	usersByEmail map[string]db.User
	usersByID    map[int64]db.User
	nextID       int64
}

func newMockDBQuerier() *mockDBQuerier {
	return &mockDBQuerier{
		usersByEmail: make(map[string]db.User),
		usersByID:    make(map[int64]db.User),
		nextID:       1,
	}
}

func (m *mockDBQuerier) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	u := db.User{
		ID:           m.nextID,
		Email:        arg.Email,
		PasswordHash: arg.PasswordHash,
		FullName:     arg.FullName,
		Role:         arg.Role,
		PhoneNumber:  arg.PhoneNumber,
		IsActive:     true,
		CreatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.nextID++
	m.usersByEmail[u.Email] = u
	m.usersByID[u.ID] = u
	return u, nil
}

func (m *mockDBQuerier) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	return m.usersByEmail[email], nil
}

func (m *mockDBQuerier) GetUserByID(ctx context.Context, id int64) (db.User, error) {
	return m.usersByID[id], nil
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
		t.Errorf("expected ID %d, got %d", created.ID, byEmail.ID)
	}

	byID, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get user by ID: %v", err)
	}
	if byID.Email != "repo@example.com" {
		t.Errorf("expected email repo@example.com, got %s", byID.Email)
	}
}
