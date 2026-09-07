// Package config turns process environment variables into a typed, validated
// Config value. It is the only package that reads os.Getenv, so there is exactly
// one place to look when you want to know what the app is configurable by.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the whole application configuration, grouped by concern.
//
// It is deliberately nested rather than one flat struct: NewPostgres only ever
// receives cfg.Postgres, so it is not even able to read an unrelated secret.
type Config struct {
	Env      string
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	JWT      JWTConfig
}

// IsDevelopment reports whether the app is running in local development, which
// turns on chattier logging (SQL statements, request bodies on error).
func (c Config) IsDevelopment() bool { return c.Env == "development" }

type HTTPConfig struct {
	Host string
	Port int

	// ReadTimeout and WriteTimeout bound how long a single request may occupy a
	// connection. Go's http.Server has NO timeouts by default, so a slow client
	// can hold a connection open forever; these are not optional in practice.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// ShutdownTimeout bounds how long we wait for in-flight requests to finish
	// after a SIGINT/SIGTERM before cutting them off.
	ShutdownTimeout time.Duration

	CORSAllowedOrigins []string
}

// Addr is the "host:port" string http.Server binds to.
func (h HTTPConfig) Addr() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

type PostgresConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration

	// AutoMigrate makes cmd/api run GORM's AutoMigrate against the models on
	// startup, so `go run ./cmd/api` brings the schema up to date on its own.
	//
	// It defaults to true only in development. In any other environment the
	// application must not reshape its own database while starting: several
	// instances would race, and AutoMigrate applies changes with no review step
	// and no way back. There, the schema is a deploy step (cmd/migrate).
	AutoMigrate bool
}

// DSN builds the Postgres connection string.
//
// This is the ONLY place the DSN is assembled, and it must never be logged: it
// carries the database password in plaintext.
func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.Name, p.SSLMode,
	)
}

// MinJWTSecretLen is the shortest secret Load will accept.
//
// HS256 signs with HMAC-SHA256, whose block size is 32 bytes. A shorter secret
// does not make the algorithm weaker in a mathematical sense, but a secret short
// enough to be typed by hand is short enough to be brute-forced offline: an
// attacker with one valid token can grind candidate secrets locally, at no cost
// and with no rate limit, until the signature matches. Then they can mint tokens
// for any user and any role.
const MinJWTSecretLen = 32

type JWTConfig struct {
	// Secret is the HMAC signing key. []byte rather than string because that is
	// what the signer wants, and because it reads as key material rather than as
	// an ordinary printable setting. Never log it.
	Secret []byte

	// Expiry is how long an issued token stays valid, and also how long the auth
	// cookie lives, so the two cannot drift apart.
	Expiry time.Duration
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int

	DialTimeout time.Duration
}

