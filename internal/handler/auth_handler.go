package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"waap-backend/internal/model"
	"waap-backend/internal/service"
	"waap-backend/pkg/response"
)

// AuthHandler exposes authentication HTTP endpoints.
type AuthHandler struct {
	authService service.AuthService
	log         *zap.Logger
}

// NewAuthHandler constructs an auth HTTP handler.
func NewAuthHandler(authService service.AuthService, log *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		log:         log,
	}
}

// RegisterRoutes registers auth endpoints on the provided router group.
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/register", h.Register)
	rg.POST("/login", h.Login)
	rg.POST("/refresh", h.Refresh)
}

// Register godoc
// @Summary Register a new customer account
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.RegisterInput true "Registration Payload"
// @Success 201 {object} response.Envelope{data=model.UserResponse}
// @Failure 400 {object} response.Envelope{error=response.APIError}
// @Failure 409 {object} response.Envelope{error=response.APIError}
// @Failure 422 {object} response.Envelope{error=response.APIError}
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var in model.RegisterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_REQUEST_BODY", "malformed or missing JSON request body")
		return
	}

	user, err := h.authService.Register(c.Request.Context(), in)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.Created(c, user)
}

// Login godoc
// @Summary Authenticate user and issue JWT token pair
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.LoginInput true "Credentials Payload"
// @Success 200 {object} response.Envelope{data=model.LoginResponse}
// @Failure 400 {object} response.Envelope{error=response.APIError}
// @Failure 401 {object} response.Envelope{error=response.APIError}
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var in model.LoginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_REQUEST_BODY", "malformed or missing JSON request body")
		return
	}

	res, err := h.authService.Login(c.Request.Context(), in)
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
// @Param request body model.RefreshInput true "Refresh Token Payload"
// @Success 200 {object} response.Envelope{data=model.TokenPair}
// @Failure 400 {object} response.Envelope{error=response.APIError}
// @Failure 401 {object} response.Envelope{error=response.APIError}
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var in model.RefreshInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_REQUEST_BODY", "malformed or missing JSON request body")
		return
	}

	tokens, err := h.authService.RefreshToken(c.Request.Context(), in.RefreshToken)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.OK(c, tokens)
}
