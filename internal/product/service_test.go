package product

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"waap-backend/pkg/db"
)

type mockRepo struct {
	product db.Product
	cats    []db.Category
}

func (m *mockRepo) GetProductByID(ctx context.Context, id int64) (db.Product, error) {
	return m.product, nil
}

func (m *mockRepo) GetProductBySlug(ctx context.Context, slug string) (db.Product, error) {
	return m.product, nil
}

func (m *mockRepo) ListCategories(ctx context.Context) ([]db.Category, error) {
	return m.cats, nil
}

func TestProductService(t *testing.T) {
	var price pgtype.Numeric
	_ = price.Scan("99.95")

	mock := &mockRepo{
		product: db.Product{
			ID:        10,
			Title:     "Sample Shoes",
			Slug:      "sample-shoes",
			BasePrice: price,
			Status:    "ACTIVE",
			CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
		cats: []db.Category{
			{ID: 1, Name: "Footwear", Slug: "footwear"},
		},
	}

	svc := NewService(mock)
	ctx := context.Background()

	p, err := svc.GetProductByID(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Title != "Sample Shoes" {
		t.Errorf("expected Title Sample Shoes, got %s", p.Title)
	}
	if p.BasePrice != "99.95" {
		t.Errorf("expected BasePrice 99.95, got %s", p.BasePrice)
	}

	cats, err := svc.ListCategories(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cats) != 1 {
		t.Errorf("expected 1 category, got %d", len(cats))
	}
}
