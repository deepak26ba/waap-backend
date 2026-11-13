package product

import (
	"context"

	"waap-backend/pkg/db"
)

// Repository defines data access operations for products and categories.
type Repository interface {
	GetProductByID(ctx context.Context, id int64) (db.Product, error)
	GetProductBySlug(ctx context.Context, slug string) (db.Product, error)
	ListCategories(ctx context.Context) ([]db.Category, error)
}

type repository struct {
	queries db.Querier
}

// NewRepository constructs a product repository.
func NewRepository(queries db.Querier) Repository {
	return &repository{queries: queries}
}

func (r *repository) GetProductByID(ctx context.Context, id int64) (db.Product, error) {
	return r.queries.GetProductByID(ctx, id)
}

func (r *repository) GetProductBySlug(ctx context.Context, slug string) (db.Product, error) {
	return r.queries.GetProductBySlug(ctx, slug)
}

func (r *repository) ListCategories(ctx context.Context) ([]db.Category, error) {
	return r.queries.ListCategories(ctx)
}
