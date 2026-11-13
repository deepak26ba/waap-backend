package user

import (
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"go.uber.org/zap"

	"waap-backend/pkg/response"
)

// Handler exposes user and authentication HTTP endpoints.
type Handler struct {
	service Service
	log     *zap.Logger
}

// NewHandler constructs a user HTTP handler.
func NewHandler(service Service, log *zap.Logger) *Handler {
	return &Handler{
		service: service,
		log:     log,
	}
}

// Register godoc
// @Summary Register a new customer account
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterInput true "Registration Payload"
// @Success 201 {object} response.Envelope{data=UserResponse}
// @Failure 400 {object} response.Envelope{error=response.APIError}
// @Failure 409 {object} response.Envelope{error=response.APIError}
// @Failure 422 {object} response.Envelope{error=response.APIError}
// @Router /api/v1/auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var in RegisterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_REQUEST_BODY", "malformed or missing JSON request body")
		return
	}

	res, err := h.service.Register(c.Request.Context(), in)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.Created(c, res)
}

// Login godoc
// @Summary Authenticate user and issue JWT token pair
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginInput true "Credentials Payload"
// @Success 200 {object} response.Envelope{data=LoginResponse}
// @Failure 400 {object} response.Envelope{error=response.APIError}
// @Failure 401 {object} response.Envelope{error=response.APIError}
// @Router /api/v1/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var in LoginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_REQUEST_BODY", "malformed or missing JSON request body")
		return
	}

	res, err := h.service.Login(c.Request.Context(), in)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.OK(c, res)
}

// Refresh godoc
// @Summary Refresh expired access token using refresh token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RefreshInput true "Refresh Token Payload"
// @Success 200 {object} response.Envelope{data=TokenPair}
// @Failure 400 {object} response.Envelope{error=response.APIError}
// @Failure 401 {object} response.Envelope{error=response.APIError}
// @Router /api/v1/auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var in RefreshInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_REQUEST_BODY", "malformed or missing JSON request body")
		return
	}

	tokens, err := h.service.RefreshToken(c.Request.Context(), in.RefreshToken)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.OK(c, tokens)
}

// GetProfile godoc
// @Summary Get current user profile
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.Envelope{data=UserResponse}
// @Failure 401 {object} response.Envelope{error=response.APIError}
// @Failure 404 {object} response.Envelope{error=response.APIError}
// @Router /api/v1/users/me [get]
func (h *Handler) GetProfile(c *gin.Context) {
	val, exists := c.Get(ContextUserID)
	if !exists {
		response.Unauthorized(c, response.ErrCodeUnauthorized, "unauthenticated user context")
		return
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		response.Unauthorized(c, response.ErrCodeUnauthorized, "invalid user context")
		return
	}

	res, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.OK(c, res)
}
