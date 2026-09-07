// Package database opens and configures the application's data stores. It is a
// thin construction layer only — no queries live here. Repositories own those.
package database

import (
	"context"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"log/slog"
	"time"

	"ecommerce/backend/internal/config"
)

// NewPostgres opens a GORM connection, tunes the underlying connection pool and
// verifies the database is actually reachable before returning.
//
// A failure here is fatal to startup by design: Postgres is the source of truth,
// so serving traffic without it would only produce 500s. Failing at second 0
// with a clear message beats discovering it on the first customer request.
func NewPostgres(ctx context.Context, cfg config.PostgresConfig, devMode bool) (*gorm.DB, error) {
	// GORM logs every statement at Info. That is useful locally (it is how you
	// spot an N+1 loop) and far too noisy in production, where Warn leaves just
	// the slow-query and error lines.
	logLevel := gormlogger.Warn
	if devMode {
		logLevel = gormlogger.Info
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
		// Cache the prepared statement for each distinct query instead of
		// re-planning it on every call.
		PrepareStmt: true,
		// Map driver-specific codes onto GORM's own error values, so a unique
		// violation arrives as gorm.ErrDuplicatedKey rather than a
		// *pgconn.PgError carrying the string "23505".
		//
		// Without this, a repository wanting to tell "duplicate email" apart
		// from a genuine database failure would have to import a pgx type and
		// compare error codes — which would make pgx a direct dependency of the
		// repository layer just to read one constant.
		TranslateError: true,
	})
	if err != nil {
		// Deliberately does not wrap err with the DSN: it contains the password.
		return nil, fmt.Errorf("opening postgres connection: %w", err)
	}

	// GORM does not pool connections itself — it wraps database/sql, which does.
	// Reaching through to the *sql.DB is the only way to configure the pool, and
	// the stdlib defaults (unlimited open conns, 2 idle) are wrong for Postgres:
	// unbounded connections will exhaust max_connections under load.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("accessing underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("pinging postgres at %s:%d: %w", cfg.Host, cfg.Port, err)
	}

	slog.Info("connected to postgres",
		"host", cfg.Host,
		"port", cfg.Port,
		"database", cfg.Name,
		"max_open_conns", cfg.MaxOpenConns,
	)
	return db, nil
}

// ClosePostgres releases the connection pool. Call it only after the HTTP server
// has finished draining, or in-flight requests will fail on their last query.
func ClosePostgres(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("accessing underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}
