// Package config loads deployment configuration from the environment.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	minSessionTTL = time.Minute
	maxConcurrent = 32
)

// Config is the process configuration. Secret values are never logged.
type Config struct {
	AdminUsername        string
	AdminPassword        string
	AdminPasswordReset   bool
	EncryptionKey        []byte
	SessionTTL           time.Duration
	CookieSecure         bool
	BackupDir            string
	MetadataDBPath       string
	SQLiteAllowedRoots   []string
	HTTPAddr             string
	TrustedProxies       []string
	MaxConcurrentBackups int
	LogLevel             string
	DBHostAllowlist      []string
	S3AllowInsecure      bool
	BackupTimeout        time.Duration
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	key, err := decodeKey(os.Getenv("APP_ENCRYPTION_KEY"))
	if err != nil {
		return Config{}, err
	}
	ttl, err := durationEnv("SESSION_TTL", 12*time.Hour, minSessionTTL)
	if err != nil {
		return Config{}, err
	}
	timeout, err := durationEnv("BACKUP_TIMEOUT", 2*time.Hour, time.Minute)
	if err != nil {
		return Config{}, err
	}
	maxN, err := intEnv("MAX_CONCURRENT_BACKUPS", 2, 1, maxConcurrent)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		AdminUsername:        strings.TrimSpace(os.Getenv("ADMIN_USERNAME")),
		AdminPassword:        os.Getenv("ADMIN_PASSWORD"),
		AdminPasswordReset:   boolEnv("ADMIN_PASSWORD_RESET"),
		EncryptionKey:        key,
		SessionTTL:           ttl,
		CookieSecure:         boolEnv("COOKIE_SECURE"),
		BackupDir:            envDefault("BACKUP_DIR", "/data/backups"),
		MetadataDBPath:       envDefault("METADATA_DB_PATH", "/data/meta/dbackup.db"),
		SQLiteAllowedRoots:   splitCSV(envDefault("SQLITE_ALLOWED_ROOTS", "/sqlite-sources")),
		HTTPAddr:             envDefault("HTTP_ADDR", ":8080"),
		TrustedProxies:       splitCSV(os.Getenv("TRUSTED_PROXIES")),
		MaxConcurrentBackups: maxN,
		LogLevel:             strings.ToLower(envDefault("LOG_LEVEL", "info")),
		DBHostAllowlist:      splitCSV(os.Getenv("DB_HOST_ALLOWLIST")),
		S3AllowInsecure:      boolEnv("S3_ALLOW_INSECURE"),
		BackupTimeout:        timeout,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks structural configuration. Administrator credentials are
// checked during bootstrap because they are optional once an account exists.
func (c Config) Validate() error {
	if len(c.EncryptionKey) != 32 {
		return fmt.Errorf("APP_ENCRYPTION_KEY must decode to 32 bytes")
	}
	if c.SessionTTL < minSessionTTL {
		return fmt.Errorf("SESSION_TTL must be at least 1m")
	}
	if c.BackupTimeout < time.Minute {
		return fmt.Errorf("BACKUP_TIMEOUT must be at least 1m")
	}
	if c.MaxConcurrentBackups < 1 || c.MaxConcurrentBackups > maxConcurrent {
		return fmt.Errorf("MAX_CONCURRENT_BACKUPS must be between 1 and %d", maxConcurrent)
	}
	if strings.TrimSpace(c.BackupDir) == "" || strings.TrimSpace(c.MetadataDBPath) == "" {
		return fmt.Errorf("BACKUP_DIR and METADATA_DB_PATH are required")
	}
	if len(c.SQLiteAllowedRoots) == 0 {
		return fmt.Errorf("SQLITE_ALLOWED_ROOTS must contain at least one directory")
	}
	if strings.TrimSpace(c.HTTPAddr) == "" {
		return fmt.Errorf("HTTP_ADDR is required")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
	return nil
}

func decodeKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("APP_ENCRYPTION_KEY is required")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("APP_ENCRYPTION_KEY must be standard base64 for exactly 32 bytes")
	}
	return key, nil
}

func envDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func boolEnv(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func durationEnv(key string, fallback, min time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < min {
		return 0, fmt.Errorf("%s must be a valid duration of at least %s", key, min)
	}
	return d, nil
}

func intEnv(key string, fallback, min, max int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", key, min, max)
	}
	return n, nil
}
