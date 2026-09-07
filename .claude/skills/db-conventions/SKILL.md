---
name: db-conventions
description: Database and migration rules — schema, money, historical records, locking
paths: "backend/internal/migration/**, backend/internal/model/**, backend/internal/repository/**"
user-invocable: false
---

# Data model

MVP tables (nothing else without asking me):
users, categories, products, carts, cart_items, orders, order_items

## Hard rules

- Primary keys are uuid, defaulted in Postgres with gen_random_uuid().
- Money is int64 cents in Go and BIGINT in Postgres. Column names end in
  `_cents`. Never float, never NUMERIC, never a money string. Format only
  at the API boundary.
- `order_items` stores its own copy of product_name and unit_price_cents.
  Never join to products to display a past order. Orders are historical
  records and must not change when a product is renamed, repriced, or removed.
- `cart_items` does the opposite: it stores only product_id and quantity,
  and reads the current price from products at read time.
- Products are soft-deleted with deleted_at. Never hard-delete a product;
  order_items reference it.
- Every table has created_at and updated_at (timestamptz, not null).
- Foreign keys are declared in the migration with explicit ON DELETE
  behaviour. Do not rely on GORM associations to enforce integrity.
- products.stock is the source of truth for availability. Any change to it
  happens inside a transaction with row locking or a conditional UPDATE.
  Never read stock, compute, then write.
- orders.status is a fixed set: pending, paid, shipped, delivered, cancelled.
  Adding a status requires a migration and my approval.

## Schema changes

Migrations are Go functions driven by gormigrate, not `.sql` files. They live in
`all()` in `backend/internal/migration/migrations.go`. `backend/migrations/` is
empty and unused — do not put files there.

- Every schema change is a new entry with a working `Rollback` that fully
  reverses the `Migrate`.
- Never reshape the schema outside a versioned migration entry — no ad-hoc
  `AutoMigrate` in `cmd/api`, a test, or a script. Inside an entry,
  `tx.AutoMigrate(&model.X{})` is the correct way to build table shape from the
  struct; follow it with `tx.Exec` for what tags cannot express: CHECK
  constraints, expression indexes such as `lower(email)`, partial indexes, and
  foreign keys with no association field to hang a `constraint:` tag on.
- One table (or one tightly-coupled pair) per migration.
- Append only. Never edit or reorder an entry that has already run anywhere —
  that database will disagree with the list forever.
- IDs are sequential and descriptive: `000006_create_reviews`.

Entry shape:

```go
{
    ID: "000006_create_x",
    Migrate: func(tx *gorm.DB) error {
        if err := tx.AutoMigrate(&model.X{}); err != nil {
            return err
        }
        return execAll(tx, `ALTER TABLE x ADD CONSTRAINT ...`)
    },
    Rollback: func(tx *gorm.DB) error {
        return tx.Migrator().DropTable(&model.X{})
    },
}
```

Apply with `cd backend && go run ./cmd/migrate up`; check with `status`.

## Layer boundaries for models

- GORM structs live in internal/model. No `json` tags on them.
- Request and response types live in internal/dto. Handlers map dto <-> model.
- Repositories accept and return models. Services accept and return models.
  Handlers are the only layer that touches dto.
