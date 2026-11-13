package cart

import (
	"context"
	"errors"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"waap-backend/pkg/db"
)

// Repository defines data access operations for shopping cart persistence.
type Repository interface {
	DeductStock(ctx context.Context, variantID int64, quantity int32) (bool, error)
	GetOrCreateCart(ctx context.Context, userID *uuid.UUID, guestToken string) (*CartResponse, error)
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

func (r *repository) GetOrCreateCart(ctx context.Context, userID *uuid.UUID, guestToken string) (*CartResponse, error) {
	var cart db.Cart

	if userID != nil && *userID != uuid.Nil {
		c, err := r.queries.GetCartByUserID(ctx, pgtype.UUID{Bytes: *userID, Valid: true})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				created, cErr := r.queries.CreateCart(ctx, db.CreateCartParams{
					UserID: pgtype.UUID{Bytes: *userID, Valid: true},
				})
				if cErr != nil {
					return nil, cErr
				}
				cart = created
			} else {
				return nil, err
			}
		} else {
			cart = c
		}
	} else {
		if guestToken == "" {
			guestToken = uuid.Must(uuid.NewV4()).String()
		}

		c, err := r.queries.GetCartByGuestToken(ctx, pgtype.Text{String: guestToken, Valid: true})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				created, cErr := r.queries.CreateCart(ctx, db.CreateCartParams{
					GuestToken: pgtype.Text{String: guestToken, Valid: true},
				})
				if cErr != nil {
					return nil, cErr
				}
				cart = created
			} else {
				return nil, err
			}
		} else {
			cart = c
		}
	}

	rawItems, err := r.queries.GetCartItems(ctx, cart.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	items := make([]Item, 0, len(rawItems))
	var totalCount int32
	for _, it := range rawItems {
		items = append(items, Item{
			ID:        it.ID,
			CartID:    it.CartID,
			VariantID: it.VariantID,
			Quantity:  it.Quantity,
			CreatedAt: it.CreatedAt.Time,
		})
		totalCount += it.Quantity
	}

	var resUserID *uuid.UUID
	if cart.UserID.Valid {
		uid, uErr := uuid.FromBytes(cart.UserID.Bytes[:])
		if uErr == nil {
			resUserID = &uid
		}
	}

	return &CartResponse{
		ID:        cart.ID,
		UserID:    resUserID,
		Items:     items,
		ItemCount: totalCount,
	}, nil
}
