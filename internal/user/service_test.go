package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	u.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.usersByID[u.ID] = u
	m.usersByEmail[u.Email] = u
	return u, nil
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

func (m *mockUserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) (db.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	u.PasswordHash = passwordHash
	u.PasswordChangedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	u.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.usersByID[id] = u
	m.usersByEmail[u.Email] = u
	return u, nil
}

type mockEmailSender struct {
	mu              sync.Mutex
	sentEmails      []struct{ Email, Name, OTP string }
	sentResetEmails []struct{ Email, Name, Token string }
	err             error
}

func (m *mockEmailSender) SendVerificationEmail(ctx context.Context, toEmail, toName, otp string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sentEmails = append(m.sentEmails, struct{ Email, Name, OTP string }{
		Email: toEmail,
		Name:  toName,
		OTP:   otp,
	})
	return nil
}

func (m *mockEmailSender) SendPasswordResetEmail(ctx context.Context, toEmail, toName, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sentResetEmails = append(m.sentResetEmails, struct{ Email, Name, Token string }{
		Email: toEmail,
		Name:  toName,
		Token: token,
	})
	return nil
}

func (m *mockEmailSender) getSentResetEmails() []struct{ Email, Name, Token string } {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]struct{ Email, Name, Token string }, len(m.sentResetEmails))
	copy(res, m.sentResetEmails)
	return res
}

var _ Repository = (*mockUserRepository)(nil)

