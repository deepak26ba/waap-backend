package cart

import (
	"context"
	"testing"

	"github.com/gofrs/uuid/v5"
)

type mockCartRepo struct {
	availableStock map[int64]int32
	cart           *CartResponse
}

func (m *mockCartRepo) DeductStock(ctx context.Context, variantID int64, quantity int32) (bool, error) {
	curr, ok := m.availableStock[variantID]
	if !ok || curr < quantity {
		return false, nil
	}
	m.availableStock[variantID] -= quantity
	return true, nil
}

func (m *mockCartRepo) GetOrCreateCart(ctx context.Context, userID *uuid.UUID, guestToken string) (*CartResponse, error) {
	if m.cart != nil {
		return m.cart, nil
	}
	return &CartResponse{
		ID:        uuid.Must(uuid.NewV4()),
		UserID:    userID,
		Items:     []Item{},
		ItemCount: 0,
	}, nil
}

func TestCartService_CheckAndReserveStock(t *testing.T) {
	mock := &mockCartRepo{
		availableStock: map[int64]int32{
			100: 5,
		},
	}
	svc := NewService(mock)
	ctx := context.Background()

	ok, err := svc.CheckAndReserveStock(ctx, 100, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Errorf("expected stock reservation to succeed")
	}

	// Reject excessive quantity
	ok, err = svc.CheckAndReserveStock(ctx, 100, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Errorf("expected stock reservation to fail for excessive quantity")
	}

	// Reject negative quantity
	ok, err = svc.CheckAndReserveStock(ctx, 100, -2)
	if err == nil || ok {
		t.Errorf("expected negative quantity to be rejected")
	}
	// Verify stock unchanged: 5 - 2 = 3
	if mock.availableStock[100] != 3 {
		t.Errorf("expected stock to remain 3, got %d", mock.availableStock[100])
	}

	// Reject zero quantity
	ok, err = svc.CheckAndReserveStock(ctx, 100, 0)
	if err == nil || ok {
		t.Errorf("expected zero quantity to be rejected")
	}
}

func TestCartService_GetCart(t *testing.T) {
	cartID := uuid.Must(uuid.NewV4())
	mock := &mockCartRepo{
		cart: &CartResponse{
			ID:        cartID,
			Items:     []Item{{ID: 1, CartID: cartID, VariantID: 10, Quantity: 2}},
			ItemCount: 2,
		},
	}
	svc := NewService(mock)
	res, err := svc.GetCart(context.Background(), nil, "guest-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != cartID || res.ItemCount != 2 {
		t.Errorf("unexpected cart result: %+v", res)
	}
}
