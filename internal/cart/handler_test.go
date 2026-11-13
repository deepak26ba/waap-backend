package cart

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCartHandler_GetCart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(&mockCartService{}, nil)
	r := gin.New()
	rg := r.Group("/cart")
	h.RegisterRoutes(rg)

	req, _ := http.NewRequest(http.MethodGet, "/cart", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
}

type mockCartService struct{}

func (m *mockCartService) CheckAndReserveStock(ctx context.Context, id int64, qty int32) (bool, error) {
	return true, nil
}
