// Command migrate applies and rolls back the schema migrations in
// internal/migration.
//
// The migrations are Go functions driven by GORM, not .sql files, so this
// command opens the same GORM connection cmd/api does and hands it to the same
// runner. Nothing here knows any SQL.
//
// Usage (run from backend/, like cmd/api, because .env is resolved relatively):
//
//	go run ./cmd/migrate up          apply every pending migration
//	go run ./cmd/migrate down 1      roll back the last N migrations
//	go run ./cmd/migrate status      list what has been applied
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"gorm.io/gorm"

	"ecommerce/backend/internal/config"
	"ecommerce/backend/internal/database"
	"ecommerce/backend/internal/migration"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) == 0 {
		return errors.New("usage: migrate <up|down N|status>")
	}

	// Same two-step config load as cmd/api: .env populates the environment, then
	// real environment variables win over it.
	if err := config.LoadDotEnv(".env"); err != nil {
		return fmt.Errorf("loading .env: %w", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// The same constructor the API uses, so the pool settings, the DSN and the
	// SQL logging behaviour are identical to what migrations will run under in
	// the application.
	db, err := database.NewPostgres(context.Background(), cfg.Postgres, cfg.IsDevelopment())
	if err != nil {
		return err
	}
	defer func() {
		if err := database.ClosePostgres(db); err != nil {
			slog.Error("closing postgres", "error", err)
		}
	}()

	switch args[0] {
	case "up":
		if err := migration.Run(db); err != nil {
			return err
		}
		slog.Info("migrations applied")
		return printStatus(db)

	case "down":
		// No bare `down` that unwinds everything — the step count has to be
		// stated so the blast radius is a decision rather than a default.
		if len(args) < 2 {
			return errors.New("usage: migrate down N  (number of migrations to roll back)")
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 {
			return fmt.Errorf("down needs a positive number of steps, got %q", args[1])
		}
		if err := migration.Rollback(db, n); err != nil {
			return err
		}
		slog.Info("migrations rolled back", "steps", n)
		return printStatus(db)

	case "status":
		return printStatus(db)

	default:
		return fmt.Errorf("unknown command %q: expected up, down or status", args[0])
	}
}

func printStatus(db *gorm.DB) error {
	applied, err := migration.Applied(db)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		slog.Info("no migrations applied yet")
		return nil
	}
	slog.Info("migrations applied", "count", len(applied), "latest", applied[len(applied)-1])
	for _, id := range applied {
		slog.Info("  " + id)
	}
	return nil
}
