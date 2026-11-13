package cart

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
)

func TestCartHandler_GetCart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tok := "test-guest-token-123"
	h := NewHandler(&mockCartService{guestToken: &tok}, nil)
	r := gin.New()
	r.GET("/cart", h.GetCart)

	req, _ := http.NewRequest(http.MethodGet, "/cart", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	if w.Header().Get("X-Guest-Token") != tok {
		t.Errorf("expected X-Guest-Token %s, got %s", tok, w.Header().Get("X-Guest-Token"))
	}
}

type mockCartService struct {
	guestToken *string
}

func (m *mockCartService) CheckAndReserveStock(ctx context.Context, id int64, qty int32) (bool, error) {
	return true, nil
}

func (m *mockCartService) GetCart(ctx context.Context, userID *uuid.UUID, guestToken string) (*CartResponse, error) {
	return &CartResponse{
		ID:         uuid.Must(uuid.NewV4()),
		UserID:     userID,
		GuestToken: m.guestToken,
		Items:      []Item{},
		ItemCount:  0,
	}, nil
}
