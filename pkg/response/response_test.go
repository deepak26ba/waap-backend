package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func executeRequest(handler gin.HandlerFunc) (*httptest.ResponseRecorder, Envelope) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	c.Request = req

	handler(c)

	var env Envelope
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &env)
	}
	return w, env
}

func TestNewPagination(t *testing.T) {
	tests := []struct {
		name       string
		page       int
		perPage    int
		totalItems int64
		wantPages  int
		wantPage   int
		wantPerP   int
	}{
		{
			name:       "exact division",
			page:       1,
			perPage:    10,
			totalItems: 30,
			wantPages:  3,
			wantPage:   1,
			wantPerP:   10,
		},
		{
			name:       "remainder division",
			page:       2,
			perPage:    10,
			totalItems: 25,
			wantPages:  3,
			wantPage:   2,
			wantPerP:   10,
		},
		{
			name:       "zero items",
			page:       1,
			perPage:    10,
			totalItems: 0,
			wantPages:  0,
			wantPage:   1,
			wantPerP:   10,
		},
		{
			name:       "defaults for invalid input",
			page:       0,
			perPage:    -5,
			totalItems: 45,
			wantPages:  3,
			wantPage:   1,
			wantPerP:   20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.perPage, tt.totalItems)
			if p.TotalPages != tt.wantPages {
				t.Errorf("TotalPages = %d, want %d", p.TotalPages, tt.wantPages)
			}
			if p.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", p.Page, tt.wantPage)
			}
			if p.PerPage != tt.wantPerP {
				t.Errorf("PerPage = %d, want %d", p.PerPage, tt.wantPerP)
			}
		})
	}
}

func TestSuccessResponses(t *testing.T) {
	t.Run("Success / OK", func(t *testing.T) {
		payload := map[string]string{"message": "all good"}
		w, env := executeRequest(func(c *gin.Context) {
			Success(c, payload)
		})

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
		if !env.Success {
			t.Errorf("expected success true")
		}
		dataMap, ok := env.Data.(map[string]interface{})
		if !ok || dataMap["message"] != "all good" {
			t.Errorf("unexpected data: %v", env.Data)
		}
		if env.Error != nil {
			t.Errorf("expected error to be nil")
		}
	})

	t.Run("Created", func(t *testing.T) {
		payload := map[string]int{"id": 42}
		w, env := executeRequest(func(c *gin.Context) {
			Created(c, payload)
		})

		if w.Code != http.StatusCreated {
			t.Errorf("status = %d, want 201", w.Code)
		}
		if !env.Success {
			t.Errorf("expected success true")
		}
	})

	t.Run("NoContent", func(t *testing.T) {
		w, _ := executeRequest(func(c *gin.Context) {
			NoContent(c)
		})

		if w.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", w.Code)
		}
		if w.Body.Len() != 0 {
			t.Errorf("expected empty body, got %s", w.Body.String())
		}
	})

	t.Run("Paginated", func(t *testing.T) {
		items := []string{"item1", "item2"}
		pagination := NewPagination(1, 10, 2)

		w, env := executeRequest(func(c *gin.Context) {
			Paginated(c, items, pagination)
		})

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
		if !env.Success {
			t.Errorf("expected success true")
		}
		if env.Pagination == nil || env.Pagination.TotalItems != 2 {
			t.Errorf("unexpected pagination: %+v", env.Pagination)
		}
	})
}

