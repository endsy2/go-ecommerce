// Package migration holds the ordered schema migrations, written in Go against
// GORM rather than as .sql files.
//
// Each migration does two things:
//
//   - tx.AutoMigrate builds the table shape from the model struct — columns,
//     types, defaults, plain and unique indexes, and the foreign keys whose
//     ON DELETE rule is declared with a `constraint:` tag on an association.
//   - tx.Exec adds what struct tags cannot express: CHECK constraints, indexes
//     on an expression such as lower(email), partial indexes, and the foreign
//     keys whose model has no association field to hang a tag on.
//
// The second half is the reason this package exists rather than a single
// AutoMigrate call. Those constraints are what stop the database overselling
// stock, accepting an invented order status, or deleting a customer's purchase
// history — none of which a Go struct tag can say.
//
// Migrations are versioned and reversible: every entry has a Rollback, and
// gormigrate records applied ids in a `migrations` table, so a redeploy applies
// only what is new.
package migration

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"ecommerce/backend/internal/model"
)

// options configures the gormigrate bookkeeping table.
func options() *gormigrate.Options {
	o := *gormigrate.DefaultOptions
	// Postgres has transactional DDL, so a migration that fails half way can be
	// rolled back completely instead of leaving the schema in a state no
	// migration knows how to describe. gormigrate leaves this off by default
	// because MySQL cannot do it; Postgres can, so turn it on.
	o.UseTransaction = true
	return &o
}

