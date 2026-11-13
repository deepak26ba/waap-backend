package cart

import (
	"context"
	"testing"
)

type mockCartRepo struct {
	availableStock map[int64]int32
}

func (m *mockCartRepo) DeductStock(ctx context.Context, variantID int64, quantity int32) (bool, error) {
	curr, ok := m.availableStock[variantID]
	if !ok || curr < quantity {
		return false, nil
	}
	m.availableStock[variantID] -= quantity
	return true, nil
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

	ok, err = svc.CheckAndReserveStock(ctx, 100, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Errorf("expected stock reservation to fail for excessive quantity")
	}
}