func TestUserService_Register(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	tokenStore := NewMemoryTokenStore(1 * time.Minute)
	defer tokenStore.Close()
	mailer := &mockEmailSender{}
	svc := NewService(mock, tokenStore, mailer, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

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
		if res.Message != "Registration successful. Please enter the OTP sent to your email to activate your account." {
			t.Errorf("unexpected message: %s", res.Message)
		}
		if res.User.Email != "test@example.com" {
			t.Errorf("expected email test@example.com, got %s", res.User.Email)
		}
		if res.User.FullName != "John Doe" {
			t.Errorf("expected full name John Doe, got %s", res.User.FullName)
		}
		if res.User.Role != "customer" {
			t.Errorf("expected role customer, got %s", res.User.Role)
		}
		if res.User.PhoneNumber == nil || *res.User.PhoneNumber != phone {
			t.Errorf("expected phone %s, got %v", phone, res.User.PhoneNumber)
		}
		if res.User.EmailVerified {
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
		if sent.OTP == "" || len(sent.OTP) != 6 {
			t.Errorf("expected 6-digit verification OTP in email, got %s", sent.OTP)
		}

		// Verify OTP was stored in memory store keyed by user ID
		expectedHash := hashOTP(sent.OTP)
		tokenStore.mu.RLock()
		otpItem, exists := tokenStore.otps[res.User.ID]
		tokenStore.mu.RUnlock()
		if !exists {
			t.Errorf("expected otp entry for user %s to exist in otp store", res.User.ID)
		}
		if otpItem.otpHash != expectedHash {
			t.Errorf("expected otp hash %s, got %s", expectedHash, otpItem.otpHash)
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
	tokenStore := NewMemoryTokenStore(1 * time.Minute)
	defer tokenStore.Close()
	mailer := &mockEmailSender{}
	svc := NewService(mock, tokenStore, mailer, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

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
	otp := mailer.sentEmails[0].OTP

	t.Run("Valid OTP and email marks user as email verified", func(t *testing.T) {
		err := svc.VerifyEmail(ctx, "verify@example.com", otp)
		if err != nil {
			t.Fatalf("unexpected error verifying email: %v", err)
		}

		// Verify user status in repository
		u, err := mock.GetByID(ctx, regRes.User.ID)
		if err != nil {
			t.Fatalf("failed to get user: %v", err)
		}
		if !u.EmailVerified {
			t.Error("expected user email_verified to be true")
		}

		// Verify profile reflects email verified
		prof, err := svc.GetProfile(ctx, regRes.User.ID)
		if err != nil {
			t.Fatalf("failed to get profile: %v", err)
		}
		if !prof.EmailVerified {
			t.Error("expected profile email_verified to be true")
		}
	})

	t.Run("OTP cannot be reused", func(t *testing.T) {
		err := svc.VerifyEmail(ctx, "verify@example.com", otp)
		if err == nil {
			t.Fatal("expected error on otp reuse, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 400 {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
		if apiErr.Code != "OTP_ALREADY_USED" {
			t.Errorf("expected code OTP_ALREADY_USED, got %s", apiErr.Code)
		}
	})

	t.Run("Invalid OTP is rejected", func(t *testing.T) {
		_, err := svc.Register(ctx, RegisterInput{
			Email:    "invalidotp@example.com",
			Password: "ValidPassword123",
			FullName: "Invalid OTP User",
		})
		if err != nil {
			t.Fatalf("failed to register: %v", err)
		}

		err = svc.VerifyEmail(ctx, "invalidotp@example.com", "000000")
		if err == nil {
			t.Fatal("expected error for invalid otp, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 400 {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
		if apiErr.Code != "INVALID_OTP" {
			t.Errorf("expected code INVALID_OTP, got %s", apiErr.Code)
		}
	})

	t.Run("Empty OTP is rejected", func(t *testing.T) {
		err := svc.VerifyEmail(ctx, "verify@example.com", "   ")
		if err == nil {
			t.Fatal("expected error for empty otp, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 422 {
			t.Errorf("expected status 422, got %d", apiErr.StatusCode)
		}
	})

	t.Run("Empty email is rejected", func(t *testing.T) {
		err := svc.VerifyEmail(ctx, "   ", "123456")
		if err == nil {
			t.Fatal("expected error for empty email, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 422 {
			t.Errorf("expected status 422, got %d", apiErr.StatusCode)
		}
	})

	t.Run("Mismatched email is rejected", func(t *testing.T) {
		// Register another fresh user to have a valid unconsumed otp
		otherReg, err := svc.Register(ctx, RegisterInput{
			Email:    "another@example.com",
			Password: "ValidPassword123",
			FullName: "Another User",
		})
		if err != nil {
			t.Fatalf("failed to register: %v", err)
		}
		_ = otherReg
		anotherOTP := mailer.sentEmails[len(mailer.sentEmails)-1].OTP

		// Attempt to verify with wrong email
		err = svc.VerifyEmail(ctx, "wrongemail@example.com", anotherOTP)
		if err == nil {
			t.Fatal("expected error for mismatched email, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 400 {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
		if apiErr.Code != "INVALID_OTP" {
			t.Errorf("expected code INVALID_OTP, got %s", apiErr.Code)
		}
	})

	t.Run("Expired OTP is rejected and deleted", func(t *testing.T) {
		// Register a user with expired otp
		expiredUser, err := mock.Create(ctx, db.CreateUserParams{
			Email:        "expired@example.com",
			PasswordHash: "hashed",
			FullName:     "Expired User",
			Role:         "customer",
		})
		if err != nil {
			t.Fatalf("failed to create user: %v", err)
		}

		expiredRawOTP := "987654"
		expiredHash := hashOTP(expiredRawOTP)
		tokenStore.Store(expiredUser.ID, expiredHash, -1*time.Second)

		err = svc.VerifyEmail(ctx, "expired@example.com", expiredRawOTP)
		if err == nil {
			t.Fatal("expected error for expired otp, got nil")
		}

		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 400 {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
		if apiErr.Code != "OTP_EXPIRED" {
			t.Errorf("expected code OTP_EXPIRED, got %s", apiErr.Code)
		}

		// Second attempt should report invalid (purged)
		err = svc.VerifyEmail(ctx, "expired@example.com", expiredRawOTP)
		if err == nil {
			t.Fatal("expected error for purged otp, got nil")
		}
	})

	t.Run("OTP remains unconsumed if database persistence fails", func(t *testing.T) {
		reg, err := svc.Register(ctx, RegisterInput{
			Email:    "dbfail@example.com",
			Password: "ValidPassword123",
			FullName: "DB Fail User",
		})
		if err != nil {
			t.Fatalf("failed to register: %v", err)
		}
		testOTP := mailer.sentEmails[len(mailer.sentEmails)-1].OTP

		// Remove user from repository to simulate persistence failure in MarkUserEmailVerified
		delete(mock.usersByID, reg.User.ID)

		err = svc.VerifyEmail(ctx, "dbfail@example.com", testOTP)
		if err == nil {
			t.Fatal("expected error on db failure, got nil")
		}

		// Restore user in mock repository to verify OTP can still be used on retry
		mock.usersByID[reg.User.ID] = db.User{
			ID:       reg.User.ID,
			Email:    "dbfail@example.com",
			FullName: "DB Fail User",
		}

		// Retry succeeds because OTP was not consumed on failure
		err = svc.VerifyEmail(ctx, "dbfail@example.com", testOTP)
		if err != nil {
			t.Fatalf("expected retry to succeed with unconsumed OTP, got %v", err)
		}
	})

	t.Run("Entering 5 wrong OTPs enforces 30-minute cooling period", func(t *testing.T) {
		_, err := svc.Register(ctx, RegisterInput{
			Email:    "cooldown@example.com",
			Password: "ValidPassword123",
			FullName: "Cooldown User",
		})
		if err != nil {
			t.Fatalf("failed to register: %v", err)
		}

		// Submit 4 incorrect OTPs -> rejected with INVALID_OTP (400)
		for i := 1; i <= 4; i++ {
			err = svc.VerifyEmail(ctx, "cooldown@example.com", "000000")
			if err == nil {
				t.Fatalf("attempt %d: expected error, got nil", i)
			}
			var apiErr *response.AppError
			if !errors.As(err, &apiErr) || apiErr.Code != "INVALID_OTP" {
				t.Fatalf("attempt %d: expected INVALID_OTP, got %v", i, err)
			}
		}

		// 5th attempt -> triggers OTP_COOLING_PERIOD (429)
		err = svc.VerifyEmail(ctx, "cooldown@example.com", "000000")
		if err == nil {
			t.Fatal("5th attempt: expected cooling period error, got nil")
		}
		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 429 {
			t.Errorf("expected status 429, got %d", apiErr.StatusCode)
		}
		if apiErr.Code != "OTP_COOLING_PERIOD" {
			t.Errorf("expected code OTP_COOLING_PERIOD, got %s", apiErr.Code)
		}

		// Subsequent verify attempts during cooling period are blocked with 429
		err = svc.VerifyEmail(ctx, "cooldown@example.com", "123456")
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 {
			t.Errorf("expected 429 during cooling period, got %v", err)
		}

		// Resend returns nil during cooling period (anti-enumeration) and suppresses email dispatch
		emailCountBefore := len(mailer.sentEmails)
		err = svc.ResendVerificationEmail(ctx, "cooldown@example.com")
		if err != nil {
			t.Errorf("expected resend to return nil during cooling period, got %v", err)
		}
		if len(mailer.sentEmails) != emailCountBefore {
			t.Errorf("expected no additional email sent during cooling period")
		}
	})
}

func TestUserService_ResendVerificationEmail(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	tokenStore := NewMemoryTokenStore(1 * time.Minute)
	defer tokenStore.Close()
	mailer := &mockEmailSender{}
	svc := NewService(mock, tokenStore, mailer, nil, secret, 15*time.Minute, 7*24*time.Hour, 5*time.Minute)

	// Register user
	_, err := svc.Register(ctx, RegisterInput{
		Email:    "resend@example.com",
		Password: "ValidPassword123",
		FullName: "Resend User",
	})
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	initialOTP := mailer.sentEmails[len(mailer.sentEmails)-1].OTP

	t.Run("Successfully resends verification email and invalidates old OTP", func(t *testing.T) {
		err := svc.ResendVerificationEmail(ctx, "resend@example.com")
		if err != nil {
			t.Fatalf("unexpected error resending verification: %v", err)
		}

		newOTP := mailer.sentEmails[len(mailer.sentEmails)-1].OTP
		if newOTP == initialOTP {
			t.Error("expected fresh OTP on resend")
		}

		// Verify old OTP was invalidated
		err = svc.VerifyEmail(ctx, "resend@example.com", initialOTP)
		if err == nil {
			t.Error("expected old OTP to be rejected after resend, got nil")
		}

		// Verify new OTP works
		err = svc.VerifyEmail(ctx, "resend@example.com", newOTP)
		if err != nil {
			t.Fatalf("expected new OTP to verify email: %v", err)
		}
	})

	t.Run("Already verified user returns nil (anti-enumeration)", func(t *testing.T) {
		emailCountBefore := len(mailer.sentEmails)
		err := svc.ResendVerificationEmail(ctx, "resend@example.com")
		if err != nil {
			t.Fatalf("expected nil error for anti-enumeration on verified account, got %v", err)
		}
		if len(mailer.sentEmails) != emailCountBefore {
			t.Errorf("expected no additional email sent to verified user")
		}
	})

	t.Run("Delivery failure returns error", func(t *testing.T) {
		_, err := svc.Register(ctx, RegisterInput{
			Email:    "mailerfail@example.com",
			Password: "ValidPassword123",
			FullName: "Mailer Fail",
		})
		if err != nil {
			t.Fatalf("failed to register: %v", err)
		}

		mailer.err = errors.New("brevo network error")
		defer func() { mailer.err = nil }()

		err = svc.ResendVerificationEmail(ctx, "mailerfail@example.com")
		if err == nil {
			t.Fatal("expected error when email dispatch fails, got nil")
		}
		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 500 {
			t.Errorf("expected status 500, got %d", apiErr.StatusCode)
		}
	})

	t.Run("Non-existent email returns nil (anti-enumeration)", func(t *testing.T) {
		err := svc.ResendVerificationEmail(ctx, "ghost@example.com")
		if err != nil {
			t.Fatalf("expected nil error for anti-enumeration, got %v", err)
		}
	})

	t.Run("Invalid email format returns 422", func(t *testing.T) {
		err := svc.ResendVerificationEmail(ctx, "not-an-email")
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

func TestUserService_Login(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	svc := NewService(mock, nil, nil, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

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
	svc := NewService(mock, nil, nil, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

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
		if claims.UserID != loginRes.User.ID {
			t.Errorf("expected user ID %s, got %s", loginRes.User.ID, claims.UserID)
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
		badSecretTokens, _ := GenerateTokenPair(loginRes.User.ID, loginRes.User.Email, loginRes.User.Role, "wrong-secret-key", 15*time.Minute, 7*24*time.Hour)
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

	t.Run("Profile edit or email verification does not revoke refresh token", func(t *testing.T) {
		u := mock.usersByEmail["refresh@example.com"]
		u.IsActive = true
		mock.usersByID[u.ID] = u

		freshLogin, err := svc.Login(ctx, LoginInput{
			Email:    "refresh@example.com",
			Password: password,
		})
		if err != nil {
			t.Fatalf("failed to login: %v", err)
		}

		initialUpdatedAt := u.UpdatedAt.Time

		// Simulate profile edit: updates u.UpdatedAt without touching PasswordChangedAt
		time.Sleep(2 * time.Millisecond)
		updatedUser, err := mock.Update(ctx, db.UpdateUserParams{
			ID:       u.ID,
			FullName: "Updated Name",
		})
		if err != nil {
			t.Fatalf("failed to update profile: %v", err)
		}

		// Assert that UpdatedAt advanced and PasswordChangedAt remained unchanged
		if !updatedUser.UpdatedAt.Time.After(initialUpdatedAt) {
			t.Errorf("expected UpdatedAt to advance from %v, got %v", initialUpdatedAt, updatedUser.UpdatedAt.Time)
		}
		if updatedUser.PasswordChangedAt.Valid {
			t.Errorf("expected PasswordChangedAt to remain unset/invalid, got %v", updatedUser.PasswordChangedAt)
		}

		// Refresh token must STILL work because password was not changed!
		refreshed, err := svc.RefreshToken(ctx, freshLogin.RefreshToken)
		if err != nil {
			t.Fatalf("expected refresh token to remain valid after profile edit, got error: %v", err)
		}
		if refreshed.AccessToken == "" {
			t.Error("expected non-empty access token")
		}
	})
}

func TestUserService_GetProfile(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	svc := NewService(mock, nil, nil, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	reg, err := svc.Register(ctx, RegisterInput{
		Email:    "profile@example.com",
		Password: "Password123!",
		FullName: "Profile User",
	})
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	t.Run("Returns profile for existing user", func(t *testing.T) {
		prof, err := svc.GetProfile(ctx, reg.User.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prof.ID != reg.User.ID {
			t.Errorf("expected ID %s, got %s", reg.User.ID, prof.ID)
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

func TestUserService_ForgotPassword(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	mailer := &mockEmailSender{}
	svc := NewService(mock, nil, mailer, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)

	_, err := svc.Register(ctx, RegisterInput{
		Email:    "forgot@example.com",
		Password: "OldPassword123!",
		FullName: "Forgot User",
	})
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	t.Run("Valid registered email sends reset token and returns success", func(t *testing.T) {
		err := svc.ForgotPassword(ctx, "forgot@example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var sent []struct{ Email, Name, Token string }
		for start := time.Now(); time.Since(start) < 2*time.Second; {
			sent = mailer.getSentResetEmails()
			if len(sent) > 0 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if len(sent) != 1 {
			t.Fatalf("expected 1 reset email sent, got %d", len(sent))
		}
		if sent[0].Email != "forgot@example.com" {
			t.Errorf("expected recipient forgot@example.com, got %s", sent[0].Email)
		}
		if sent[0].Token == "" {
			t.Error("expected non-empty reset token")
		}
	})

	t.Run("Non-existent email returns success without sending email (anti-enumeration)", func(t *testing.T) {
		countBefore := len(mailer.getSentResetEmails())
		err := svc.ForgotPassword(ctx, "doesnotexist@example.com")
		if err != nil {
			t.Fatalf("expected nil error for anti-enumeration, got %v", err)
		}
		if len(mailer.getSentResetEmails()) != countBefore {
			t.Errorf("expected no additional email sent for unknown address")
		}
	})

	t.Run("Delivery failure does not leak account existence and returns success", func(t *testing.T) {
		failingMailer := &mockEmailSender{err: errors.New("brevo service unavailable")}
		failSvc := NewService(mock, nil, failingMailer, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
		err := failSvc.ForgotPassword(ctx, "forgot@example.com")
		if err != nil {
			t.Fatalf("expected nil error even if delivery fails to prevent account enumeration, got: %v", err)
		}
	})

	t.Run("Invalid email format returns 422 validation error", func(t *testing.T) {
		err := svc.ForgotPassword(ctx, "not-an-email")
		if err == nil {
			t.Fatal("expected error for invalid email format, got nil")
		}
		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 422 {
			t.Errorf("expected status 422, got %d", apiErr.StatusCode)
		}
	})

	t.Run("Empty email returns 422 validation error", func(t *testing.T) {
		err := svc.ForgotPassword(ctx, "   ")
		if err == nil {
			t.Fatal("expected error for empty email, got nil")
		}
		var apiErr *response.AppError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 422 {
			t.Errorf("expected status 422, got %v", err)
		}
	})
}

func TestUserService_ResetPassword(t *testing.T) {
	ctx := context.Background()
	secret := "test-secret"
	mock := newMockUserRepository()
	mailer := &mockEmailSender{}
	resetStore := NewMemoryPasswordResetStore(1 * time.Minute)
	svc := NewService(mock, nil, mailer, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	svc.SetPasswordResetStore(resetStore)

	// Register user
	_, err := svc.Register(ctx, RegisterInput{
		Email:    "resetuser@example.com",
		Password: "OldPassword123!",
		FullName: "Reset User",
	})
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Login to obtain active tokens before reset
	loginRes, err := svc.Login(ctx, LoginInput{
		Email:    "resetuser@example.com",
		Password: "OldPassword123!",
	})
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	preResetRefreshToken := loginRes.RefreshToken

	// Verify pre-reset refresh token works before password change
	_, err = svc.RefreshToken(ctx, preResetRefreshToken)
	if err != nil {
		t.Fatalf("expected refresh token to work before reset: %v", err)
	}

	// Request forgot password
	err = svc.ForgotPassword(ctx, "resetuser@example.com")
	if err != nil {
		t.Fatalf("failed to request forgot password: %v", err)
	}
	var sentReset []struct{ Email, Name, Token string }
	for start := time.Now(); time.Since(start) < 2*time.Second; {
		sentReset = mailer.getSentResetEmails()
		if len(sentReset) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(sentReset) == 0 {
		t.Fatal("expected reset email to be sent")
	}
	resetToken := sentReset[len(sentReset)-1].Token

	t.Run("Successfully resets password, revokes existing refresh tokens, and allows new login", func(t *testing.T) {
		newPassword := "NewStrongPassword123!"
		err := svc.ResetPassword(ctx, resetToken, newPassword)
		if err != nil {
			t.Fatalf("failed to reset password: %v", err)
		}

		// 1. Old password is now rejected
		_, err = svc.Login(ctx, LoginInput{
			Email:    "resetuser@example.com",
			Password: "OldPassword123!",
		})
		if err == nil {
			t.Fatal("expected old password to fail login, got nil")
		}

		// 2. New password succeeds
		newLoginRes, err := svc.Login(ctx, LoginInput{
			Email:    "resetuser@example.com",
			Password: newPassword,
		})
		if err != nil {
			t.Fatalf("expected new password to succeed, got %v", err)
		}
		if newLoginRes.AccessToken == "" {
			t.Error("expected non-empty access token after reset login")
		}

		// 3. Existing refresh tokens issued before password reset are revoked!
		_, err = svc.RefreshToken(ctx, preResetRefreshToken)
		if err == nil {
			t.Fatal("expected pre-reset refresh token to be revoked, got nil")
		}
		var apiErr *response.AppError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
			t.Errorf("expected 401 for revoked refresh token, got %v", err)
		}

		// 4. Fresh refresh token issued with new password works
		_, err = svc.RefreshToken(ctx, newLoginRes.RefreshToken)
		if err != nil {
			t.Fatalf("expected new refresh token to work, got %v", err)
		}
	})

	t.Run("Used token cannot be reused", func(t *testing.T) {
		err := svc.ResetPassword(ctx, resetToken, "AnotherPassword123!")
		if err == nil {
			t.Fatal("expected used token to be rejected, got nil")
		}
		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 400 || apiErr.Code != "TOKEN_ALREADY_USED" {
			t.Errorf("expected 400 TOKEN_ALREADY_USED, got %d %s", apiErr.StatusCode, apiErr.Code)
		}
	})

	t.Run("Concurrent password reset requests with same token - only one succeeds", func(t *testing.T) {
		err := svc.ForgotPassword(ctx, "resetuser@example.com")
		if err != nil {
			t.Fatalf("failed to request forgot password: %v", err)
		}
		var sent []struct{ Email, Name, Token string }
		for start := time.Now(); time.Since(start) < 2*time.Second; {
			sent = mailer.getSentResetEmails()
			if len(sent) > len(sentReset) {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		concurrentToken := sent[len(sent)-1].Token

		var wg sync.WaitGroup
		startSignal := make(chan struct{})
		results := make([]error, 2)

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-startSignal
				results[idx] = svc.ResetPassword(ctx, concurrentToken, fmt.Sprintf("ConcurrentPass%d!", idx))
			}(i)
		}

		close(startSignal)
		wg.Wait()

		successCount := 0
		conflictCount := 0
		for _, res := range results {
			if res == nil {
				successCount++
			} else {
				var appErr *response.AppError
				if errors.As(res, &appErr) && appErr.Code == "TOKEN_ALREADY_USED" {
					conflictCount++
				}
			}
		}

		if successCount != 1 || conflictCount != 1 {
			t.Fatalf("expected exactly 1 success and 1 TOKEN_ALREADY_USED rejection, got %d successes and %d conflicts (errors: %v)", successCount, conflictCount, results)
		}
	})

	t.Run("Revocation cutoff persists across server restarts", func(t *testing.T) {
		curLogin, err := svc.Login(ctx, LoginInput{
			Email:    "resetuser@example.com",
			Password: "ConcurrentPass0!",
		})
		if err != nil {
			curLogin, err = svc.Login(ctx, LoginInput{
				Email:    "resetuser@example.com",
				Password: "ConcurrentPass1!",
			})
			if err != nil {
				t.Fatalf("failed to login: %v", err)
			}
		}
		curRefreshToken := curLogin.RefreshToken

		// Verify refresh token works initially
		_, err = svc.RefreshToken(ctx, curRefreshToken)
		if err != nil {
			t.Fatalf("expected refresh token to work: %v", err)
		}

		// Request and perform password reset
		err = svc.ForgotPassword(ctx, "resetuser@example.com")
		if err != nil {
			t.Fatalf("forgot password failed: %v", err)
		}
		var sent []struct{ Email, Name, Token string }
		curSentCount := len(mailer.getSentResetEmails())
		for start := time.Now(); time.Since(start) < 2*time.Second; {
			sent = mailer.getSentResetEmails()
			if len(sent) > curSentCount {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		token := sent[len(sent)-1].Token

		restartedPassword := "RestartPassword999!"
		err = svc.ResetPassword(ctx, token, restartedPassword)
		if err != nil {
			t.Fatalf("reset password failed: %v", err)
		}

		// Simulate server restart: create brand new service instance with fresh empty in-memory store
		newResetStore := NewMemoryPasswordResetStore(1 * time.Minute)
		restartedSvc := NewService(mock, nil, mailer, nil, secret, 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
		restartedSvc.SetPasswordResetStore(newResetStore)

		// Old refresh token must STILL be rejected due to persisted UpdatedAt in repository
		_, err = restartedSvc.RefreshToken(ctx, curRefreshToken)
		if err == nil {
			t.Fatal("expected pre-reset refresh token to be rejected after server restart, got nil")
		}

		// Logging in after reset works on restarted server
		postRestartLogin, err := restartedSvc.Login(ctx, LoginInput{
			Email:    "resetuser@example.com",
			Password: restartedPassword,
		})
		if err != nil {
			t.Fatalf("failed to login after restart: %v", err)
		}
		_, err = restartedSvc.RefreshToken(ctx, postRestartLogin.RefreshToken)
		if err != nil {
			t.Fatalf("expected new refresh token to work after server restart: %v", err)
		}
	})

	t.Run("Refresh token chain cannot cross password reset", func(t *testing.T) {
		loginRes, err := svc.Login(ctx, LoginInput{
			Email:    "resetuser@example.com",
			Password: "RestartPassword999!",
		})
		if err != nil {
			t.Fatalf("failed to login: %v", err)
		}

		// User refreshes token once before reset
		refreshed1, err := svc.RefreshToken(ctx, loginRes.RefreshToken)
		if err != nil {
			t.Fatalf("refresh 1 failed: %v", err)
		}

		// Reset password
		err = svc.ForgotPassword(ctx, "resetuser@example.com")
		if err != nil {
			t.Fatalf("forgot password failed: %v", err)
		}
		var sent []struct{ Email, Name, Token string }
		curCount := len(mailer.getSentResetEmails())
		for start := time.Now(); time.Since(start) < 2*time.Second; {
			sent = mailer.getSentResetEmails()
			if len(sent) > curCount {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		tok := sent[len(sent)-1].Token

		err = svc.ResetPassword(ctx, tok, "EvenNewerPass999!")
		if err != nil {
			t.Fatalf("reset password failed: %v", err)
		}

		// Attempting to refresh refreshed1.RefreshToken must FAIL because its session originated before reset!
		_, err = svc.RefreshToken(ctx, refreshed1.RefreshToken)
		if err == nil {
			t.Fatal("expected refreshed token from pre-reset session to be revoked, got nil")
		}
	})

	t.Run("Invalid token is rejected", func(t *testing.T) {
		err := svc.ResetPassword(ctx, "completely-invalid-token", "AnotherPassword123!")
		if err == nil {
			t.Fatal("expected invalid token to be rejected, got nil")
		}
		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 400 || apiErr.Code != "INVALID_TOKEN" {
			t.Errorf("expected 400 INVALID_TOKEN, got %d %s", apiErr.StatusCode, apiErr.Code)
		}
	})

	t.Run("Expired token is rejected", func(t *testing.T) {
		u := mock.usersByEmail["resetuser@example.com"]
		expiredRawToken := "expired-token-raw-12345"
		expiredHash := hashResetToken(expiredRawToken)
		resetStore.Store(expiredHash, u.ID, -1*time.Second)

		err := svc.ResetPassword(ctx, expiredRawToken, "AnotherPassword123!")
		if err == nil {
			t.Fatal("expected expired token to be rejected, got nil")
		}
		var apiErr *response.AppError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if apiErr.StatusCode != 400 || apiErr.Code != "TOKEN_EXPIRED" {
			t.Errorf("expected 400 TOKEN_EXPIRED, got %d %s", apiErr.StatusCode, apiErr.Code)
		}
	})

	t.Run("Rejects password with invalid length", func(t *testing.T) {
		err := svc.ResetPassword(ctx, "some-token", "short")
		if err == nil {
			t.Fatal("expected validation error for short password, got nil")
		}
		var apiErr *response.AppError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 422 {
			t.Errorf("expected 422 for short password, got %v", err)
		}
	})
}
