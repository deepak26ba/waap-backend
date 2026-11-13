package cart

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"go.uber.org/zap"

	"waap-backend/internal/user"
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
// @Param X-Guest-Token header string false "Guest Cart Token"
// @Param guest_token query string false "Guest Cart Token Query Parameter"
// @Success 200 {object} response.Envelope{data=CartResponse}
// @Failure 500 {object} response.Envelope{error=response.APIError}
// @Router /cart [get]
func (h *Handler) GetCart(c *gin.Context) {
	var userUUID *uuid.UUID
	if val, exists := c.Get(user.ContextUserID); exists {
		if id, ok := val.(uuid.UUID); ok && id != uuid.Nil {
			userUUID = &id
		}
	}

	guestToken := strings.TrimSpace(c.GetHeader("X-Guest-Token"))
	if guestToken == "" {
		guestToken = strings.TrimSpace(c.Query("guest_token"))
	}

	res, err := h.service.GetCart(c.Request.Context(), userUUID, guestToken)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	if res.GuestToken != nil && *res.GuestToken != "" {
		c.Header("X-Guest-Token", *res.GuestToken)
	}

	response.OK(c, res)
}
