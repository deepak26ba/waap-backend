package product

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"waap-backend/pkg/db"
)

type mockProductQuerier struct {
	db.Querier
	productsByID map[int64]db.Product
	categories   []db.Category
}

func (m *mockProductQuerier) GetProductByID(ctx context.Context, id int64) (db.Product, error) {
	return m.productsByID[id], nil
}

func (m *mockProductQuerier) GetProductBySlug(ctx context.Context, slug string) (db.Product, error) {
	for _, p := range m.productsByID {
		if p.Slug == slug {
			return p, nil
		}
	}
	return db.Product{}, nil
}

func (m *mockProductQuerier) ListCategories(ctx context.Context) ([]db.Category, error) {
	return m.categories, nil
}

func TestProductRepository(t *testing.T) {
	mock := &mockProductQuerier{
		productsByID: map[int64]db.Product{
			1: {
				ID:          1,
				Title:       "Test Product",
				Slug:        "test-product",
				Description: "Product Description",
				Status:      "ACTIVE",
				CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
			},
		},
		categories: []db.Category{
			{ID: 1, Name: "Electronics", Slug: "electronics"},
		},
	}

	repo := NewRepository(mock)
	ctx := context.Background()

	p, err := repo.GetProductByID(ctx, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Title != "Test Product" {
		t.Errorf("expected Title 'Test Product', got %s", p.Title)
	}

	cats, err := repo.ListCategories(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cats) != 1 {
		t.Errorf("expected 1 category, got %d", len(cats))
	}
}