// all is the ordered migration list. Append only — never edit or reorder an
// entry that has already run somewhere, or that database will disagree with
// this list forever.
func all() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		{
			ID: "000001_create_users",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&model.User{}); err != nil {
					return err
				}
				return execAll(tx,
					// Mirrors model.Role. Without it the column takes any string.
					`ALTER TABLE users ADD CONSTRAINT users_role_check
					   CHECK (role IN ('customer', 'admin'))`,

					// Case-insensitive uniqueness. A struct tag can only produce
					// a unique index on the raw column, which would let
					// Bob@example.com and bob@example.com both register.
					`CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email))`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&model.User{})
			},
		},
		{
			ID: "000002_create_categories",
			Migrate: func(tx *gorm.DB) error {
				// Nothing beyond the struct: the unique index on slug comes from
				// the uniqueIndex tag, which is all this table needs.
				return tx.AutoMigrate(&model.Category{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&model.Category{})
			},
		},
		{
			ID: "000003_create_products",
			Migrate: func(tx *gorm.DB) error {
				// The products -> categories foreign key with ON DELETE RESTRICT
				// comes from the constraint tag on Product.Category.
				if err := tx.AutoMigrate(&model.Product{}); err != nil {
					return err
				}
				return execAll(tx,
					`ALTER TABLE products ADD CONSTRAINT products_price_cents_check
					   CHECK (price_cents >= 0)`,

					// The backstop that makes the required conditional UPDATE
					// safe: even if a caller forgets `AND stock >= ?`, the
					// database refuses to record negative stock.
					`ALTER TABLE products ADD CONSTRAINT products_stock_check
					   CHECK (stock >= 0)`,

					// Partial index: nearly every read wants products still on
					// sale, and this only stores those rows.
					`CREATE INDEX products_active_idx ON products (id)
					   WHERE deleted_at IS NULL`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&model.Product{})
			},
		},
		{
			ID: "000004_create_carts",
			Migrate: func(tx *gorm.DB) error {
				// Both tables together: a cart with no line-item table is
				// unusable, so they are one tightly-coupled pair.
				//
				// cart_items -> carts (CASCADE) and cart_items -> products
				// (RESTRICT) both come from constraint tags.
				if err := tx.AutoMigrate(&model.Cart{}, &model.CartItem{}); err != nil {
					return err
				}
				return execAll(tx,
					// Cart has no User association field, so GORM cannot derive
					// this one.
					`ALTER TABLE carts ADD CONSTRAINT carts_user_id_fkey
					   FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE`,

					`ALTER TABLE cart_items ADD CONSTRAINT cart_items_quantity_check
					   CHECK (quantity > 0)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				// Child first: cart_items points at carts.
				return tx.Migrator().DropTable(&model.CartItem{}, &model.Cart{})
			},
		},
		{
			ID: "000005_create_orders",
			Migrate: func(tx *gorm.DB) error {
				// order_items -> orders (CASCADE) comes from the tag on
				// Order.Items.
				if err := tx.AutoMigrate(&model.Order{}, &model.OrderItem{}); err != nil {
					return err
				}
				return execAll(tx,
					// RESTRICT, not CASCADE: deleting a customer must not erase
					// what they bought and were charged.
					`ALTER TABLE orders ADD CONSTRAINT orders_user_id_fkey
					   FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT`,

					// OrderItem has no Product association on purpose, so this
					// foreign key has to be written out.
					`ALTER TABLE order_items ADD CONSTRAINT order_items_product_id_fkey
					   FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE RESTRICT`,

					// Mirrors model.OrderStatus.
					`ALTER TABLE orders ADD CONSTRAINT orders_status_check
					   CHECK (status IN ('pending', 'paid', 'shipped', 'delivered', 'cancelled'))`,

					`ALTER TABLE orders ADD CONSTRAINT orders_total_cents_check
					   CHECK (total_cents >= 0)`,
					`ALTER TABLE order_items ADD CONSTRAINT order_items_quantity_check
					   CHECK (quantity > 0)`,
					`ALTER TABLE order_items ADD CONSTRAINT order_items_unit_price_cents_check
					   CHECK (unit_price_cents >= 0)`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&model.OrderItem{}, &model.Order{})
			},
		},
		{
			ID: "000006_add_products_slug",
			Migrate: func(tx *gorm.DB) error {
				// Every statement here is idempotent, and that is not defensive
				// habit — it is required by the way this package works.
				//
				// 000003 builds products with tx.AutoMigrate(&model.Product{}),
				// which reads the struct as it is TODAY. Adding Slug to the model
				// therefore changed what that earlier entry does: on a fresh
				// database 000003 now creates the column and a plain unique index
				// from the tag, while a database already at 000005 has neither. An
				// AutoMigrate-driven migration list has no frozen snapshots, so a
				// later entry has to converge both cases onto the same schema.
				return execAll(tx,
					`ALTER TABLE products ADD COLUMN IF NOT EXISTS slug text`,

					// The same transformation util.Slugify does, in SQL. It has to
					// be duplicated here because a migration cannot call into
					// application code that may change under it.
					`UPDATE products
					   SET slug = trim(both '-' from regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g'))
					   WHERE slug IS NULL OR slug = ''`,

					// A name of only punctuation, or one in a non-Latin script,
					// slugifies to the empty string. Fall back to the id, which is
					// ugly but unique and keeps the row addressable.
					`UPDATE products SET slug = id::text WHERE slug IS NULL OR slug = ''`,

					// Two products legitimately share a name. Suffixing every member
					// of a colliding group with its own id prefix makes them distinct
					// before the unique index is built.
					`UPDATE products p
					   SET slug = p.slug || '-' || left(p.id::text, 8)
					   WHERE EXISTS (
					     SELECT 1 FROM products q WHERE q.slug = p.slug AND q.id <> p.id
					   )`,

					`ALTER TABLE products ALTER COLUMN slug SET NOT NULL`,

					// Drop whatever the struct tag may have produced on a fresh
					// database, then create the index this schema actually wants:
					// PARTIAL, matching products_active_idx. A soft-deleted product
					// keeps its slug row but stops reserving the name, so the
					// catalogue can list a replacement under it. Both indexes carry
					// the same name, so a later AutoMigrate sees it already exists
					// and leaves this version alone.
					`DROP INDEX IF EXISTS idx_products_slug`,
					`CREATE UNIQUE INDEX idx_products_slug ON products (slug)
					   WHERE deleted_at IS NULL`,
				)
			},
			Rollback: func(tx *gorm.DB) error {
				// Drops exactly what Migrate added — the index and the column.
				// Dropping the table here would destroy the catalogue over a
				// reversible column change.
				return execAll(tx,
					`DROP INDEX IF EXISTS idx_products_slug`,
					`ALTER TABLE products DROP COLUMN IF EXISTS slug`,
				)
			},
		},
	}
}

// execAll runs statements in order and stops at the first failure, so a
// migration reads as a list of DDL rather than six identical error checks.
func execAll(tx *gorm.DB, statements ...string) error {
	for _, s := range statements {
		if err := tx.Exec(s).Error; err != nil {
			return fmt.Errorf("executing %q: %w", s, err)
		}
	}
	return nil
}

// Run applies every migration that has not run yet. Safe to call on every
// startup: already-applied ids are skipped.
func Run(db *gorm.DB) error {
	if err := gormigrate.New(db, options(), all()).Migrate(); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}

// Rollback undoes the last n migrations, most recent first.
//
// There is no "roll back everything" convenience on purpose: a bare command
// that drops the whole schema is far too easy to run against the wrong
// database. The caller has to say how far.
func Rollback(db *gorm.DB, n int) error {
	m := gormigrate.New(db, options(), all())
	for i := range n {
		if err := m.RollbackLast(); err != nil {
			return fmt.Errorf("rolling back (step %d of %d): %w", i+1, n, err)
		}
	}
	return nil
}

// Applied returns the ids recorded as run, oldest first, for the status command.
func Applied(db *gorm.DB) ([]string, error) {
	var ids []string

	// Query the bookkeeping table directly: gormigrate exposes no listing API.
	// Table.rowid ordering would be wrong, so rely on insertion order via ctid,
	// which is good enough for a human-facing status readout.
	err := db.Table(options().TableName).
		Select(options().IDColumnName).
		Order("ctid").
		Pluck(options().IDColumnName, &ids).Error
	if err != nil {
		return nil, fmt.Errorf("reading applied migrations: %w", err)
	}
	return ids, nil
}
