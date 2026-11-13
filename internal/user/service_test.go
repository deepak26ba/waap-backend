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

type mockEmailSender struct {
	sentEmails []struct {
		Email, Name, OTP string
	}
	err error
}

func (m *mockEmailSender) SendVerificationEmail(ctx context.Context, toEmail, toName, otp string) error {
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

		// Resend is also blocked with 429 during cooling period
		err = svc.ResendVerificationEmail(ctx, "cooldown@example.com")
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 {
			t.Errorf("expected resend to be blocked with 429 during cooling period, got %v", err)
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