// Addr is the "host:port" string the Redis client connects to.
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// Load reads configuration from the environment and validates it.
//
// It returns an error rather than exiting or panicking: only main decides to
// terminate the process. That also keeps Load usable from tests that want to
// assert on the failure path.
//
// Every problem found is reported at once, so a fresh checkout with an empty
// .env tells you all four missing variables in one run instead of one per
// restart.
func Load() (*Config, error) {
	var problems []string

	fail := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	cfg := &Config{
		Env: optionalString("APP_ENV", "development"),
	}

	// --- HTTP -------------------------------------------------------------
	cfg.HTTP = HTTPConfig{
		Host:               optionalString("HTTP_HOST", "0.0.0.0"),
		CORSAllowedOrigins: optionalCSV("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
	}
	cfg.HTTP.Port = optionalInt("HTTP_PORT", 8080, fail)
	cfg.HTTP.ReadTimeout = optionalDuration("HTTP_READ_TIMEOUT", 10*time.Second, fail)
	cfg.HTTP.WriteTimeout = optionalDuration("HTTP_WRITE_TIMEOUT", 15*time.Second, fail)
	cfg.HTTP.IdleTimeout = optionalDuration("HTTP_IDLE_TIMEOUT", 60*time.Second, fail)
	cfg.HTTP.ShutdownTimeout = optionalDuration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second, fail)

	// --- Postgres ---------------------------------------------------------
	// These four have no safe default: guessing them would mean connecting to
	// the wrong database, so they are required.
	cfg.Postgres = PostgresConfig{
		Host:     requiredString("POSTGRES_HOST", fail),
		User:     requiredString("POSTGRES_USER", fail),
		Password: requiredString("POSTGRES_PASSWORD", fail),
		Name:     requiredString("POSTGRES_DB", fail),
		SSLMode:  optionalString("POSTGRES_SSLMODE", "disable"),
	}
	cfg.Postgres.Port = optionalInt("POSTGRES_PORT", 5432, fail)
	cfg.Postgres.MaxOpenConns = optionalInt("POSTGRES_MAX_OPEN_CONNS", 25, fail)
	cfg.Postgres.MaxIdleConns = optionalInt("POSTGRES_MAX_IDLE_CONNS", 5, fail)
	cfg.Postgres.ConnMaxLifetime = optionalDuration("POSTGRES_CONN_MAX_LIFETIME", 5*time.Minute, fail)
	// Defaults to on in development only — see the field comment on AutoMigrate.
	cfg.Postgres.AutoMigrate = optionalBool("POSTGRES_AUTO_MIGRATE", cfg.IsDevelopment(), fail)

	// --- JWT --------------------------------------------------------------
	// The secret is required with no default. A default signing key would be
	// the same on every checkout and in every deployment, which means anyone
	// who has read the repository can mint an admin token.
	secret := requiredString("JWT_SECRET", fail)
	if secret != "" && len(secret) < MinJWTSecretLen {
		fail("JWT_SECRET must be at least %d characters, got %d",
			MinJWTSecretLen, len(secret))
	}
	cfg.JWT = JWTConfig{Secret: []byte(secret)}
	cfg.JWT.Expiry = optionalDuration("JWT_EXPIRY", 24*time.Hour, fail)

	// --- Redis ------------------------------------------------------------
	// Redis is a cache here, not a source of truth, so localhost defaults are
	// fine and an empty password is legitimate.
	cfg.Redis = RedisConfig{
		Host:     optionalString("REDIS_HOST", "localhost"),
		Password: os.Getenv("REDIS_PASSWORD"),
	}
	cfg.Redis.Port = optionalInt("REDIS_PORT", 6379, fail)
	cfg.Redis.DB = optionalInt("REDIS_DB", 0, fail)
	cfg.Redis.DialTimeout = optionalDuration("REDIS_DIAL_TIMEOUT", 5*time.Second, fail)

	if cfg.Postgres.MaxIdleConns > cfg.Postgres.MaxOpenConns {
		fail("POSTGRES_MAX_IDLE_CONNS (%d) must not exceed POSTGRES_MAX_OPEN_CONNS (%d)",
			cfg.Postgres.MaxIdleConns, cfg.Postgres.MaxOpenConns)
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid configuration:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}

// --- small typed readers -------------------------------------------------
//
// Each takes a `fail` callback instead of returning an error so that Load can
// collect every problem in one pass rather than stopping at the first.

func requiredString(key string, fail func(string, ...any)) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		fail("%s is required but not set", key)
	}
	return v
}

func optionalString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func optionalInt(key string, def int, fail func(string, ...any)) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		fail("%s must be an integer, got %q", key, raw)
		return def
	}
	return v
}

func optionalBool(key string, def bool, fail func(string, ...any)) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	// ParseBool accepts 1/t/T/true/TRUE and 0/f/F/false/FALSE, which covers the
	// spellings people actually put in a .env file.
	v, err := strconv.ParseBool(raw)
	if err != nil {
		fail("%s must be true or false, got %q", key, raw)
		return def
	}
	return v
}

func optionalDuration(key string, def time.Duration, fail func(string, ...any)) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		fail("%s must be a duration such as 10s or 5m, got %q", key, raw)
		return def
	}
	if v <= 0 {
		fail("%s must be positive, got %q", key, raw)
		return def
	}
	return v
}

func optionalCSV(key string, def []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}
