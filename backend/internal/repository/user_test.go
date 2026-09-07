package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"ecommerce/backend/internal/domain"
	"ecommerce/backend/internal/migration"
	"ecommerce/backend/internal/model"
)

// newTestDB starts a throwaway Postgres, applies the real migrations to it, and
// returns a GORM handle.
//
// A real database rather than a mock, per backend/CLAUDE.md. The point is that
// this exercises the actual SQL: a mock would happily accept the
// lower(email) = lower(?) predicate without ever proving Postgres does.
//
// The container is torn down by t.Cleanup, so each test that calls this gets a
// clean schema and they cannot leak state into each other.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping: needs Docker (run without -short)")
	}

	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("ecommerce_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		// Postgres starts, runs its init scripts, then restarts — so the ready
		// line appears twice. Waiting for only the first one hands back a
		// database that is about to go away, and the first query fails.
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(2*time.Minute),
		),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Logf("terminating container: %v", err)
		}
	})

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("building connection string: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{
		// Silent: a failing test should show its own assertion, not 40 lines of
		// SQL above it.
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("opening gorm connection: %v", err)
	}

	// The real migrations, not a bare AutoMigrate. This is what makes the test
	// meaningful: the CHECK constraints, the lower(email) unique index and the
	// ON DELETE rules only exist because migration.Run creates them, so a test
	// schema built any other way would not be the schema that runs in
	// production.
	if err := migration.Run(db); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}

	return db
}

// seedUser inserts a user and returns it, with the id Postgres generated.
func seedUser(t *testing.T, db *gorm.DB, email string) *model.User {
	t.Helper()

	u := &model.User{
		Email:        email,
		PasswordHash: "not-a-real-hash",
		Name:         "Test User",
		Role:         model.RoleCustomer,
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seeding user %q: %v", email, err)
	}
	return u
}

func TestUserRepositoryFindByEmail(t *testing.T) {
	db := newTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	seeded := seedUser(t, db, "Bob@Example.com")

	tests := []struct {
		name      string
		email     string
		wantFound bool
	}{
		{name: "exact match", email: "Bob@Example.com", wantFound: true},
		{name: "lowercase match", email: "bob@example.com", wantFound: true},
		{name: "uppercase match", email: "BOB@EXAMPLE.COM", wantFound: true},
		{name: "unknown email", email: "nobody@example.com"},
		{name: "empty email", email: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.FindByEmail(ctx, tt.email)

			if !tt.wantFound {
				// domain.ErrNotFound, never gorm.ErrRecordNotFound: nothing
				// above this layer should have to know GORM exists.
				if !errors.Is(err, domain.ErrNotFound) {
					t.Fatalf("got error %v, want domain.ErrNotFound", err)
				}
				if got != nil {
					t.Errorf("got user %+v, want nil", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			}
			if got.ID != seeded.ID {
				t.Errorf("got id %s, want %s", got.ID, seeded.ID)
			}
			// The stored spelling comes back, not the spelling that was queried.
			if got.Email != "Bob@Example.com" {
				t.Errorf("got email %q, want the stored spelling", got.Email)
			}
		})
	}
}

// TestUserRepositoryIDIsGenerated proves the gen_random_uuid() default reaches
// Go: the model leaves ID zero on insert and Postgres fills it in.
func TestUserRepositoryIDIsGenerated(t *testing.T) {
	db := newTestDB(t)

	u := seedUser(t, db, "generated@example.com")

	if u.ID.String() == "00000000-0000-0000-0000-000000000000" {
		t.Error("id was not populated from the database default")
	}
	if u.CreatedAt.IsZero() {
		t.Error("created_at was not populated")
	}
}

func TestUserRepositoryFindByID(t *testing.T) {
	db := newTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	seeded := seedUser(t, db, "byid@example.com")

	t.Run("existing id", func(t *testing.T) {
		got, err := repo.FindByID(ctx, seeded.ID)
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}
		if got.ID != seeded.ID {
			t.Errorf("got id %s, want %s", got.ID, seeded.ID)
		}
		if got.Email != "byid@example.com" {
			t.Errorf("got email %q, want %q", got.Email, "byid@example.com")
		}
	})

	t.Run("unknown id", func(t *testing.T) {
		got, err := repo.FindByID(ctx, uuid.New())
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got error %v, want domain.ErrNotFound", err)
		}
		if got != nil {
			t.Errorf("got user %+v, want nil", got)
		}
	})

	t.Run("nil uuid finds nothing", func(t *testing.T) {
		// uuid.Nil is what reaches this method if a handler ever reads a missing
		// context value without checking the ok flag. It has to miss cleanly
		// rather than match whatever row happens to sort first.
		got, err := repo.FindByID(ctx, uuid.Nil)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got error %v, want domain.ErrNotFound", err)
		}
		if got != nil {
			t.Errorf("got user %+v, want nil", got)
		}
	})
}
