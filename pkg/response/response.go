package response

import (
	"errors"
	"fmt"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Standard error codes used across the API.
const (
	ErrCodeInternalServerError = "INTERNAL_SERVER_ERROR"
	ErrCodeBadRequest          = "BAD_REQUEST"
	ErrCodeValidationFailed    = "VALIDATION_FAILED"
	ErrCodeUnauthorized        = "UNAUTHORIZED"
	ErrCodeForbidden           = "FORBIDDEN"
	ErrCodeNotFound            = "NOT_FOUND"
	ErrCodeConflict            = "CONFLICT"
	ErrCodePayloadTooLarge     = "PAYLOAD_TOO_LARGE"
	ErrCodeTooManyRequests     = "TOO_MANY_REQUESTS"
	ErrCodeServiceUnavailable  = "SERVICE_UNAVAILABLE"
)

// FieldError represents a specific validation failure on a request field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// APIError defines the standard error payload in the response envelope.
type APIError struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

// Pagination represents pagination metadata for collection endpoints.
type Pagination struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

// NewPagination calculates total pages and creates a new Pagination instance.
func NewPagination(page, perPage int, totalItems int64) *Pagination {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = int(math.Ceil(float64(totalItems) / float64(perPage)))
	}

	return &Pagination{
		Page:       page,
		PerPage:    perPage,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
}

// Envelope is the unified JSON response envelope for all API endpoints.
type Envelope struct {
	Success    bool        `json:"success"`
	Data       interface{} `json:"data,omitempty"`
	Error      *APIError   `json:"error,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// AppError is an application-level error carrying HTTP status and structured error details.
type AppError struct {
	StatusCode int
	Code       string
	Message    string
	Details    []FieldError
	Err        error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// AppError constructor functions.
func NewAppError(statusCode int, code, message string) *AppError {
	return &AppError{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
	}
}

func NewValidationError(message string, details []FieldError) *AppError {
	return &AppError{
		StatusCode: http.StatusUnprocessableEntity,
		Code:       ErrCodeValidationFailed,
		Message:    message,
		Details:    details,
	}
}

func NewBadRequestError(code, message string) *AppError {
	if code == "" {
		code = ErrCodeBadRequest
	}
	return &AppError{
		StatusCode: http.StatusBadRequest,
		Code:       code,
		Message:    message,
	}
}

func NewUnauthorizedError(message string) *AppError {
	if message == "" {
		message = "authentication is required to access this resource"
	}
	return &AppError{
		StatusCode: http.StatusUnauthorized,
		Code:       ErrCodeUnauthorized,
		Message:    message,
	}
}

func NewForbiddenError(message string) *AppError {
	if message == "" {
		message = "you do not have permission to perform this action"
	}
	return &AppError{
		StatusCode: http.StatusForbidden,
		Code:       ErrCodeForbidden,
		Message:    message,
	}
}

func NewNotFoundError(message string) *AppError {
	if message == "" {
		message = "requested resource was not found"
	}
	return &AppError{
		StatusCode: http.StatusNotFound,
		Code:       ErrCodeNotFound,
		Message:    message,
	}
}

func NewConflictError(code, message string) *AppError {
	if code == "" {
		code = ErrCodeConflict
	}
	return &AppError{
		StatusCode: http.StatusConflict,
		Code:       code,
		Message:    message,
	}
}

func NewInternalError(err error) *AppError {
	return &AppError{
		StatusCode: http.StatusInternalServerError,
		Code:       ErrCodeInternalServerError,
		Message:    "an unexpected internal server error occurred",
		Err:        err,
	}
}

// Success responses.

// JSON writes a generic HTTP response using the standard Envelope.
func JSON(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, Envelope{
		Success: statusCode >= 200 && statusCode < 300,
		Data:    data,
	})
}

// Success writes an HTTP 200 OK response with the provided payload.
func Success(c *gin.Context, data interface{}) {
	JSON(c, http.StatusOK, data)
}

// OK is an alias for Success.
func OK(c *gin.Context, data interface{}) {
	Success(c, data)
}

// Created writes an HTTP 201 Created response.
func Created(c *gin.Context, data interface{}) {
	JSON(c, http.StatusCreated, data)
}

// NoContent writes an HTTP 204 No Content response and aborts further processing.
func NoContent(c *gin.Context) {
	c.AbortWithStatus(http.StatusNoContent)
}

// Paginated writes an HTTP 200 OK response with data and pagination metadata.
func Paginated(c *gin.Context, data interface{}, pagination *Pagination) {
	c.JSON(http.StatusOK, Envelope{
		Success:    true,
		Data:       data,
		Pagination: pagination,
	})
}

// Error responses.

// Error writes an error response envelope with custom status code, code, and message.
func Error(c *gin.Context, statusCode int, code, message string) {
	c.AbortWithStatusJSON(statusCode, Envelope{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

// ErrorWithDetails writes an error response envelope including field-level validation details.
func ErrorWithDetails(c *gin.Context, statusCode int, code, message string, details []FieldError) {
	c.AbortWithStatusJSON(statusCode, Envelope{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// BadRequest writes an HTTP 400 Bad Request error.
func BadRequest(c *gin.Context, code, message string) {
	if code == "" {
		code = ErrCodeBadRequest
	}
	Error(c, http.StatusBadRequest, code, message)
}

// ValidationError writes an HTTP 422 Unprocessable Entity error with validation field failures.
func ValidationError(c *gin.Context, message string, details []FieldError) {
	if message == "" {
		message = "validation failed for one or more fields"
	}
	ErrorWithDetails(c, http.StatusUnprocessableEntity, ErrCodeValidationFailed, message, details)
}

// Unauthorized writes an HTTP 401 Unauthorized error.
func Unauthorized(c *gin.Context, code, message string) {
	if code == "" {
		code = ErrCodeUnauthorized
	}
	if message == "" {
		message = "authentication required"
	}
	Error(c, http.StatusUnauthorized, code, message)
}

// Forbidden writes an HTTP 403 Forbidden error.
func Forbidden(c *gin.Context, code, message string) {
	if code == "" {
		code = ErrCodeForbidden
	}
	if message == "" {
		message = "access denied"
	}
	Error(c, http.StatusForbidden, code, message)
}

// NotFound writes an HTTP 404 Not Found error.
func NotFound(c *gin.Context, code, message string) {
	if code == "" {
		code = ErrCodeNotFound
	}
	if message == "" {
		message = "resource not found"
	}
	Error(c, http.StatusNotFound, code, message)
}

// Conflict writes an HTTP 409 Conflict error.
func Conflict(c *gin.Context, code, message string) {
	if code == "" {
		code = ErrCodeConflict
	}
	Error(c, http.StatusConflict, code, message)
}

// PayloadTooLarge writes an HTTP 413 Payload Too Large error.
func PayloadTooLarge(c *gin.Context, code, message string) {
	if code == "" {
		code = ErrCodePayloadTooLarge
	}
	if message == "" {
		message = "request payload exceeds maximum allowed size"
	}
	Error(c, http.StatusRequestEntityTooLarge, code, message)
}

// TooManyRequests writes an HTTP 429 Too Many Requests error.
func TooManyRequests(c *gin.Context, code, message string) {
	if code == "" {
		code = ErrCodeTooManyRequests
	}
	if message == "" {
		message = "too many requests, please slow down"
	}
	Error(c, http.StatusTooManyRequests, code, message)
}

// InternalServerError writes an HTTP 500 Internal Server Error.
func InternalServerError(c *gin.Context, code, message string) {
	if code == "" {
		code = ErrCodeInternalServerError
	}
	if message == "" {
		message = "an unexpected internal server error occurred"
	}
	Error(c, http.StatusInternalServerError, code, message)
}

// ServiceUnavailable writes an HTTP 503 Service Unavailable error.
func ServiceUnavailable(c *gin.Context, code, message string) {
	if code == "" {
		code = ErrCodeServiceUnavailable
	}
	if message == "" {
		message = "service temporarily unavailable"
	}
	Error(c, http.StatusServiceUnavailable, code, message)
}

// HandleError evaluates an error and responds with the appropriate HTTP envelope.
// If the error is an *AppError, its status code and fields are preserved.
// Otherwise, it returns an HTTP 500 Internal Server Error.
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		if len(appErr.Details) > 0 {
			ErrorWithDetails(c, appErr.StatusCode, appErr.Code, appErr.Message, appErr.Details)
		} else {
			Error(c, appErr.StatusCode, appErr.Code, appErr.Message)
		}
		return
	}

	InternalServerError(c, ErrCodeInternalServerError, "an unexpected internal server error occurred")
}
