package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"waap-backend/internal/model"
	"waap-backend/internal/repository"
	"waap-backend/pkg/db"
	"waap-backend/pkg/response"
)

// AuthService defines the business logic contract for authentication.
type AuthService interface {
	Register(ctx context.Context, in model.RegisterInput) (*model.UserResponse, error)
	Login(ctx context.Context, in model.LoginInput) (*model.LoginResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*model.TokenPair, error)
}

type authService struct {
	userRepo   repository.UserRepository
	jwtSecret  string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewAuthService constructs an instance of the authentication service.
func NewAuthService(userRepo repository.UserRepository, jwtSecret string, accessTTL, refreshTTL time.Duration) AuthService {
	if accessTTL <= 0 {
		accessTTL = DefaultAccessTTL
	}
	if refreshTTL <= 0 {
		refreshTTL = DefaultRefreshTTL
	}
	return &authService{
		userRepo:   userRepo,
		jwtSecret:  jwtSecret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// Register handles validation, Bcrypt password hashing (cost 12), and database insertion.
func (s *authService) Register(ctx context.Context, in model.RegisterInput) (*model.UserResponse, error) {
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
	_, err = s.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return nil, response.NewConflictError("EMAIL_ALREADY_EXISTS", "a user with this email address already exists")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, response.NewInternalError(errors.New("failed to check existing user"))
	}

	// 3. Hash password using Bcrypt with cost 12
	hashedPassword, err := HashPassword(in.Password)
	if err != nil {
		return nil, response.NewInternalError(errors.New("failed to process credentials"))
	}

	// 4. Create user in database
	var phone pgtype.Text
	if in.PhoneNumber != nil && strings.TrimSpace(*in.PhoneNumber) != "" {
		phone = pgtype.Text{String: strings.TrimSpace(*in.PhoneNumber), Valid: true}
	}

	user, err := s.userRepo.Create(ctx, db.CreateUserParams{
		Email:        email,
		PasswordHash: hashedPassword,
		FullName:     fullName,
		Role:         "customer",
		PhoneNumber:  phone,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, response.NewConflictError("EMAIL_ALREADY_EXISTS", "a user with this email address already exists")
		}
		return nil, response.NewInternalError(errors.New("failed to register user"))
	}

	var resPhone *string
	if user.PhoneNumber.Valid {
		p := user.PhoneNumber.String
		resPhone = &p
	}

	return &model.UserResponse{
		ID:          user.ID,
		Email:       user.Email,
		FullName:    user.FullName,
		Role:        user.Role,
		PhoneNumber: resPhone,
		CreatedAt:   user.CreatedAt.Time,
	}, nil
}

// Login authenticates credentials, applies timing-safe dummy check on missing users, and returns JWT token pair.
func (s *authService) Login(ctx context.Context, in model.LoginInput) (*model.LoginResponse, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || in.Password == "" {
		return nil, response.NewUnauthorizedError("invalid email or password")
	}

	// 1. Fetch user by email
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Anti-enumeration: execute dummy bcrypt check to neutralize timing side-channels
			CheckDummyPassword(in.Password)
			return nil, response.NewUnauthorizedError("invalid email or password")
		}
		return nil, response.NewInternalError(errors.New("failed to verify credentials"))
	}

	// 2. Verify password
	if err := CheckPassword(user.PasswordHash, in.Password); err != nil {
		return nil, response.NewUnauthorizedError("invalid email or password")
	}

	// 3. Verify active status (only checked AFTER password matches to prevent enumeration of inactive accounts)
	if !user.IsActive {
		return nil, response.NewForbiddenError("user account is deactivated")
	}

	// 4. Issue dual tokens (15m access, 7d refresh)
	tokens, err := GenerateTokenPair(user.ID, user.Email, user.Role, s.jwtSecret, s.accessTTL, s.refreshTTL)
	if err != nil {
		return nil, response.NewInternalError(errors.New("failed to issue authentication tokens"))
	}

	var resPhone *string
	if user.PhoneNumber.Valid {
		p := user.PhoneNumber.String
		resPhone = &p
	}

	return &model.LoginResponse{
		TokenPair: *tokens,
		User: model.UserResponse{
			ID:          user.ID,
			Email:       user.Email,
			FullName:    user.FullName,
			Role:        user.Role,
			PhoneNumber: resPhone,
			CreatedAt:   user.CreatedAt.Time,
		},
	}, nil
}

// RefreshToken validates an active refresh token and returns a fresh token pair.
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*model.TokenPair, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, response.NewUnauthorizedError("refresh token is required")
	}

	// 1. Parse and validate claims
	claims, err := ValidateToken(refreshToken, s.jwtSecret)
	if err != nil {
		return nil, response.NewUnauthorizedError("invalid or expired refresh token")
	}

	// 2. Ensure token type is refresh
	if claims.TokenType != model.TokenTypeRefresh {
		return nil, response.NewUnauthorizedError("invalid token type: refresh token expected")
	}

	// 3. Verify user still exists and is active
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, response.NewUnauthorizedError("user account not found")
		}
		return nil, response.NewInternalError(errors.New("failed to verify user status"))
	}

	if !user.IsActive {
		return nil, response.NewForbiddenError("user account is deactivated")
	}

	// 4. Issue fresh token pair
	return GenerateTokenPair(user.ID, user.Email, user.Role, s.jwtSecret, s.accessTTL, s.refreshTTL)
}
