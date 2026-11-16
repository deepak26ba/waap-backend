package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"waap-backend/pkg/db"
	"waap-backend/pkg/email"
	"waap-backend/pkg/response"
)

const (
	// BcryptCost12 is the required hashing cost factor specified in TICKET-06.
	BcryptCost12 = 12

	DefaultAccessTTL             = 15 * time.Minute
	DefaultRefreshTTL            = 7 * 24 * time.Hour
	DefaultVerificationOTPTTL    = 5 * time.Minute
	DefaultVerificationTokenTTL  = DefaultVerificationOTPTTL
	DefaultPasswordResetTokenTTL = 15 * time.Minute
)

// Precomputed dummy bcrypt hash (cost 12) for constant-time password comparisons
// when user is not found, effectively eliminating timing-based user enumeration attacks.
const dummyBcryptHash = "$2a$12$e8Yk2uKzLqO2Vl5h4v1lQO8sC6P0f9F2a6D1H4J7K9L2M5N7O0P3Q"

// Service defines the business logic contract for authentication and user management.
type Service interface {
	Register(ctx context.Context, in RegisterInput) (*RegisterResponse, error)
	Login(ctx context.Context, in LoginInput) (*LoginResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*UserResponse, error)
	VerifyEmail(ctx context.Context, email, otp string) error
	ResendVerificationEmail(ctx context.Context, email string) error
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	SetPasswordResetStore(store PasswordResetStore)
	SetPasswordResetTTL(ttl time.Duration)
}

type service struct {
	repo                 Repository
	tokenStore           TokenStore
	passwordResetStore   PasswordResetStore
	mailer               email.Sender
	log                  *zap.Logger
	jwtSecret            string
	accessTTL            time.Duration
	refreshTTL           time.Duration
	verificationTokenTTL time.Duration
	passwordResetTTL     time.Duration
}

// NewService constructs an instance of the user service.
func NewService(
	repo Repository,
	tokenStore TokenStore,
	mailer email.Sender,
	log *zap.Logger,
	jwtSecret string,
	accessTTL, refreshTTL, verificationTokenTTL time.Duration,
) Service {
	if tokenStore == nil {
		tokenStore = NewMemoryTokenStore(1 * time.Minute)
	}
	if mailer == nil {
		mailer = &email.NoopSender{}
	}
	if log == nil {
		log = zap.NewNop()
	}
	if accessTTL <= 0 {
		accessTTL = DefaultAccessTTL
	}
	if refreshTTL <= 0 {
		refreshTTL = DefaultRefreshTTL
	}
	if verificationTokenTTL <= 0 {
		verificationTokenTTL = DefaultVerificationTokenTTL
	}
	return &service{
		repo:                 repo,
		tokenStore:           tokenStore,
		passwordResetStore:   NewMemoryPasswordResetStore(1 * time.Minute),
		mailer:               mailer,
		log:                  log,
		jwtSecret:            jwtSecret,
		accessTTL:            accessTTL,
		refreshTTL:           refreshTTL,
		verificationTokenTTL: verificationTokenTTL,
		passwordResetTTL:     DefaultPasswordResetTokenTTL,
	}
}

func (s *service) SetPasswordResetStore(store PasswordResetStore) {
	if store != nil {
		s.passwordResetStore = store
	}
}

func (s *service) SetPasswordResetTTL(ttl time.Duration) {
	if ttl > 0 {
		s.passwordResetTTL = ttl
	}
}

