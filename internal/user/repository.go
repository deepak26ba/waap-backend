package user

import (
	"context"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"waap-backend/pkg/db"
)

// Repository defines data access operations for users and email verification tokens.
type Repository interface {
	GetByEmail(ctx context.Context, email string) (db.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.User, error)
	Create(ctx context.Context, params db.CreateUserParams) (db.User, error)
	Update(ctx context.Context, params db.UpdateUserParams) (db.User, error)
	CreateVerificationToken(ctx context.Context, params db.CreateEmailVerificationTokenParams) (db.EmailVerificationToken, error)
	GetVerificationTokenByHash(ctx context.Context, tokenHash string) (db.EmailVerificationToken, error)
	MarkVerificationTokenUsed(ctx context.Context, id int64) (db.EmailVerificationToken, error)
	MarkUserEmailVerified(ctx context.Context, id uuid.UUID) (db.User, error)
	VerifyEmailTx(ctx context.Context, tokenID int64, userID uuid.UUID) error
}

type repository struct {
	pool    *pgxpool.Pool
	queries db.Querier
}

// NewRepository creates a new user Repository backed by db.Querier and optional pool for transactions.
func NewRepository(pool *pgxpool.Pool, queries db.Querier) Repository {
	return &repository{pool: pool, queries: queries}
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

func (r *repository) CreateVerificationToken(ctx context.Context, params db.CreateEmailVerificationTokenParams) (db.EmailVerificationToken, error) {
	return r.queries.CreateEmailVerificationToken(ctx, params)
}

func (r *repository) GetVerificationTokenByHash(ctx context.Context, tokenHash string) (db.EmailVerificationToken, error) {
	return r.queries.GetEmailVerificationTokenByHash(ctx, tokenHash)
}

func (r *repository) MarkVerificationTokenUsed(ctx context.Context, id int64) (db.EmailVerificationToken, error) {
	return r.queries.MarkEmailVerificationTokenUsed(ctx, id)
}

func (r *repository) MarkUserEmailVerified(ctx context.Context, id uuid.UUID) (db.User, error) {
	return r.queries.MarkUserEmailVerified(ctx, id)
}

func (r *repository) VerifyEmailTx(ctx context.Context, tokenID int64, userID uuid.UUID) error {
	if r.pool != nil {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		qtx := db.New(tx)
		if _, err := qtx.MarkEmailVerificationTokenUsed(ctx, tokenID); err != nil {
			return err
		}
		if _, err := qtx.MarkUserEmailVerified(ctx, userID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	if _, err := r.queries.MarkEmailVerificationTokenUsed(ctx, tokenID); err != nil {
		return err
	}
	if _, err := r.queries.MarkUserEmailVerified(ctx, userID); err != nil {
		return err
	}
	return nil
}
