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
	tokensByHash map[string]db.EmailVerificationToken
	tokensByID   map[int64]db.EmailVerificationToken
	tokenSeq     int64
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		usersByEmail: make(map[string]db.User),
		usersByID:    make(map[uuid.UUID]db.User),
		tokensByHash: make(map[string]db.EmailVerificationToken),
		tokensByID:   make(map[int64]db.EmailVerificationToken),
	}
}

func (m *mockUserRepository) Create(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	if _, exists := m.usersByEmail[arg.Email]; exists {
		return db.User{}, errors.New("duplicate key value violates unique constraint \"users_email_key\"")
	}

	newID := uuid.Must(uuid.NewV4())
	u := db.User{
		ID:            newID,
		Email:         arg.Email,
		PasswordHash:  arg.PasswordHash,
		FullName:      arg.FullName,
		Role:          arg.Role,
		PhoneNumber:   arg.PhoneNumber,
		IsActive:      true,
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
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

func (m *mockUserRepository) CreateVerificationToken(ctx context.Context, arg db.CreateEmailVerificationTokenParams) (db.EmailVerificationToken, error) {
	m.tokenSeq++
	tok := db.EmailVerificationToken{
		ID:        m.tokenSeq,
		UserID:    arg.UserID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.tokensByHash[arg.TokenHash] = tok
	m.tokensByID[tok.ID] = tok
	return tok, nil
}

func (m *mockUserRepository) GetVerificationTokenByHash(ctx context.Context, tokenHash string) (db.EmailVerificationToken, error) {
	tok, ok := m.tokensByHash[tokenHash]
	if !ok {
		return db.EmailVerificationToken{}, pgx.ErrNoRows
	}
	return tok, nil
}

func (m *mockUserRepository) MarkVerificationTokenUsed(ctx context.Context, id int64) (db.EmailVerificationToken, error) {
	tok, ok := m.tokensByID[id]
	if !ok {
		return db.EmailVerificationToken{}, pgx.ErrNoRows
	}
	tok.UsedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.tokensByID[id] = tok
	m.tokensByHash[tok.TokenHash] = tok
	return tok, nil
}

func (m *mockUserRepository) MarkUserEmailVerified(ctx context.Context, id uuid.UUID) (db.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	u.EmailVerified = true
	m.usersByID[id] = u
	m.usersByEmail[u.Email] = u
	return u, nil
}

func (m *mockUserRepository) VerifyEmailTx(ctx context.Context, tokenID int64, userID uuid.UUID) error {
	if _, err := m.MarkVerificationTokenUsed(ctx, tokenID); err != nil {
		return err
	}
	if _, err := m.MarkUserEmailVerified(ctx, userID); err != nil {
		return err
	}
	return nil
}

type mockEmailSender struct {
	sentEmails []struct {
		Email, Name, Token string
	}
	err error
}

func (m *mockEmailSender) SendVerificationEmail(ctx context.Context, toEmail, toName, token string) error {
	if m.err != nil {
		return m.err
	}
	m.sentEmails = append(m.sentEmails, struct{ Email, Name, Token string }{
		Email: toEmail,
		Name:  toName,
		Token: token,
	})
	return nil
}

var _ Repository = (*mockUserRepository)(nil)

func TestUserService_Register(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	mailer := &mockEmailSender{}
	svc := NewService(mock, mailer, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	t.Run("Successfully registers customer with valid payload and sends email", func(t *testing.T) {
		phone := "+1234567890"
		in := RegisterInput{
			Email:       "test@example.com",
			Password:    "SecureP@ssw0rd123",
			FullName:    "John Doe",
			PhoneNumber: &phone,
		}

		res, err := svc.Register(ctx, in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res == nil {
			t.Fatal("expected non-nil response")
		}
		if res.Email != "test@example.com" {
			t.Errorf("expected email test@example.com, got %s", res.Email)
		}
		if res.FullName != "John Doe" {
			t.Errorf("expected full name John Doe, got %s", res.FullName)
		}
		if res.Role != "customer" {
			t.Errorf("expected role customer, got %s", res.Role)
		}
		if res.PhoneNumber == nil || *res.PhoneNumber != phone {
			t.Errorf("expected phone %s, got %v", phone, res.PhoneNumber)
		}
		if res.EmailVerified {
			t.Error("expected newly registered user to not be verified yet")
		}

		// Verify email was sent
		if len(mailer.sentEmails) != 1 {
			t.Fatalf("expected 1 sent email, got %d", len(mailer.sentEmails))
		}
		sent := mailer.sentEmails[0]
		if sent.Email != "test@example.com" {
			t.Errorf("expected recipient test@example.com, got %s", sent.Email)
		}
		if sent.Token == "" {
			t.Error("expected non-empty verification token in email")
		}

		// Verify token was stored hashed in repository
		if len(mock.tokensByHash) != 1 {
			t.Fatalf("expected 1 token stored, got %d", len(mock.tokensByHash))
		}
		expectedHash := hashVerificationToken(sent.Token)
		tok, exists := mock.tokensByHash[expectedHash]
		if !exists {
			t.Errorf("expected token hash %s to exist in repository", expectedHash)
		}
		if tok.UserID != res.ID {
			t.Errorf("expected token user ID %s, got %s", res.ID, tok.UserID)
		}
	})

	t.Run("Rejects registration when email already exists", func(t *testing.T) {
		in := RegisterInput{
			Email:    "test@example.com",
			Password: "AnotherPassword123",
			FullName: "Jane Doe",
		}

		_, err := svc.Register(ctx, in)
		if err == nil {
			t.Fatal("expected conflict error, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 409 {
			t.Errorf("expected status 409, got %d", apiErr.StatusCode)
		}
		if apiErr.Code != "EMAIL_ALREADY_EXISTS" {
			t.Errorf("expected code EMAIL_ALREADY_EXISTS, got %s", apiErr.Code)
		}
	})

	t.Run("Validates invalid or missing fields", func(t *testing.T) {
		cases := []struct {
			name        string
			input       RegisterInput
			expectField string
		}{
			{
				name:        "Empty email",
				input:       RegisterInput{Email: "", Password: "ValidPass123", FullName: "John"},
				expectField: "email",
			},
			{
				name:        "Invalid email format",
				input:       RegisterInput{Email: "not-an-email", Password: "ValidPass123", FullName: "John"},
				expectField: "email",
			},
			{
				name:        "Empty full name",
				input:       RegisterInput{Email: "valid@example.com", Password: "ValidPass123", FullName: "   "},
				expectField: "full_name",
			},
			{
				name:        "Password too short",
				input:       RegisterInput{Email: "valid@example.com", Password: "short", FullName: "John"},
				expectField: "password",
			},
			{
				name:        "Password too long (>72 bytes)",
				input:       RegisterInput{Email: "valid@example.com", Password: strings.Repeat("a", 73), FullName: "John"},
				expectField: "password",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := svc.Register(ctx, tc.input)
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}
				var apiErr *response.AppError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected AppError, got %T", err)
				}
				if apiErr.StatusCode != 422 {
					t.Errorf("expected status 422, got %d", apiErr.StatusCode)
				}
			})
		}
	})
}

func TestUserService_EmailVerification(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	mailer := &mockEmailSender{}
	svc := NewService(mock, mailer, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	// Register user
	regRes, err := svc.Register(ctx, RegisterInput{
		Email:    "verify@example.com",
		Password: "ValidPassword123",
		FullName: "Verify User",
	})
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	if len(mailer.sentEmails) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(mailer.sentEmails))
	}
	token := mailer.sentEmails[0].Token

	t.Run("Valid token marks user as email verified", func(t *testing.T) {
		err := svc.VerifyEmail(ctx, token)
		if err != nil {
			t.Fatalf("unexpected error verifying email: %v", err)
		}

		// Verify user status in repository
		u, err := mock.GetByID(ctx, regRes.ID)
		if err != nil {
			t.Fatalf("failed to get user: %v", err)
		}
		if !u.EmailVerified {
			t.Error("expected user email_verified to be true")
		}

		// Verify profile reflects email verified
		prof, err := svc.GetProfile(ctx, regRes.ID)
		if err != nil {
			t.Fatalf("failed to get profile: %v", err)
		}
		if !prof.EmailVerified {
			t.Error("expected profile email_verified to be true")
		}
	})

	t.Run("Token cannot be reused", func(t *testing.T) {
		err := svc.VerifyEmail(ctx, token)
		if err == nil {
			t.Fatal("expected error on token reuse, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 400 {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
		if apiErr.Code != "TOKEN_ALREADY_USED" {
			t.Errorf("expected code TOKEN_ALREADY_USED, got %s", apiErr.Code)
		}
	})

	t.Run("Invalid token is rejected", func(t *testing.T) {
		err := svc.VerifyEmail(ctx, "completely-invalid-nonexistent-token")
		if err == nil {
			t.Fatal("expected error for invalid token, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 400 {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
		if apiErr.Code != "INVALID_TOKEN" {
			t.Errorf("expected code INVALID_TOKEN, got %s", apiErr.Code)
		}
	})

	t.Run("Empty token is rejected", func(t *testing.T) {
		err := svc.VerifyEmail(ctx, "   ")
		if err == nil {
			t.Fatal("expected error for empty token, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 400 {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
	})

	t.Run("Expired token is rejected", func(t *testing.T) {
		// Register a user with expired token
		expiredUser, err := mock.Create(ctx, db.CreateUserParams{
			Email:        "expired@example.com",
			PasswordHash: "hashed",
			FullName:     "Expired User",
			Role:         "customer",
		})
		if err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		expiredRawToken := "expired-test-token-12345"
		expiredHash := hashVerificationToken(expiredRawToken)
		_, err = mock.CreateVerificationToken(ctx, db.CreateEmailVerificationTokenParams{
			UserID:    expiredUser.ID,
			TokenHash: expiredHash,
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		})
		if err != nil {
			t.Fatalf("failed to store token: %v", err)
		}

		err = svc.VerifyEmail(ctx, expiredRawToken)
		if err == nil {
			t.Fatal("expected error for expired token, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 400 {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
		if apiErr.Code != "TOKEN_EXPIRED" {
			t.Errorf("expected code TOKEN_EXPIRED, got %s", apiErr.Code)
		}
	})
}

func TestUserService_Login(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	svc := NewService(mock, nil, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	// Create user with known password
	password := "CorrectP@ss123"
	_, err := svc.Register(ctx, RegisterInput{
		Email:    "login@example.com",
		Password: password,
		FullName: "Login User",
	})
	if err != nil {
		t.Fatalf("failed to setup user: %v", err)
	}

	t.Run("Successful login returns tokens and profile", func(t *testing.T) {
		res, err := svc.Login(ctx, LoginInput{
			Email:    "login@example.com",
			Password: password,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res == nil {
			t.Fatal("expected non-nil response")
		}
		if res.AccessToken == "" || res.RefreshToken == "" {
			t.Error("expected non-empty access and refresh tokens")
		}
		if res.User.Email != "login@example.com" {
			t.Errorf("expected email login@example.com, got %s", res.User.Email)
		}
	})

	t.Run("Case-insensitive email login", func(t *testing.T) {
		res, err := svc.Login(ctx, LoginInput{
			Email:    "LOGIN@EXAMPLE.COM",
			Password: password,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.User.Email != "login@example.com" {
			t.Errorf("expected normalized email, got %s", res.User.Email)
		}
	})

	t.Run("Rejects incorrect password with 401", func(t *testing.T) {
		_, err := svc.Login(ctx, LoginInput{
			Email:    "login@example.com",
			Password: "WrongPassword!",
		})
		if err == nil {
			t.Fatal("expected 401 error, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 401 {
			t.Errorf("expected status 401, got %d", apiErr.StatusCode)
		}
	})

	t.Run("Rejects non-existent email with 401 (anti-enumeration)", func(t *testing.T) {
		_, err := svc.Login(ctx, LoginInput{
			Email:    "ghost@example.com",
			Password: "SomePassword123",
		})
		if err == nil {
			t.Fatal("expected 401 error, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 401 {
			t.Errorf("expected status 401, got %d", apiErr.StatusCode)
		}
	})

	t.Run("Rejects deactivated account with 403", func(t *testing.T) {
		u := mock.usersByEmail["login@example.com"]
		u.IsActive = false
		mock.usersByEmail["login@example.com"] = u

		_, err := svc.Login(ctx, LoginInput{
			Email:    "login@example.com",
			Password: password,
		})
		if err == nil {
			t.Fatal("expected 403 error, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 403 {
			t.Errorf("expected status 403, got %d", apiErr.StatusCode)
		}
	})
}

func TestUserService_RefreshToken(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	svc := NewService(mock, nil, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	password := "SecureP@ss123"
	loginRes, err := svc.Register(ctx, RegisterInput{
		Email:    "refresh@example.com",
		Password: password,
		FullName: "Refresh User",
	})
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	auth, err := svc.Login(ctx, LoginInput{
		Email:    "refresh@example.com",
		Password: password,
	})
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	t.Run("Valid refresh token produces fresh token pair", func(t *testing.T) {
		tokens, err := svc.RefreshToken(ctx, auth.RefreshToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tokens == nil {
			t.Fatal("expected non-nil tokens")
		}
		if tokens.AccessToken == "" || tokens.RefreshToken == "" {
			t.Error("expected non-empty token pair")
		}

		claims, err := ValidateToken(tokens.AccessToken, secret)
		if err != nil {
			t.Fatalf("expected valid access token: %v", err)
		}
		if claims.UserID != loginRes.ID {
			t.Errorf("expected user ID %s, got %s", loginRes.ID, claims.UserID)
		}
	})

	t.Run("Rejects access token used in place of refresh token", func(t *testing.T) {
		_, err := svc.RefreshToken(ctx, auth.AccessToken)
		if err == nil {
			t.Fatal("expected error when passing access token, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 401 {
			t.Errorf("expected status 401, got %d", apiErr.StatusCode)
		}
	})

	t.Run("Rejects empty refresh token", func(t *testing.T) {
		_, err := svc.RefreshToken(ctx, "   ")
		if err == nil {
			t.Fatal("expected error with empty refresh token, got nil")
		}
	})

	t.Run("Rejects invalid signature refresh token", func(t *testing.T) {
		badSecretTokens, _ := GenerateTokenPair(loginRes.ID, loginRes.Email, loginRes.Role, "wrong-secret-key", 15*time.Minute, 7*24*time.Hour)
		_, err := svc.RefreshToken(ctx, badSecretTokens.RefreshToken)
		if err == nil {
			t.Fatal("expected error with wrong signature token, got nil")
		}
	})

	t.Run("Rejects refresh when user is deactivated", func(t *testing.T) {
		u := mock.usersByEmail["refresh@example.com"]
		u.IsActive = false
		mock.usersByID[u.ID] = u

		_, err := svc.RefreshToken(ctx, auth.RefreshToken)
		if err == nil {
			t.Fatal("expected error for deactivated user, got nil")
		}
		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 403 {
			t.Errorf("expected status 403, got %d", apiErr.StatusCode)
		}
	})
}

func TestUserService_GetProfile(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	svc := NewService(mock, nil, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	reg, err := svc.Register(ctx, RegisterInput{
		Email:    "profile@example.com",
		Password: "Password123!",
		FullName: "Profile User",
	})
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	t.Run("Returns profile for existing user", func(t *testing.T) {
		prof, err := svc.GetProfile(ctx, reg.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prof.ID != reg.ID {
			t.Errorf("expected ID %s, got %s", reg.ID, prof.ID)
		}
		if prof.Email != "profile@example.com" {
			t.Errorf("expected email profile@example.com, got %s", prof.Email)
		}
	})

	t.Run("Returns 404 for non-existent user", func(t *testing.T) {
		missingID := uuid.Must(uuid.NewV4())
		_, err := svc.GetProfile(ctx, missingID)
		if err == nil {
			t.Fatal("expected error for missing user, got nil")
		}
		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 404 {
			t.Errorf("expected status 404, got %d", apiErr.StatusCode)
		}
	})
}