func TestErrorResponses(t *testing.T) {
	tests := []struct {
		name       string
		handler    gin.HandlerFunc
		wantStatus int
		wantCode   string
	}{
		{
			name: "BadRequest",
			handler: func(c *gin.Context) {
				BadRequest(c, "", "bad request syntax")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeBadRequest,
		},
		{
			name: "ValidationError",
			handler: func(c *gin.Context) {
				ValidationError(c, "", []FieldError{{Field: "email", Message: "invalid email format"}})
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   ErrCodeValidationFailed,
		},
		{
			name: "Unauthorized",
			handler: func(c *gin.Context) {
				Unauthorized(c, "", "")
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   ErrCodeUnauthorized,
		},
		{
			name: "Forbidden",
			handler: func(c *gin.Context) {
				Forbidden(c, "", "")
			},
			wantStatus: http.StatusForbidden,
			wantCode:   ErrCodeForbidden,
		},
		{
			name: "NotFound",
			handler: func(c *gin.Context) {
				NotFound(c, "", "")
			},
			wantStatus: http.StatusNotFound,
			wantCode:   ErrCodeNotFound,
		},
		{
			name: "Conflict",
			handler: func(c *gin.Context) {
				Conflict(c, "EMAIL_EXISTS", "email is already taken")
			},
			wantStatus: http.StatusConflict,
			wantCode:   "EMAIL_EXISTS",
		},
		{
			name: "PayloadTooLarge",
			handler: func(c *gin.Context) {
				PayloadTooLarge(c, "", "")
			},
			wantStatus: http.StatusRequestEntityTooLarge,
			wantCode:   ErrCodePayloadTooLarge,
		},
		{
			name: "TooManyRequests",
			handler: func(c *gin.Context) {
				TooManyRequests(c, "", "")
			},
			wantStatus: http.StatusTooManyRequests,
			wantCode:   ErrCodeTooManyRequests,
		},
		{
			name: "InternalServerError",
			handler: func(c *gin.Context) {
				InternalServerError(c, "", "")
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   ErrCodeInternalServerError,
		},
		{
			name: "ServiceUnavailable",
			handler: func(c *gin.Context) {
				ServiceUnavailable(c, "", "")
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   ErrCodeServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, env := executeRequest(tt.handler)
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if env.Success {
				t.Errorf("expected success false")
			}
			if env.Error == nil || env.Error.Code != tt.wantCode {
				t.Errorf("error code = %v, want %s", env.Error, tt.wantCode)
			}
		})
	}
}

func TestAppErrorAndHandleError(t *testing.T) {
	t.Run("AppError Error formatting and unwrap", func(t *testing.T) {
		baseErr := errors.New("underlying db error")
		appErr := NewInternalError(baseErr)

		if !errors.Is(appErr, baseErr) {
			t.Errorf("expected appErr to wrap baseErr")
		}
		if appErr.Error() == "" {
			t.Errorf("expected non-empty error string")
		}
	})

	t.Run("HandleError with AppError", func(t *testing.T) {
		appErr := NewNotFoundError("product not found")
		w, env := executeRequest(func(c *gin.Context) {
			HandleError(c, appErr)
		})

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
		if env.Error == nil || env.Error.Code != ErrCodeNotFound {
			t.Errorf("unexpected error: %+v", env.Error)
		}
	})

	t.Run("HandleError with ValidationError and Details", func(t *testing.T) {
		appErr := NewValidationError("validation failed", []FieldError{
			{Field: "password", Message: "must be at least 8 characters"},
		})
		w, env := executeRequest(func(c *gin.Context) {
			HandleError(c, appErr)
		})

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want 422", w.Code)
		}
		if env.Error == nil || len(env.Error.Details) != 1 {
			t.Fatalf("expected 1 error detail, got %+v", env.Error)
		}
		if env.Error.Details[0].Field != "password" {
			t.Errorf("expected field password, got %s", env.Error.Details[0].Field)
		}
	})

	t.Run("HandleError with generic error", func(t *testing.T) {
		err := errors.New("random unexpected error")
		w, env := executeRequest(func(c *gin.Context) {
			HandleError(c, err)
		})

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
		if env.Error == nil || env.Error.Code != ErrCodeInternalServerError {
			t.Errorf("unexpected error: %+v", env.Error)
		}
	})

	t.Run("HandleError with nil error", func(t *testing.T) {
		w, _ := executeRequest(func(c *gin.Context) {
			HandleError(c, nil)
		})

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})
}