// Register handles validation, Bcrypt password hashing (cost 12), database insertion,
// in-memory verification token storage, and dispatching verification email via Brevo.
func (s *service) Register(ctx context.Context, in RegisterInput) (*RegisterResponse, error) {
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
	emailAddr := strings.ToLower(parsedAddr.Address)

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
	_, err = s.repo.GetByEmail(ctx, emailAddr)
	if err == nil {
		return nil, response.NewConflictError("EMAIL_ALREADY_EXISTS", "a user with this email address already exists")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		s.log.Error("failed to check existing user by email", zap.Error(err), zap.String("email", emailAddr))
		return nil, response.NewInternalError(errors.New("failed to check existing user"))
	}

	// 3. Hash password using Bcrypt with cost 12
	hashedPassword, err := HashPassword(in.Password)
	if err != nil {
		s.log.Error("failed to hash user password", zap.Error(err))
		return nil, response.NewInternalError(errors.New("failed to process credentials"))
	}

	// 4. Create user in database
	var phone pgtype.Text
	if in.PhoneNumber != nil && strings.TrimSpace(*in.PhoneNumber) != "" {
		phone = pgtype.Text{String: strings.TrimSpace(*in.PhoneNumber), Valid: true}
	}

	u, err := s.repo.Create(ctx, db.CreateUserParams{
		Email:        emailAddr,
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
		s.log.Error("failed to create user in database", zap.Error(err), zap.String("email", emailAddr))
		return nil, response.NewInternalError(errors.New("failed to register user"))
	}

	// 5. Generate secure 6-digit numeric OTP and store in memory (deleted automatically after expired)
	rawOTP, otpHash, err := generateOTP()
	if err != nil {
		s.log.Error("failed to generate email verification otp", zap.Error(err), zap.String("user_id", u.ID.String()))
	} else {
		// Store in memory with 5-minute expiration TTL
		s.tokenStore.Store(u.ID, otpHash, s.verificationTokenTTL)

		// 6. Deliver verification email via Brevo
		// Sensitive OTP is passed to email transport, NEVER logged in application logs
		if err := s.mailer.SendVerificationEmail(ctx, u.Email, u.FullName, rawOTP); err != nil {
			s.log.Error("failed to dispatch verification email", zap.Error(err), zap.String("user_id", u.ID.String()))
		}
	}

	var resPhone *string
	if u.PhoneNumber.Valid {
		p := u.PhoneNumber.String
		resPhone = &p
	}

	return &RegisterResponse{
		Message: "Registration successful. Please enter the OTP sent to your email to activate your account.",
		User: UserResponse{
			ID:            u.ID,
			Email:         u.Email,
			FullName:      u.FullName,
			Role:          u.Role,
			PhoneNumber:   resPhone,
			EmailVerified: u.EmailVerified,
			CreatedAt:     u.CreatedAt.Time,
		},
	}, nil
}

// Login authenticates credentials, applies timing-safe dummy check on missing users, and returns JWT token pair.
func (s *service) Login(ctx context.Context, in LoginInput) (*LoginResponse, error) {
	emailAddr := strings.ToLower(strings.TrimSpace(in.Email))
	if emailAddr == "" || in.Password == "" {
		return nil, response.NewUnauthorizedError("invalid email or password")
	}

	// 1. Fetch user by email
	u, err := s.repo.GetByEmail(ctx, emailAddr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Anti-enumeration: execute dummy bcrypt check to neutralize timing side-channels
			CheckDummyPassword(in.Password)
			return nil, response.NewUnauthorizedError("invalid email or password")
		}
		return nil, response.NewInternalError(errors.New("failed to verify credentials"))
	}

	// 2. Verify password
	if err := CheckPassword(u.PasswordHash, in.Password); err != nil {
		return nil, response.NewUnauthorizedError("invalid email or password")
	}

	// 3. Verify active status (only checked AFTER password matches to prevent enumeration of inactive accounts)
	if !u.IsActive {
		return nil, response.NewForbiddenError("user account is deactivated")
	}

	// 4. Issue dual tokens (15m access, 7d refresh)
	tokens, err := GenerateTokenPair(u.ID, u.Email, u.Role, s.jwtSecret, s.accessTTL, s.refreshTTL)
	if err != nil {
		return nil, response.NewInternalError(errors.New("failed to issue authentication tokens"))
	}

	var resPhone *string
	if u.PhoneNumber.Valid {
		p := u.PhoneNumber.String
		resPhone = &p
	}

	return &LoginResponse{
		TokenPair: *tokens,
		User: UserResponse{
			ID:            u.ID,
			Email:         u.Email,
			FullName:      u.FullName,
			Role:          u.Role,
			PhoneNumber:   resPhone,
			EmailVerified: u.EmailVerified,
			CreatedAt:     u.CreatedAt.Time,
		},
	}, nil
}

