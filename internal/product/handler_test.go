package product

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type mockProductService struct {
	getProductFunc     func(ctx context.Context, id int64) (*ProductResponse, error)
	listCategoriesFunc func(ctx context.Context) ([]CategoryResponse, error)
}

func (m *mockProductService) GetProductByID(ctx context.Context, id int64) (*ProductResponse, error) {
	if m.getProductFunc != nil {
		return m.getProductFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockProductService) ListCategories(ctx context.Context) ([]CategoryResponse, error) {
	if m.listCategoriesFunc != nil {
		return m.listCategoriesFunc(ctx)
	}
	return nil, nil
}

func setupTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/catalog")
	h.RegisterRoutes(rg)
	return r
}

func TestProductHandler(t *testing.T) {
	t.Run("ListCategories returns 200 OK", func(t *testing.T) {
		svc := &mockProductService{
			listCategoriesFunc: func(ctx context.Context) ([]CategoryResponse, error) {
				return []CategoryResponse{
					{ID: 1, Name: "Gadgets", Slug: "gadgets"},
				}, nil
			},
		}
		h := NewHandler(svc, nil)
		r := setupTestRouter(h)

		req, _ := http.NewRequest(http.MethodGet, "/catalog/categories", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("GetProduct with invalid ID returns 400", func(t *testing.T) {
		h := NewHandler(&mockProductService{}, nil)
		r := setupTestRouter(h)

		req, _ := http.NewRequest(http.MethodGet, "/catalog/products/abc", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

