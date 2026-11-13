package cart

import (
	"context"
)

// Service defines business operations for shopping cart management.
type Service interface {
	CheckAndReserveStock(ctx context.Context, variantID int64, quantity int32) (bool, error)
}

type service struct {
	repo Repository
}

// NewService constructs a cart service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CheckAndReserveStock(ctx context.Context, variantID int64, quantity int32) (bool, error) {
	return s.repo.DeductStock(ctx, variantID, quantity)
}

