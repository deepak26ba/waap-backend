package cart

import (
	"context"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"waap-backend/pkg/db"
)

type mockCartDBQuerier struct {
	db.Querier
	cartsByUserID     map[uuid.UUID]db.Cart
	cartsByGuestToken map[string]db.Cart
	cartItems         map[uuid.UUID][]db.CartItem
	deductRows        int64
	deductErr         error
}

func newMockCartDBQuerier() *mockCartDBQuerier {
	return &mockCartDBQuerier{
		cartsByUserID:     make(map[uuid.UUID]db.Cart),
		cartsByGuestToken: make(map[string]db.Cart),
		cartItems:         make(map[uuid.UUID][]db.CartItem),
		deductRows:        1,
	}
}

func (m *mockCartDBQuerier) GetCartByUserID(ctx context.Context, userID pgtype.UUID) (db.Cart, error) {
	if !userID.Valid {
		return db.Cart{}, pgx.ErrNoRows
	}
	uid, _ := uuid.FromBytes(userID.Bytes[:])
	c, ok := m.cartsByUserID[uid]
	if !ok {
		return db.Cart{}, pgx.ErrNoRows
	}
	return c, nil
}

func (m *mockCartDBQuerier) GetCartByGuestToken(ctx context.Context, guestToken pgtype.Text) (db.Cart, error) {
	if !guestToken.Valid || guestToken.String == "" {
		return db.Cart{}, pgx.ErrNoRows
	}
	c, ok := m.cartsByGuestToken[guestToken.String]
	if !ok {
		return db.Cart{}, pgx.ErrNoRows
	}
	return c, nil
}

func (m *mockCartDBQuerier) CreateCart(ctx context.Context, arg db.CreateCartParams) (db.Cart, error) {
	newID := uuid.Must(uuid.NewV4())
	c := db.Cart{
		ID:         newID,
		UserID:     arg.UserID,
		GuestToken: arg.GuestToken,
		CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	if arg.UserID.Valid {
		uid, _ := uuid.FromBytes(arg.UserID.Bytes[:])
		m.cartsByUserID[uid] = c
	}
	if arg.GuestToken.Valid {
		m.cartsByGuestToken[arg.GuestToken.String] = c
	}
	return c, nil
}

func (m *mockCartDBQuerier) GetCartItems(ctx context.Context, cartID uuid.UUID) ([]db.CartItem, error) {
	return m.cartItems[cartID], nil
}

func (m *mockCartDBQuerier) DeductStockForCheckout(ctx context.Context, arg db.DeductStockForCheckoutParams) (int64, error) {
	return m.deductRows, m.deductErr
}

func TestRepository_GetOrCreateCart_Guest_New(t *testing.T) {
	mock := newMockCartDBQuerier()
	repo := NewRepository(nil, mock)
	ctx := context.Background()

	res, err := repo.GetOrCreateCart(ctx, nil, "guest-abc-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil cart response")
	}
	if res.GuestToken == nil || *res.GuestToken != "guest-abc-123" {
		t.Errorf("expected guest token guest-abc-123, got %v", res.GuestToken)
	}
	if res.ItemCount != 0 {
		t.Errorf("expected 0 items, got %d", res.ItemCount)
	}
}

func TestRepository_GetOrCreateCart_User_ExistingWithItems(t *testing.T) {
	mock := newMockCartDBQuerier()
	userID := uuid.Must(uuid.NewV4())
	cartID := uuid.Must(uuid.NewV4())

	mock.cartsByUserID[userID] = db.Cart{
		ID:        cartID,
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	mock.cartItems[cartID] = []db.CartItem{
		{ID: 1, CartID: cartID, VariantID: 101, Quantity: 3, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
		{ID: 2, CartID: cartID, VariantID: 102, Quantity: 2, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
	}

	repo := NewRepository(nil, mock)
	ctx := context.Background()

	res, err := repo.GetOrCreateCart(ctx, &userID, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.ID != cartID {
		t.Errorf("expected cart ID %s, got %s", cartID, res.ID)
	}
	if res.UserID == nil || *res.UserID != userID {
		t.Errorf("expected user ID %s, got %v", userID, res.UserID)
	}
	if res.ItemCount != 5 {
		t.Errorf("expected item count 5, got %d", res.ItemCount)
	}
	if len(res.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(res.Items))
	}
}

func TestRepository_GetOrCreateCart_NilQuerier(t *testing.T) {
	repo := NewRepository(nil, nil)
	ctx := context.Background()

	_, err := repo.GetOrCreateCart(ctx, nil, "guest-tok")
	if err == nil {
		t.Fatal("expected error when queries is nil, got nil")
	}
}

func TestRepository_DeductStock(t *testing.T) {
	mock := newMockCartDBQuerier()
	repo := NewRepository(nil, mock)
	ctx := context.Background()

	mock.deductRows = 1
	ok, err := repo.DeductStock(ctx, 10, 2)
	if err != nil || !ok {
		t.Fatalf("expected success, got ok=%v, err=%v", ok, err)
	}

	mock.deductRows = 0
	ok, err = repo.DeductStock(ctx, 10, 2)
	if err != nil || ok {
		t.Fatalf("expected false, got ok=%v, err=%v", ok, err)
	}
}
