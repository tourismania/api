// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config aggregates the full application configuration.
type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Kafka    KafkaConfig
}

// AppConfig holds app-level metadata.
type AppConfig struct {
	Env     string
	Version string
}

// ServerConfig describes the HTTP server.
type ServerConfig struct {
	Address string

	// CORSAllowedOrigins is the whitelist of origins that may call the API.
	// Loaded from CORS_ALLOWED_ORIGINS (comma-separated).
	// Use "*" to allow any origin (development only).
	CORSAllowedOrigins []string
}

// DatabaseConfig describes PostgreSQL connection.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

// DSN returns a libpq DSN string.
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// URL returns a postgres URL-style DSN (for migrate).
func (c DatabaseConfig) URL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

// JWTConfig describes JWT signing material.
type JWTConfig struct {
	PrivateKeyPath string
	PublicKeyPath  string
	Passphrase     string
	TTL            time.Duration
}

// KafkaConfig describes Kafka brokers and topic.
type KafkaConfig struct {
	Brokers string
	Topic   string
}

// Load reads .env (best-effort) and constructs Config from environment.
func Load() (*Config, error) {
	// Best-effort: missing .env is not an error (e.g. in production).
	_ = godotenv.Load()

	port, err := atoiDefault("DATABASE_PORT", 5432)
	if err != nil {
		return nil, err
	}

	ttlSeconds, err := atoiDefault("JWT_TTL", 86400)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		App: AppConfig{
			Env:     getEnv("APP_ENV", "dev"),
			Version: getEnv("APP_VERSION", "0.0.0"),
		},
		Server: ServerConfig{
			Address:            getEnv("SERVER_NAME", ":8080"),
			CORSAllowedOrigins: splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS")),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DATABASE_HOST", "localhost"),
			Port:     port,
			User:     getEnv("DATABASE_USER", "root"),
			Password: getEnv("DATABASE_PASSWORD", ""),
			Name:     getEnv("DATABASE_NAME", "tourismania"),
			SSLMode:  getEnv("DATABASE_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			PrivateKeyPath: getEnv("JWT_PRIVATE_KEY_PATH", "./config/jwt/private.pem"),
			PublicKeyPath:  getEnv("JWT_PUBLIC_KEY_PATH", "./config/jwt/public.pem"),
			Passphrase:     os.Getenv("JWT_PASSPHRASE"),
			TTL:            time.Duration(ttlSeconds) * time.Second,
		},
		Kafka: KafkaConfig{
			Brokers: getEnv("KAFKA_DSN", "kafka:9092"),
			Topic:   getEnv("KAFKA_TOPIC", "events"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Database.Password == "" {
		return errors.New("DATABASE_PASSWORD is required")
	}
	if c.JWT.PrivateKeyPath == "" || c.JWT.PublicKeyPath == "" {
		return errors.New("JWT key paths are required")
	}
	return nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// splitCSV splits a comma-separated string into trimmed, non-empty tokens.
func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func atoiDefault(key string, def int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return v, nil
}
