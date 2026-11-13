package cart

import (
	"context"

	"github.com/gofrs/uuid/v5"

	"waap-backend/pkg/response"
)

// Service defines business operations for shopping cart management.
type Service interface {
	CheckAndReserveStock(ctx context.Context, variantID int64, quantity int32) (bool, error)
	GetCart(ctx context.Context, userID *uuid.UUID, guestToken string) (*CartResponse, error)
}

type service struct {
	repo Repository
}

// NewService constructs a cart service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CheckAndReserveStock(ctx context.Context, variantID int64, quantity int32) (bool, error) {
	if quantity <= 0 {
		return false, response.NewBadRequestError("", "quantity must be greater than zero")
	}
	return s.repo.DeductStock(ctx, variantID, quantity)
}

func (s *service) GetCart(ctx context.Context, userID *uuid.UUID, guestToken string) (*CartResponse, error) {
	return s.repo.GetOrCreateCart(ctx, userID, guestToken)
}