// RefreshToken validates an active refresh token and returns a fresh token pair.
func (s *service) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, response.NewUnauthorizedError("refresh token is required")
	}

	// 1. Parse and validate claims
	claims, err := ValidateToken(refreshToken, s.jwtSecret)
	if err != nil {
		return nil, response.NewUnauthorizedError("invalid or expired refresh token")
	}

	// 2. Ensure token type is refresh
	if claims.TokenType != TokenTypeRefresh {
		return nil, response.NewUnauthorizedError("invalid token type: refresh token expected")
	}

	// 3. Verify user still exists and is active
	u, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, response.NewUnauthorizedError("user account not found")
		}
		return nil, response.NewInternalError(errors.New("failed to verify user status"))
	}

	if !u.IsActive {
		return nil, response.NewForbiddenError("user account is deactivated")
	}

	// 4. Verify that session has not been revoked due to password reset
	var issuedAt time.Time
	if claims.IssuedAt != nil {
		issuedAt = claims.IssuedAt.Time
	}
	if s.passwordResetStore.IsSessionRevoked(u.ID, issuedAt, claims.IssuedAtUnixNano) {
		return nil, response.NewUnauthorizedError("session has been revoked due to password reset, please log in again")
	}
	if u.UpdatedAt.Valid && !u.UpdatedAt.Time.IsZero() {
		// Persisted revocation check across server restarts: tokens issued strictly before the last password update
		cutoffNano := u.UpdatedAt.Time.UnixNano()
		if claims.IssuedAtUnixNano > 0 {
			if claims.IssuedAtUnixNano < cutoffNano {
				return nil, response.NewUnauthorizedError("session has been revoked due to password reset, please log in again")
			}
		} else if issuedAt.Before(u.UpdatedAt.Time) {
			return nil, response.NewUnauthorizedError("session has been revoked due to password reset, please log in again")
		}
	}

	// 5. Issue fresh token pair
	return GenerateTokenPair(u.ID, u.Email, u.Role, s.jwtSecret, s.accessTTL, s.refreshTTL)
}

