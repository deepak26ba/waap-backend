package config

import (
	"fmt"
	"os"
	"strconv"
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

	// Auth
	JWTSecret string
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

	cfg := &Config{
		Port:              getEnv("PORT", "8080"),
		Env:               getEnv("ENV", "development"),
		DBHost:            dbHost,
		DBPort:            dbPort,
		DBUser:            dbUser,
		DBPassword:        dbPass,
		DBName:            dbName,
		DBSSLMode:         dbSSLMode,
		DBMaxConns:        dbMaxConns,
		DBMinConns:        dbMinConns,
		DBMaxConnLifetime: dbMaxConnLifetime,
		DBMaxConnIdleTime: dbMaxConnIdleTime,
		DatabaseURL:       dbURL,
		JWTSecret:         getEnv("JWT_SECRET", "default-dev-secret-replace-in-production"),
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(valStr)
	if err != nil {
		return defaultVal
	}
	return d
}

