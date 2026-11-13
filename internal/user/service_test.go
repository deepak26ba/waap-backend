package user

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"waap-backend/pkg/db"
	"waap-backend/pkg/response"
)

type mockUserRepository struct {
	usersByEmail map[string]db.User
	usersByID    map[uuid.UUID]db.User
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		usersByEmail: make(map[string]db.User),
		usersByID:    make(map[uuid.UUID]db.User),
	}
}

func (m *mockUserRepository) Create(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	if _, exists := m.usersByEmail[arg.Email]; exists {
		return db.User{}, errors.New("duplicate key value violates unique constraint \"users_email_key\"")
	}

	newID := uuid.Must(uuid.NewV4())
	u := db.User{
		ID:           newID,
		Email:        arg.Email,
		PasswordHash: arg.PasswordHash,
		FullName:     arg.FullName,
		Role:         arg.Role,
		PhoneNumber:  arg.PhoneNumber,
		IsActive:     true,
		CreatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.usersByEmail[u.Email] = u
	m.usersByID[u.ID] = u
	return u, nil
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (db.User, error) {
	u, exists := m.usersByEmail[email]
	if !exists {
		return db.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *mockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	u, exists := m.usersByID[id]
	if !exists {
		return db.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *mockUserRepository) Update(ctx context.Context, arg db.UpdateUserParams) (db.User, error) {
	u, exists := m.usersByID[arg.ID]
	if !exists {
		return db.User{}, pgx.ErrNoRows
	}
	if arg.FullName != "" {
		u.FullName = arg.FullName
	}
	if arg.PhoneNumber.Valid {
		u.PhoneNumber = arg.PhoneNumber
	}
	m.usersByID[u.ID] = u
	m.usersByEmail[u.Email] = u
	return u, nil
}

var _ Repository = (*mockUserRepository)(nil)

func TestUserService_Register(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	svc := NewService(mock, secret, 15*time.Minute, 7*24*time.Hour)

	t.Run("Successful Registration", func(t *testing.T) {
		phone := "+1234567890"
		in := RegisterInput{
			Email:       "test@example.com",
			Password:    "password123",
			FullName:    "John Doe",
			PhoneNumber: &phone,
		}

		res, err := svc.Register(ctx, in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.ID == uuid.Nil {
			t.Errorf("expected non-nil UUID")
		}
		if res.Email != "test@example.com" {
			t.Errorf("expected email test@example.com, got %s", res.Email)
		}
		if res.Role != "customer" {
			t.Errorf("expected default role customer, got %s", res.Role)
		}

		// Ensure password was actually hashed in DB
		savedUser := mock.usersByEmail["test@example.com"]
		if savedUser.PasswordHash == "password123" {
			t.Errorf("password must not be saved in plain text")
		}
		if err := CheckPassword(savedUser.PasswordHash, "password123"); err != nil {
			t.Errorf("saved hash does not verify: %v", err)
		}
	})

	t.Run("Duplicate Email Rejection (409 Conflict)", func(t *testing.T) {
		in := RegisterInput{
			Email:    "test@example.com", // already registered above
			Password: "newpassword123",
			FullName: "Jane Doe",
		}

		_, err := svc.Register(ctx, in)
		if err == nil {
			t.Fatalf("expected error on duplicate email, got nil")
		}

		var appErr *response.AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T: %v", err, err)
		}

		if appErr.StatusCode != 409 {
			t.Errorf("expected status 409, got %d", appErr.StatusCode)
		}
		if appErr.Code != "EMAIL_ALREADY_EXISTS" {
			t.Errorf("expected code EMAIL_ALREADY_EXISTS, got %s", appErr.Code)
		}
	})

	t.Run("Short Password Validation (< 8 chars)", func(t *testing.T) {
		in := RegisterInput{
			Email:    "short@example.com",
			Password: "short",
			FullName: "Short Pass",
		}

		_, err := svc.Register(ctx, in)
		if err == nil {
			t.Fatalf("expected validation error for password < 8 chars")
		}

		var appErr *response.AppError
		if !errors.As(err, &appErr) || appErr.StatusCode != 422 {
			t.Errorf("expected 422 Unprocessable Entity, got %v", err)
		}
	})

	t.Run("Long Password Validation (> 72 bytes)", func(t *testing.T) {
		in := RegisterInput{
			Email:    "longpass@example.com",
			Password: strings.Repeat("A", 73),
			FullName: "Long Pass",
		}

		_, err := svc.Register(ctx, in)
		if err == nil {
			t.Fatalf("expected validation error for password > 72 bytes")
		}

		var appErr *response.AppError
		if !errors.As(err, &appErr) || appErr.StatusCode != 422 {
			t.Errorf("expected 422 Unprocessable Entity, got %v", err)
		}
	})

	t.Run("Display Name In Email Format Rejected", func(t *testing.T) {
		in := RegisterInput{
			Email:    "John Doe <john@example.com>",
			Password: "validpassword123",
			FullName: "John Doe",
		}

		_, err := svc.Register(ctx, in)
		if err == nil {
			t.Fatalf("expected validation error for display-name email format")
		}

		var appErr *response.AppError
		if !errors.As(err, &appErr) || appErr.StatusCode != 422 {
			t.Errorf("expected 422 Unprocessable Entity, got %v", err)
		}
	})

	t.Run("Invalid Email Format", func(t *testing.T) {
		in := RegisterInput{
			Email:    "invalid-email-address",
			Password: "validpassword123",
			FullName: "Bad Email",
		}

		_, err := svc.Register(ctx, in)
		if err == nil {
			t.Fatalf("expected validation error for invalid email")
		}

		var appErr *response.AppError
		if !errors.As(err, &appErr) || appErr.StatusCode != 422 {
			t.Errorf("expected 422 Unprocessable Entity, got %v", err)
		}
	})
}

func TestUserService_Login(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	svc := NewService(mock, secret, 15*time.Minute, 7*24*time.Hour)

	// Pre-register user
	regIn := RegisterInput{
		Email:    "customer@example.com",
		Password: "SecurePassword123!",
		FullName: "Alice Customer",
	}
	_, err := svc.Register(ctx, regIn)
	if err != nil {
		t.Fatalf("failed to register test user: %v", err)
	}

	t.Run("Valid Login Credentials", func(t *testing.T) {
		in := LoginInput{
			Email:    "customer@example.com",
			Password: "SecurePassword123!",
		}

		res, err := svc.Login(ctx, in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.AccessToken == "" || res.RefreshToken == "" {
			t.Fatalf("expected non-empty tokens in login response")
		}
		if res.TokenType != "Bearer" {
			t.Errorf("expected Bearer token type, got %s", res.TokenType)
		}
		if res.User.Email != "customer@example.com" {
			t.Errorf("expected user email customer@example.com, got %s", res.User.Email)
		}

		// Verify returned access token contains expected claims
		claims, err := ValidateToken(res.AccessToken, secret)
		if err != nil {
			t.Fatalf("token validation failed: %v", err)
		}
		if claims.Role != "customer" {
			t.Errorf("expected role customer, got %s", claims.Role)
		}
	})

	t.Run("Invalid Password Returns 401", func(t *testing.T) {
		in := LoginInput{
			Email:    "customer@example.com",
			Password: "WrongPassword!",
		}

		_, err := svc.Login(ctx, in)
		if err == nil {
			t.Fatalf("expected error on wrong password")
		}

		var appErr *response.AppError
		if !errors.As(err, &appErr) || appErr.StatusCode != 401 {
			t.Errorf("expected 401 Unauthorized, got %v", err)
		}
	})

	t.Run("Non-Existent Email Returns 401", func(t *testing.T) {
		in := LoginInput{
			Email:    "nonexistent@example.com",
			Password: "AnyPassword123!",
		}

		_, err := svc.Login(ctx, in)
		if err == nil {
			t.Fatalf("expected error on non-existent user")
		}

		var appErr *response.AppError
		if !errors.As(err, &appErr) || appErr.StatusCode != 401 {
			t.Errorf("expected 401 Unauthorized, got %v", err)
		}
	})

	t.Run("Inactive User with Wrong Password Returns 401 (Prevent Status Enumeration)", func(t *testing.T) {
		u := mock.usersByEmail["customer@example.com"]
		u.IsActive = false
		mock.usersByEmail["customer@example.com"] = u

		in := LoginInput{
			Email:    "customer@example.com",
			Password: "WrongPassword123!",
		}

		_, err := svc.Login(ctx, in)
		if err == nil {
			t.Fatalf("expected error on inactive user with wrong password")
		}

		var appErr *response.AppError
		if !errors.As(err, &appErr) || appErr.StatusCode != 401 {
			t.Errorf("expected 401 Unauthorized (not 403), got %v", err)
		}
	})

	t.Run("Inactive User with Correct Password Returns 403", func(t *testing.T) {
		u := mock.usersByEmail["customer@example.com"]
		u.IsActive = false
		mock.usersByEmail["customer@example.com"] = u

		in := LoginInput{
			Email:    "customer@example.com",
			Password: "SecurePassword123!",
		}

		_, err := svc.Login(ctx, in)
		if err == nil {
			t.Fatalf("expected error on inactive user")
		}

		var appErr *response.AppError
		if !errors.As(err, &appErr) || appErr.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden, got %v", err)
		}
	})
}

func TestUserService_RefreshToken(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	svc := NewService(mock, secret, 15*time.Minute, 7*24*time.Hour)

	_, err := svc.Register(ctx, RegisterInput{
		Email:    "refresh@example.com",
		Password: "Password123!",
		FullName: "Refresh User",
	})
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	loginRes, err := svc.Login(ctx, LoginInput{
		Email:    "refresh@example.com",
		Password: "Password123!",
	})
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	t.Run("Successful Refresh", func(t *testing.T) {
		tokens, err := svc.RefreshToken(ctx, loginRes.RefreshToken)
		if err != nil {
			t.Fatalf("failed to refresh token: %v", err)
		}
		if tokens.AccessToken == "" || tokens.RefreshToken == "" {
			t.Errorf("expected valid new tokens")
		}
	})

	t.Run("Access Token Rejected as Refresh Token", func(t *testing.T) {
		_, err := svc.RefreshToken(ctx, loginRes.AccessToken)
		if err == nil {
			t.Fatalf("expected error when passing access token as refresh token")
		}
	})

	t.Run("Refresh Token with Unknown User Returns 401", func(t *testing.T) {
		unknownID := uuid.Must(uuid.NewV4())
		unknownTokens, err := GenerateTokenPair(unknownID, "unknown@test.com", "customer", secret, 15*time.Minute, 7*24*time.Hour)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = svc.RefreshToken(ctx, unknownTokens.RefreshToken)
		if err == nil {
			t.Fatalf("expected error for non-existent user refresh token")
		}

		var appErr *response.AppError
		if !errors.As(err, &appErr) || appErr.StatusCode != 401 {
			t.Errorf("expected 401 Unauthorized, got %v", err)
		}
	})
}

func TestPasswordHashingAndVerification(t *testing.T) {
	password := "SecretPassword123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if hash == password {
		t.Errorf("hash should not equal plaintext password")
	}

	if err := CheckPassword(hash, password); err != nil {
		t.Errorf("expected password match, got error: %v", err)
	}

	if err := CheckPassword(hash, "WrongPassword"); err == nil {
		t.Errorf("expected error on mismatched password, got nil")
	}
}

func TestCheckDummyPassword(t *testing.T) {
	CheckDummyPassword("arbitrary_password")
}

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "super-secure-jwt-test-secret-key-32b"
	userID := uuid.Must(uuid.NewV4())
	email := "user@example.com"
	role := "customer"
	accessTTL := 15 * time.Minute
	refreshTTL := 7 * 24 * time.Hour

	tokens, err := GenerateTokenPair(userID, email, role, secret, accessTTL, refreshTTL)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	claims, err := ValidateToken(tokens.AccessToken, secret)
	if err != nil {
		t.Fatalf("failed to validate access token: %v", err)
	}

	if claims.UserID != userID || claims.Email != email || claims.Role != role || claims.TokenType != TokenTypeAccess {
		t.Errorf("unexpected claims: %+v", claims)
	}

	refreshClaims, err := ValidateToken(tokens.RefreshToken, secret)
	if err != nil {
		t.Fatalf("failed to validate refresh token: %v", err)
	}
	if refreshClaims.TokenType != TokenTypeRefresh {
		t.Errorf("expected token_type %s, got %s", TokenTypeRefresh, refreshClaims.TokenType)
	}

	_, err = ValidateToken(tokens.AccessToken, "wrong-secret-key")
	if err == nil {
		t.Errorf("expected error when validating with wrong secret")
	}

	_, err = ValidateToken("not.a.valid.jwt", secret)
	if err == nil {
		t.Errorf("expected error when validating malformed token")
	}
}

func TestValidateExpiredToken(t *testing.T) {
	secret := "secret-key"
	userID := uuid.Must(uuid.NewV4())
	tokens, err := GenerateTokenPair(userID, "test@test.com", "customer", secret, -1*time.Minute, -1*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	_, err = ValidateToken(tokens.AccessToken, secret)
	if err == nil {
		t.Errorf("expected error when validating expired token")
	}
}

func TestService_GetProfile(t *testing.T) {
	repo := newMockUserRepository()
	svc := NewService(repo, "secret", 15*time.Minute, 24*time.Hour)
	ctx := context.Background()

	phone := "1234567890"
	created, err := repo.Create(ctx, db.CreateUserParams{
		Email:        "profile@example.com",
		PasswordHash: "hashed",
		FullName:     "Profile User",
		Role:         "customer",
		PhoneNumber:  pgtype.Text{String: phone, Valid: true},
	})
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	t.Run("Existing user returns profile", func(t *testing.T) {
		res, err := svc.GetProfile(ctx, created.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ID != created.ID || res.Email != "profile@example.com" || *res.PhoneNumber != phone {
			t.Errorf("profile fields mismatch: %+v", res)
		}
	})

	t.Run("Non-existent user returns 404", func(t *testing.T) {
		randomID := uuid.Must(uuid.NewV4())
		_, err := svc.GetProfile(ctx, randomID)
		if err == nil {
			t.Fatalf("expected error for non-existent user")
		}
	})
}