// VerifyEmail validates the submitted verification OTP and ensures it matches the provided email address.
func (s *service) VerifyEmail(ctx context.Context, rawEmail, rawOTP string) error {
	rawEmail = strings.TrimSpace(rawEmail)
	if rawEmail == "" {
		return response.NewValidationError("validation failed", []response.FieldError{
			{Field: "email", Message: "email is required"},
		})
	}
	rawOTP = strings.TrimSpace(rawOTP)
	if rawOTP == "" {
		return response.NewValidationError("validation failed", []response.FieldError{
			{Field: "otp", Message: "verification otp is required"},
		})
	}

	parsedAddr, err := mail.ParseAddress(rawEmail)
	if err != nil || parsedAddr.Address != rawEmail {
		return response.NewValidationError("validation failed", []response.FieldError{
			{Field: "email", Message: "invalid email format"},
		})
	}
	emailAddr := strings.ToLower(parsedAddr.Address)

	// 1. Resolve submitted email to user identity
	u, err := s.repo.GetByEmail(ctx, emailAddr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return response.NewBadRequestError("INVALID_OTP", "invalid or expired verification otp")
		}
		s.log.Error("failed to find user by email during verification", zap.Error(err), zap.String("email", emailAddr))
		return response.NewInternalError(errors.New("failed to verify email"))
	}

	// 2. Validate user's OTP before consuming it
	otpHash := hashOTP(rawOTP)
	if err := s.tokenStore.Validate(u.ID, otpHash); err != nil {
		switch {
		case errors.Is(err, ErrOTPCoolingPeriod):
			return response.NewAppError(http.StatusTooManyRequests, "OTP_COOLING_PERIOD", "too many failed attempts, cooling period active: please try again after 30 minutes")
		case errors.Is(err, ErrTokenNotFound), errors.Is(err, ErrOTPNotFound):
			return response.NewBadRequestError("INVALID_OTP", "invalid or expired verification otp")
		case errors.Is(err, ErrTokenExpired), errors.Is(err, ErrOTPExpired):
			return response.NewBadRequestError("OTP_EXPIRED", "verification otp has expired")
		case errors.Is(err, ErrTokenAlreadyUsed), errors.Is(err, ErrOTPAlreadyUsed):
			return response.NewBadRequestError("OTP_ALREADY_USED", "verification otp has already been used")
		default:
			return response.NewBadRequestError("INVALID_OTP", "invalid verification otp")
		}
	}

	// 3. Mark user verified in database
	if _, err := s.repo.MarkUserEmailVerified(ctx, u.ID); err != nil {
		s.log.Error("failed to mark email as verified", zap.Error(err), zap.String("user_id", u.ID.String()))
		return response.NewInternalError(errors.New("failed to verify email"))
	}

	// 4. Consume OTP only after successful DB persistence
	_ = s.tokenStore.Consume(u.ID)

	s.log.Info("email verified successfully", zap.String("user_id", u.ID.String()))
	return nil
}

// ResendVerificationEmail sends a fresh 6-digit verification OTP to an unverified user's email.
func (s *service) ResendVerificationEmail(ctx context.Context, rawEmail string) error {
	rawEmail = strings.TrimSpace(rawEmail)
	if rawEmail == "" {
		return response.NewValidationError("validation failed", []response.FieldError{
			{Field: "email", Message: "email is required"},
		})
	}

	parsedAddr, err := mail.ParseAddress(rawEmail)
	if err != nil || parsedAddr.Address != rawEmail {
		return response.NewValidationError("validation failed", []response.FieldError{
			{Field: "email", Message: "invalid email format"},
		})
	}
	emailAddr := strings.ToLower(parsedAddr.Address)

	u, err := s.repo.GetByEmail(ctx, emailAddr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Anti-enumeration: return nil without error so malicious callers cannot probe user registration status
			return nil
		}
		s.log.Error("failed to look up user for verification resend", zap.Error(err))
		return response.NewInternalError(errors.New("failed to process verification resend"))
	}

	if u.EmailVerified {
		// Anti-enumeration: return the same success response as other accounts
		s.log.Info("verification resend skipped for verified email", zap.String("user_id", u.ID.String()))
		return nil
	}

	if s.tokenStore.IsLocked(u.ID) {
		// Anti-enumeration: return the same success response during cooling period without issuing a new OTP
		s.log.Info("verification resend suppressed during cooling period", zap.String("user_id", u.ID.String()))
		return nil
	}

	// Invalidate any older pending OTPs for this user
	s.tokenStore.DeleteByUserID(u.ID)

	// Generate fresh random 6-digit numeric OTP
	rawOTP, otpHash, err := generateOTP()
	if err != nil {
		s.log.Error("failed to generate email verification otp", zap.Error(err), zap.String("user_id", u.ID.String()))
		return response.NewInternalError(errors.New("failed to generate verification otp"))
	}

	s.tokenStore.Store(u.ID, otpHash, s.verificationTokenTTL)

	if err := s.mailer.SendVerificationEmail(ctx, u.Email, u.FullName, rawOTP); err != nil {
		s.log.Error("failed to dispatch verification email", zap.Error(err), zap.String("user_id", u.ID.String()))
		return response.NewInternalError(errors.New("failed to send verification email"))
	}

	s.log.Info("verification email resent", zap.String("user_id", u.ID.String()))
	return nil
}

