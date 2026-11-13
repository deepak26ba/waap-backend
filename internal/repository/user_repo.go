package repository

import (
	"context"

	"waap-backend/pkg/db"
)

// UserRepository defines database access operations for users.
type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (db.User, error)
	GetByID(ctx context.Context, id int64) (db.User, error)
	Create(ctx context.Context, params db.CreateUserParams) (db.User, error)
	Update(ctx context.Context, params db.UpdateUserParams) (db.User, error)
}

type userRepository struct {
	queries db.Querier
}

// NewUserRepository creates a new UserRepository backed by db.Querier.
func NewUserRepository(queries db.Querier) UserRepository {
	return &userRepository{queries: queries}
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (db.User, error) {
	return r.queries.GetUserByEmail(ctx, email)
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (db.User, error) {
	return r.queries.GetUserByID(ctx, id)
}

func (r *userRepository) Create(ctx context.Context, params db.CreateUserParams) (db.User, error) {
	return r.queries.CreateUser(ctx, params)
}

func (r *userRepository) Update(ctx context.Context, params db.UpdateUserParams) (db.User, error) {
	return r.queries.UpdateUser(ctx, params)
}
