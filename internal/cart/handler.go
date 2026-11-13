package cart

import (
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
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

// GetCart godoc
// @Summary Retrieve current shopping cart
// @Tags Cart
// @Produce json
// @Success 200 {object} response.Envelope{data=CartResponse}
// @Router /api/v1/cart [get]
func (h *Handler) GetCart(c *gin.Context) {
	response.OK(c, CartResponse{
		ID:        uuid.Must(uuid.NewV4()),
		Items:     []Item{},
		ItemCount: 0,
	})
}
