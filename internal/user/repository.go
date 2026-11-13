package user

import (
	"context"

	"github.com/gofrs/uuid/v5"

	"waap-backend/pkg/db"
)

// Repository defines data access operations for users.
type Repository interface {
	GetByEmail(ctx context.Context, email string) (db.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.User, error)
	Create(ctx context.Context, params db.CreateUserParams) (db.User, error)
	Update(ctx context.Context, params db.UpdateUserParams) (db.User, error)
	MarkUserEmailVerified(ctx context.Context, id uuid.UUID) (db.User, error)
}

type repository struct {
	queries db.Querier
}

// NewRepository creates a new user Repository backed by db.Querier.
func NewRepository(queries db.Querier) Repository {
	return &repository{queries: queries}
}

func (r *repository) GetByEmail(ctx context.Context, email string) (db.User, error) {
	return r.queries.GetUserByEmail(ctx, email)
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return r.queries.GetUserByID(ctx, id)
}

func (r *repository) Create(ctx context.Context, params db.CreateUserParams) (db.User, error) {
	return r.queries.CreateUser(ctx, params)
}

func (r *repository) Update(ctx context.Context, params db.UpdateUserParams) (db.User, error) {
	return r.queries.UpdateUser(ctx, params)
}

func (r *repository) MarkUserEmailVerified(ctx context.Context, id uuid.UUID) (db.User, error) {
	return r.queries.MarkUserEmailVerified(ctx, id)
}