// generateOTP produces a cryptographically secure, uniformly distributed 6-digit numeric OTP.
func generateOTP() (string, string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", "", fmt.Errorf("failed to generate random otp: %w", err)
	}
	rawOTP := fmt.Sprintf("%06d", 100000+n.Int64())
	otpHash := hashOTP(rawOTP)
	return rawOTP, otpHash, nil
}

// hashOTP returns the hex-encoded SHA-256 digest of an OTP string.
func hashOTP(rawOTP string) string {
	h := sha256.Sum256([]byte(rawOTP))
	return hex.EncodeToString(h[:])
}

// HashPassword hashes a plaintext password using bcrypt with cost 12.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost12)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword verifies that a plaintext password matches the stored bcrypt hash.
func CheckPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// CheckDummyPassword executes a bcrypt comparison against a dummy hash to maintain
// constant execution time when the target account does not exist.
func CheckDummyPassword(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyBcryptHash), []byte(password))
}

// ForgotPassword initiates the password reset process by generating a cryptographically secure,
// time-limited reset token and sending it to the user's email.
// The response deliberately returns nil (success) even if the account does not exist (anti-enumeration).
func (s *service) ForgotPassword(ctx context.Context, rawEmail string) error {
	rawEmail = strings.TrimSpace(rawEmail)
	if rawEmail == "" {
		return response.NewValidationError("validation failed", []response.FieldError{
			{Field: "email", Message: "email is required"},
		})
	}

	parsedAddr, err := mail.ParseAddress(rawEmail)
	if err != nil || parsedAddr.Address != rawEmail {
		return response.NewValidationError("validation failed", []response.FieldError{
			{Field: "email", Message: "invalid email format"},
		})
	}
	emailAddr := strings.ToLower(parsedAddr.Address)

	u, err := s.repo.GetByEmail(ctx, emailAddr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Anti-enumeration: do not reveal whether account exists
			return nil
		}
		s.log.Error("failed to look up user for forgot password", zap.Error(err))
		return response.NewInternalError(errors.New("failed to process password reset request"))
	}

	// Invalidate any older reset tokens for this user
	s.passwordResetStore.DeleteByUserID(u.ID)

	// Generate cryptographically secure reset token
	rawToken, tokenHash, err := generateResetToken()
	if err != nil {
		s.log.Error("failed to generate password reset token", zap.Error(err), zap.String("user_id", u.ID.String()))
		return response.NewInternalError(errors.New("failed to generate password reset token"))
	}

	// Store only the secure hash of the token
	s.passwordResetStore.Store(tokenHash, u.ID, s.passwordResetTTL)

	// Send through configured email provider asynchronously to ensure comparable response timing
	// and prevent account enumeration via delivery status or network latency
	go func(toEmail, toName, token string) {
		sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.mailer.SendPasswordResetEmail(sendCtx, toEmail, toName, token); err != nil {
			s.log.Error("failed to dispatch password reset email", zap.Error(err), zap.String("email", toEmail))
		}
	}(u.Email, u.FullName, rawToken)

	s.log.Info("password reset requested", zap.String("user_id", u.ID.String()))
	return nil
}

