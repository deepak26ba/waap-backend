package cart

import (
	"context"

	"waap-backend/pkg/db"
)

// Repository defines data access operations for shopping cart persistence.
type Repository interface {
	DeductStock(ctx context.Context, variantID int64, quantity int32) (bool, error)
}

type repository struct {
	queries db.Querier
}

// NewRepository constructs a cart repository.
func NewRepository(queries db.Querier) Repository {
	return &repository{queries: queries}
}

func (r *repository) DeductStock(ctx context.Context, variantID int64, quantity int32) (bool, error) {
	rows, err := r.queries.DeductStockForCheckout(ctx, db.DeductStockForCheckoutParams{
		ID:            variantID,
		StockQuantity: quantity,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

