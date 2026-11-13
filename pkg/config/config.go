package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Port string
	Env  string

	// Database
	DBHost            string
	DBPort            string
	DBUser            string
	DBPassword        string
	DBName            string
	DBSSLMode         string
	DBMaxConns        int32
	DBMinConns        int32
	DBMaxConnLifetime time.Duration
	DBMaxConnIdleTime time.Duration
	DatabaseURL       string
	AutoMigrate       bool

	// Auth
	JWTSecret            string
	VerificationOTPTTL   time.Duration
	VerificationTokenTTL time.Duration

	// Email (Brevo)
	BrevoAPIKey    string
	BrevoFromEmail string
	BrevoFromName  string

	// Security & Limits
	CORSAllowedOrigins []string
	MaxBodySizeBytes   int64
	RateLimitRPS       float64
	RateLimitBurst     int
}

func LoadConfig() (*Config, error) {
	// Load .env file if it exists, ignore error if missing (standard 12-factor app behavior)
	_ = godotenv.Load()

	dbMaxConns := int32(getEnvAsInt("DB_MAX_CONNS", 25))
	dbMinConns := int32(getEnvAsInt("DB_MIN_CONNS", 5))
	dbMaxConnLifetime := getEnvAsDuration("DB_MAX_CONN_LIFETIME", 1*time.Hour)
	dbMaxConnIdleTime := getEnvAsDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute)

	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "waap_db")
	dbSSLMode := getEnv("DB_SSLMODE", "disable")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			dbUser, dbPass, dbHost, dbPort, dbName, dbSSLMode)
	}

	corsOriginsStr := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:8080")
	var corsOrigins []string
	for _, o := range strings.Split(corsOriginsStr, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			corsOrigins = append(corsOrigins, trimmed)
		}
	}

	maxBodySizeBytes := int64(getEnvAsInt("MAX_BODY_SIZE_BYTES", 2*1024*1024)) // 2MB default
	rateLimitRPS := getEnvAsFloat("RATE_LIMIT_RPS", 20.0)
	rateLimitBurst := getEnvAsInt("RATE_LIMIT_BURST", 40)
	autoMigrate := getEnvAsBool("DB_AUTO_MIGRATE", true)
	env := getEnv("ENV", "development")

	brevoAPIKey := getEnv("BREVO_API_KEY", "")
	brevoFromEmail := getEnv("BREVO_FROM_EMAIL", "olivergot26@gmail.com")
	brevoFromName := getEnv("BREVO_FROM_NAME", "WAAP")
	verificationOTPTTL := getEnvAsDuration("VERIFICATION_OTP_TTL", getEnvAsDuration("VERIFICATION_TOKEN_TTL", 5*time.Minute))

	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if jwtSecret == "" {
		if env == "production" {
			return nil, fmt.Errorf("security configuration error: JWT_SECRET environment variable is required in production")
		}
		jwtSecret = "dev-insecure-secret-key-change-in-production-32b"
	} else if len(jwtSecret) < 32 && env == "production" {
		return nil, fmt.Errorf("security configuration error: JWT_SECRET in production must be at least 32 characters long")
	}

	if env == "production" && brevoAPIKey == "" {
		return nil, fmt.Errorf("configuration error: BREVO_API_KEY environment variable is required in production")
	}

	cfg := &Config{
		Port:                 getEnv("PORT", "8080"),
		Env:                  env,
		DBHost:               dbHost,
		DBPort:               dbPort,
		DBUser:               dbUser,
		DBPassword:           dbPass,
		DBName:               dbName,
		DBSSLMode:            dbSSLMode,
		DBMaxConns:           dbMaxConns,
		DBMinConns:           dbMinConns,
		DBMaxConnLifetime:    dbMaxConnLifetime,
		DBMaxConnIdleTime:    dbMaxConnIdleTime,
		DatabaseURL:          dbURL,
		AutoMigrate:          autoMigrate,
		JWTSecret:            jwtSecret,
		VerificationOTPTTL:   verificationOTPTTL,
		VerificationTokenTTL: verificationOTPTTL,
		BrevoAPIKey:          brevoAPIKey,
		BrevoFromEmail:       brevoFromEmail,
		BrevoFromName:        brevoFromName,
		CORSAllowedOrigins:   corsOrigins,
		MaxBodySizeBytes:     maxBodySizeBytes,
		RateLimitRPS:         rateLimitRPS,
		RateLimitBurst:       rateLimitBurst,
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsFloat(key string, defaultVal float64) float64 {
	valStr := getEnv(key, "")
	if val, err := strconv.ParseFloat(valStr, 64); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
	valStr := getEnv(key, "")
	if val, err := strconv.ParseBool(valStr); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	valStr := getEnv(key, "")
	if val, err := time.ParseDuration(valStr); err == nil {
		return val
	}
	return defaultVal
}
