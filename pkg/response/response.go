package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Pagination struct {
	Page       int   `json:"page,omitempty"`
	PerPage    int   `json:"per_page,omitempty"`
	TotalItems int64 `json:"total_items,omitempty"`
	TotalPages int   `json:"total_pages,omitempty"`
}

type Envelope struct {
	Success    bool        `json:"success"`
	Data       interface{} `json:"data,omitempty"`
	Error      *APIError   `json:"error,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

func JSON(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, Envelope{
		Success: statusCode >= 200 && statusCode < 300,
		Data:    data,
	})
}

func Success(c *gin.Context, data interface{}) {
	JSON(c, http.StatusOK, data)
}

func Created(c *gin.Context, data interface{}) {
	JSON(c, http.StatusCreated, data)
}

func Error(c *gin.Context, statusCode int, code, message string) {
	c.JSON(statusCode, Envelope{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

