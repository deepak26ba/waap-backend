package cart

import (
	"context"
	"errors"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"waap-backend/pkg/db"
)

// Repository defines data access operations for shopping cart persistence.
type Repository interface {
	DeductStock(ctx context.Context, variantID int64, quantity int32) (bool, error)
	GetOrCreateCart(ctx context.Context, userID *uuid.UUID, guestToken string) (*CartResponse, error)
}

type repository struct {
	pool    *pgxpool.Pool
	queries db.Querier
}

// NewRepository constructs a cart repository.
func NewRepository(pool *pgxpool.Pool, queries db.Querier) Repository {
	return &repository{pool: pool, queries: queries}
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
	if guestToken == "" && (userID == nil || *userID == uuid.Nil) {
		guestToken = uuid.Must(uuid.NewV4()).String()
	}

	// When a database pool is configured, run in a transaction with an advisory lock
	// to prevent race conditions and duplicate cart creation under concurrent requests.
	if r.pool != nil {
		return r.getOrCreateCartTx(ctx, userID, guestToken)
	}

	return r.getOrCreateCartDirect(ctx, r.queries, userID, guestToken)
}

func (r *repository) getOrCreateCartTx(ctx context.Context, userID *uuid.UUID, guestToken string) (*CartResponse, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Key advisory lock on either user ID or guest token
	var lockKey string
	if userID != nil && *userID != uuid.Nil {
		lockKey = "cart_user_" + userID.String()
	} else {
		lockKey = "cart_guest_" + guestToken
	}

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", lockKey); err != nil {
		return nil, err
	}

	qtx := db.New(tx)
	res, err := r.getOrCreateCartDirect(ctx, qtx, userID, guestToken)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return res, nil
}

func (r *repository) getOrCreateCartDirect(ctx context.Context, q db.Querier, userID *uuid.UUID, guestToken string) (*CartResponse, error) {
	if q == nil {
		return nil, errors.New("database querier is uninitialized")
	}

	var cart db.Cart

	if userID != nil && *userID != uuid.Nil {
		c, err := q.GetCartByUserID(ctx, pgtype.UUID{Bytes: *userID, Valid: true})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				created, cErr := q.CreateCart(ctx, db.CreateCartParams{
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
		c, err := q.GetCartByGuestToken(ctx, pgtype.Text{String: guestToken, Valid: true})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				created, cErr := q.CreateCart(ctx, db.CreateCartParams{
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

	rawItems, err := q.GetCartItems(ctx, cart.ID)
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

	var resGuestToken *string
	if cart.GuestToken.Valid && cart.GuestToken.String != "" {
		gt := cart.GuestToken.String
		resGuestToken = &gt
	}

	return &CartResponse{
		ID:         cart.ID,
		UserID:     resUserID,
		GuestToken: resGuestToken,
		Items:      items,
		ItemCount:  totalCount,
	}, nil
}