// ResetPassword validates the submitted reset token, hashes the new password with Bcrypt (cost 12),
// updates the user's password in the database, invalidates existing sessions/refresh tokens, and consumes the token.
func (s *service) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return response.NewValidationError("validation failed", []response.FieldError{
			{Field: "token", Message: "reset token is required"},
		})
	}

	passBytes := []byte(newPassword)
	if len(passBytes) < 8 || len(passBytes) > 72 {
		return response.NewValidationError("validation failed", []response.FieldError{
			{Field: "new_password", Message: "password must be between 8 and 72 characters long"},
		})
	}

	// 1. Atomically reserve token before changing password to prevent concurrent reuse
	tokenHash := hashResetToken(rawToken)
	userID, err := s.passwordResetStore.Reserve(tokenHash)
	if err != nil {
		switch {
		case errors.Is(err, ErrResetTokenAlreadyUsed):
			return response.NewBadRequestError("TOKEN_ALREADY_USED", "password reset token has already been used")
		case errors.Is(err, ErrResetTokenExpired):
			return response.NewBadRequestError("TOKEN_EXPIRED", "password reset token has expired")
		default:
			return response.NewBadRequestError("INVALID_TOKEN", "invalid or expired password reset token")
		}
	}

	// 2. Hash new password using Bcrypt with cost 12
	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		s.passwordResetStore.Release(tokenHash)
		s.log.Error("failed to hash new password", zap.Error(err))
		return response.NewInternalError(errors.New("failed to process password update"))
	}

	// 3. Update password in database
	if _, err := s.repo.UpdatePassword(ctx, userID, hashedPassword); err != nil {
		s.passwordResetStore.Release(tokenHash)
		s.log.Error("failed to update password in database", zap.Error(err), zap.String("user_id", userID.String()))
		return response.NewInternalError(errors.New("failed to reset password"))
	}

	// 4. Invalidate existing sessions/refresh tokens after successful password reset
	s.passwordResetStore.RevokeSessions(userID)

	s.log.Info("password reset successfully", zap.String("user_id", userID.String()))
	return nil
}

// generateResetToken produces a cryptographically secure 256-bit token (64 hex characters) and its SHA-256 hash.
func generateResetToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("failed to generate random token: %w", err)
	}
	rawToken := hex.EncodeToString(b)
	tokenHash := hashResetToken(rawToken)
	return rawToken, tokenHash, nil
}

// hashResetToken returns the hex-encoded SHA-256 digest of a password reset token string.
func hashResetToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

// GenerateTokenPair issues a short-lived access token and a longer-lived refresh token.
func GenerateTokenPair(userID uuid.UUID, emailAddr, role, secret string, accessTTL, refreshTTL time.Duration) (*TokenPair, error) {
	if secret == "" {
		return nil, errors.New("jwt secret key cannot be empty")
	}

	now := time.Now()
	nowNano := now.UnixNano()
	accessExpiry := now.Add(accessTTL)
	refreshExpiry := now.Add(refreshTTL)

	// 1. Build Access Token Claims
	accessClaims := &Claims{
		UserID:           userID,
		Email:            emailAddr,
		Role:             role,
		TokenType:        TokenTypeAccess,
		IssuedAtUnixNano: nowNano,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// 2. Build Refresh Token Claims
	refreshClaims := &Claims{
		UserID:           userID,
		Email:            emailAddr,
		Role:             role,
		TokenType:        TokenTypeRefresh,
		IssuedAtUnixNano: nowNano,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(refreshExpiry),
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refreshToken.SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTTL.Seconds()),
		ExpiresAt:    accessExpiry,
	}, nil
}

// ValidateToken parses, validates signature and expiration of a JWT token string.
func ValidateToken(tokenString, secret string) (*Claims, error) {
	if secret == "" {
		return nil, errors.New("jwt secret key cannot be empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GetProfile retrieves a user's profile by their unique ID.
func (s *service) GetProfile(ctx context.Context, userID uuid.UUID) (*UserResponse, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, response.NewNotFoundError("user profile not found")
		}
		return nil, response.NewAppError(http.StatusInternalServerError, response.ErrCodeInternalServerError, "failed to retrieve user profile")
	}

	var phone *string
	if u.PhoneNumber.Valid {
		p := u.PhoneNumber.String
		phone = &p
	}

	return &UserResponse{
		ID:            u.ID,
		Email:         u.Email,
		FullName:      u.FullName,
		Role:          u.Role,
		PhoneNumber:   phone,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt.Time,
	}, nil
}
