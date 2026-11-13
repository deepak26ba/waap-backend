package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"waap-backend/pkg/db"
	"waap-backend/pkg/response"
)

const (
	DefaultAccessTTL  = 15 * time.Minute
	DefaultRefreshTTL = 7 * 24 * time.Hour
)

// UserResponse represents safe, serialized user profile data.
type UserResponse struct {
	ID          int64     `json:"id"`
	Email       string    `json:"email"`
	FullName    string    `json:"full_name"`
	Role        string    `json:"role"`
	PhoneNumber *string   `json:"phone_number,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// LoginResponse contains authentication tokens and the authenticated user's profile.
type LoginResponse struct {
	TokenPair
	User UserResponse `json:"user"`
}

// RegisterInput models the registration request payload.
type RegisterInput struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	FullName    string  `json:"full_name"`
	PhoneNumber *string `json:"phone_number,omitempty"`
}

// LoginInput models the login request credentials.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Service defines the business logic contract for authentication.
type Service interface {
	Register(ctx context.Context, in RegisterInput) (*UserResponse, error)
	Login(ctx context.Context, in LoginInput) (*LoginResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
}

type authService struct {
	queries    db.Querier
	jwtSecret  string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewService constructs an instance of the authentication service.
func NewService(queries db.Querier, jwtSecret string, accessTTL, refreshTTL time.Duration) Service {
	if accessTTL <= 0 {
		accessTTL = DefaultAccessTTL
	}
	if refreshTTL <= 0 {
		refreshTTL = DefaultRefreshTTL
	}
	return &authService{
		queries:    queries,
		jwtSecret:  jwtSecret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// Register handles validation, Bcrypt password hashing (cost 12), and database insertion.
func (s *authService) Register(ctx context.Context, in RegisterInput) (*UserResponse, error) {
	// 1. Sanitize and validate inputs
	rawEmail := strings.TrimSpace(in.Email)
	if rawEmail == "" {
		return nil, response.NewValidationError("validation failed", []response.FieldError{
			{Field: "email", Message: "email is required"},
		})
	}

	// Strictly validate bare mailbox format (rejecting display-name formats like "Name <email>")
	parsedAddr, err := mail.ParseAddress(rawEmail)
	if err != nil || parsedAddr.Address != rawEmail {
		return nil, response.NewValidationError("validation failed", []response.FieldError{
			{Field: "email", Message: "invalid email format"},
		})
	}
	email := strings.ToLower(parsedAddr.Address)

	fullName := strings.TrimSpace(in.FullName)
	if fullName == "" {
		return nil, response.NewValidationError("validation failed", []response.FieldError{
			{Field: "full_name", Message: "full name is required"},
		})
	}

	passBytes := []byte(in.Password)
	if len(passBytes) < 8 || len(passBytes) > 72 {
		return nil, response.NewValidationError("validation failed", []response.FieldError{
			{Field: "password", Message: "password must be between 8 and 72 characters long"},
		})
	}

	// 2. Pre-check for duplicate email
	_, err = s.queries.GetUserByEmail(ctx, email)
	if err == nil {
		return nil, response.NewConflictError("EMAIL_ALREADY_EXISTS", "a user with this email address already exists")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		// Log or forward unexpected query error if not ErrNoRows
	}

	// 3. Hash password via Bcrypt cost 12
	hashedPassword, err := HashPassword(in.Password)
	if err != nil {
		return nil, response.NewInternalError(err)
	}

	// 4. Create user in PostgreSQL
	phoneText := pgtype.Text{Valid: false}
	if in.PhoneNumber != nil && strings.TrimSpace(*in.PhoneNumber) != "" {
		phoneText = pgtype.Text{String: strings.TrimSpace(*in.PhoneNumber), Valid: true}
	}

	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        email,
		PasswordHash: hashedPassword,
		FullName:     fullName,
		Role:         "customer",
		PhoneNumber:  phoneText,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, response.NewConflictError("EMAIL_ALREADY_EXISTS", "a user with this email address already exists")
		}
		return nil, response.NewInternalError(err)
	}

	var phone *string
	if user.PhoneNumber.Valid {
		p := user.PhoneNumber.String
		phone = &p
	}

	return &UserResponse{
		ID:          user.ID,
		Email:       user.Email,
		FullName:    user.FullName,
		Role:        user.Role,
		PhoneNumber: phone,
		CreatedAt:   user.CreatedAt.Time,
	}, nil
}

// Login validates user credentials and issues signed JWT access and refresh tokens.
func (s *authService) Login(ctx context.Context, in LoginInput) (*LoginResponse, error) {
	rawEmail := strings.TrimSpace(in.Email)
	if rawEmail == "" || in.Password == "" {
		return nil, response.NewBadRequestError("INVALID_INPUT", "email and password are required")
	}

	parsedAddr, err := mail.ParseAddress(rawEmail)
	if err != nil || parsedAddr.Address != rawEmail {
		return nil, response.NewUnauthorizedError("invalid email or password")
	}
	email := strings.ToLower(parsedAddr.Address)

	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Defend against timing enumeration: run constant-time bcrypt compare
			CheckDummyPassword(in.Password)
			return nil, response.NewUnauthorizedError("invalid email or password")
		}
		return nil, response.NewInternalError(err)
	}

	// Verify provided password against stored bcrypt hash first (prevents account status enumeration)
	if err := CheckPassword(user.PasswordHash, in.Password); err != nil {
		return nil, response.NewUnauthorizedError("invalid email or password")
	}

	if !user.IsActive {
		return nil, response.NewForbiddenError("account has been deactivated")
	}

	// Issue JWT token pair
	tokens, err := GenerateTokenPair(user.ID, user.Email, user.Role, s.jwtSecret, s.accessTTL, s.refreshTTL)
	if err != nil {
		return nil, response.NewInternalError(err)
	}

	var phone *string
	if user.PhoneNumber.Valid {
		p := user.PhoneNumber.String
		phone = &p
	}

	return &LoginResponse{
		TokenPair: *tokens,
		User: UserResponse{
			ID:          user.ID,
			Email:       user.Email,
			FullName:    user.FullName,
			Role:        user.Role,
			PhoneNumber: phone,
			CreatedAt:   user.CreatedAt.Time,
		},
	}, nil
}

// RefreshToken validates a refresh token and issues a fresh token pair.
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	if refreshToken == "" {
		return nil, response.NewBadRequestError("INVALID_TOKEN", "refresh token is required")
	}

	claims, err := ValidateToken(refreshToken, s.jwtSecret)
	if err != nil {
		return nil, response.NewUnauthorizedError("invalid or expired refresh token")
	}

	if claims.TokenType != TokenTypeRefresh {
		return nil, response.NewUnauthorizedError("provided token is not a refresh token")
	}

	user, err := s.queries.GetUserByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, response.NewUnauthorizedError("user associated with token not found")
		}
		return nil, response.NewInternalError(err)
	}

	if !user.IsActive {
		return nil, response.NewForbiddenError("account has been deactivated")
	}

	return GenerateTokenPair(user.ID, user.Email, user.Role, s.jwtSecret, s.accessTTL, s.refreshTTL)
}
