package product

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"waap-backend/pkg/response"
)

// Handler exposes product catalog HTTP endpoints.
type Handler struct {
	service Service
	log     *zap.Logger
}

// NewHandler constructs a product handler.
func NewHandler(service Service, log *zap.Logger) *Handler {
	return &Handler{
		service: service,
		log:     log,
	}
}

// RegisterRoutes registers catalog endpoints on the provided router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/categories", h.ListCategories)
	rg.GET("/products/:id", h.GetProduct)
}

// ListCategories godoc
// @Summary List all product categories
// @Tags Catalog
// @Produce json
// @Success 200 {object} response.Envelope{data=[]CategoryResponse}
// @Failure 500 {object} response.Envelope{error=response.APIError}
// @Router /api/v1/categories [get]
func (h *Handler) ListCategories(c *gin.Context) {
	cats, err := h.service.ListCategories(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, cats)
}

// GetProduct godoc
// @Summary Get product by ID
// @Tags Catalog
// @Produce json
// @Param id path int true "Product ID"
// @Success 200 {object} response.Envelope{data=ProductResponse}
// @Failure 400 {object} response.Envelope{error=response.APIError}
// @Failure 404 {object} response.Envelope{error=response.APIError}
// @Router /api/v1/products/{id} [get]
func (h *Handler) GetProduct(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, response.ErrCodeBadRequest, "invalid product id parameter")
		return
	}

	p, err := h.service.GetProductByID(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.OK(c, p)
}
