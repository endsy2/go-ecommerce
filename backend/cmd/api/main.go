// Command api is the backend entry point.
//
// This file is the entire dependency-injection story. There is no component
// scan, no @Autowired and no application context: every object is constructed
// here, in dependency order, and passed by hand into the next constructor. The
// call sequence below IS the wiring diagram, and the compiler enforces it — a
// missing dependency is a build error at the call site, not a runtime
// NoSuchBeanDefinitionException.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"ecommerce/backend/internal/config"
	"ecommerce/backend/internal/database"
	"ecommerce/backend/internal/handler"
	"ecommerce/backend/internal/migration"
	"ecommerce/backend/internal/repository"
	"ecommerce/backend/internal/router"
	"ecommerce/backend/internal/service"
	"ecommerce/backend/internal/util"
)

func main() {
	// run returns an error instead of calling os.Exit itself, so that every
	// deferred cleanup below actually runs. os.Exit skips defers.
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Listen for shutdown signals before anything else, so a Ctrl+C during a
	// slow database connect is still honoured.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- 1. Configuration -------------------------------------------------
	// Populate the environment from backend/.env if it exists. Real environment
	// variables take precedence, and a missing file is fine (production gets its
	// config from the orchestrator).
	if err := config.LoadDotEnv(".env"); err != nil {
		return fmt.Errorf("loading .env: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	initLogger(cfg)
	slog.Info("starting api", "env", cfg.Env)

	// --- 2. Data stores ---------------------------------------------------
	// Postgres is the source of truth: a failure here is fatal.
	db, err := database.NewPostgres(ctx, cfg.Postgres, cfg.IsDevelopment())
	if err != nil {
		return err
	}
	defer func() {
		if err := database.ClosePostgres(db); err != nil {
			slog.Error("closing postgres", "error", err)
		}
	}()

	// Bring the schema up to date before anything can serve a request against a
	// table that does not exist yet. Off outside development — see the comment
	// on config.PostgresConfig.AutoMigrate for why.
	//
	// This runs the same versioned migrations cmd/migrate does, so a database
	// brought up this way is identical to one migrated as a deploy step —
	// CHECK constraints and ON DELETE rules included. Already-applied
	// migrations are skipped, so it is cheap on every subsequent start.
	if cfg.Postgres.AutoMigrate {
		if err := migration.Run(db); err != nil {
			return err
		}
	}

	// Redis is a cache: an unreachable server degrades the app, it does not
	// stop it. NewRedis logs a warning and returns a usable client either way.
	rdb := database.NewRedis(ctx, cfg.Redis)
	defer func() {
		if err := rdb.Close(); err != nil {
			slog.Error("closing redis", "error", err)
		}
	}()

	// --- 3. Repositories --------------------------------------------------
	// Repositories are the only place GORM is used. They take *gorm.DB.
	userRepo := repository.NewUserRepository(db)

	// --- 4. Services ------------------------------------------------------
	// Services hold business logic, own transactions, and return domain errors.
	// They depend on repository INTERFACES that the service package declares
	// itself — the consumer defines the interface, not the producer. That is
	// inverted from Spring, and it is what makes a table-driven test able to
	// pass a five-line fake struct with no mocking framework.
	//
	// Neither argument below is declared as an interface at this call site: they
	// are concrete types that happen to satisfy the interfaces service declares.
	// The compiler checks the fit here, at the wiring point.
	jwt := util.NewJWT(cfg.JWT.Secret, cfg.JWT.Expiry)
	authSvc := service.NewAuthService(userRepo, jwt)

	// --- 5. Handlers ------------------------------------------------------
	// Handlers bind and validate input, call a service, and write a response.
	handlers := router.Handlers{
		Health: handler.NewHealthHandler(db, rdb),
		// The auth cookie lives exactly as long as the token inside it, and is
		// marked Secure everywhere except local development, which runs on
		// plain http and would otherwise never receive it.
		Auth: handler.NewAuthHandler(authSvc, cfg.JWT.Expiry, !cfg.IsDevelopment()),
	}

	// --- 6. HTTP server ---------------------------------------------------
	// jwt goes to the router as well as the service: the service signs tokens
	// with it, the auth middleware verifies them with it, and passing the one
	// value to both is what guarantees they cannot drift onto different secrets.
	engine := router.New(cfg, handlers, jwt)
	srv := &http.Server{
		Addr:    cfg.HTTP.Addr(),
		Handler: engine,
		// Without these, one slow client can hold a connection indefinitely:
		// net/http applies no timeouts by default.
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	// ListenAndServe blocks, so it runs in its own goroutine and reports a
	// genuine bind failure (port already in use) back through this channel.
	serveErr := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	// --- 7. Wait for shutdown --------------------------------------------
	select {
	case err := <-serveErr:
		if err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
		// Signal received. Stop listening for further signals so a second
		// Ctrl+C can force-kill a hung shutdown.
		stop()
	}

	slog.Info("shutdown signal received, draining connections",
		"timeout", cfg.HTTP.ShutdownTimeout)

	// A fresh context: ctx is already cancelled, and Shutdown needs a live
	// deadline to bound the drain.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	// Shutdown stops accepting new connections and waits for in-flight requests
	// to finish. The deferred Close calls above run only after this returns —
	// closing Postgres first would fail every request still being served.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return errors.Join(errors.New("graceful shutdown failed"), err)
	}

	slog.Info("shutdown complete")
	return nil
}

// initLogger installs the process-wide slog handler: human-readable text in
// development, JSON in every other environment so logs are machine-parseable.
func initLogger(cfg *config.Config) {
	var h slog.Handler
	if cfg.IsDevelopment() {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(h))
}
