package cart

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"waap-backend/pkg/response"
)

// Handler exposes shopping cart HTTP endpoints.
type Handler struct {
	service Service
	log     *zap.Logger
}

// NewHandler constructs a cart HTTP handler.
func NewHandler(service Service, log *zap.Logger) *Handler {
	return &Handler{
		service: service,
		log:     log,
	}
}

// RegisterRoutes registers cart endpoints on the provided router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("", h.GetCart)
}

func (h *Handler) GetCart(c *gin.Context) {
	response.OK(c, CartResponse{
		ID:        "active-cart",
		Items:     []Item{},
		ItemCount: 0,
	})
}
