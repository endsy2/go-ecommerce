# Backend (Go + Gin + GORM)

## Layering — strict
handler → service → repository. Never skip a layer.

- `handler/` — bind + validate request, call service, write response. No business logic, no GORM.
- `service/` — business logic. Owns DB transactions. Returns domain errors.
- `repository/` — the ONLY place that imports GORM. Takes/returns models.
- `model/` — GORM structs. No JSON tags; use separate DTOs in `dto/`.
- `middleware/` — auth, logging, recovery, CORS, rate limit.
- Dependencies are wired manually in `cmd/api/main.go`. No DI framework.

## Error handling
- Services return typed errors (`ErrNotFound`, `ErrConflict`, `ErrForbidden`).
- One central error-to-HTTP mapper in `handler/`. Handlers never build error JSON inline.
- Never `panic` in a request path. Recovery middleware is a last resort only.

## Database
- Schema changes go in `migrations/` via golang-migrate. Do NOT use `AutoMigrate`.
- Every query that touches user data must be scoped by user/tenant.
- Use `Preload` to avoid N+1. Flag any loop that queries inside it.
- Stock changes must be atomic: conditional UPDATE or `SELECT ... FOR UPDATE`
  inside a transaction. Never read-then-write without locking.
- Transactions are started in the service layer and passed down as `*gorm.DB`.

## API
- Routes: `/api/v1/...`
- Response shape: `{ "data": ..., "meta": ... }` or `{ "error": { "code", "message" } }`
- Validate input with binding tags; return 422 with field-level errors.
- List endpoints are always paginated (`?page=&limit=`, max limit 100).

## Testing
- Every service function gets a table-driven test.
- Repository tests use a real Postgres via testcontainers, not mocks.
- Run `go test ./...` and `go vet ./...` before saying a task is done.

## Style
- `gofmt` always. Errors wrapped with `fmt.Errorf("doing x: %w", err)`.
- Context is the first parameter on every service and repository method.

## Data model

MVP tables (nothing else without asking me):
users, categories, products, carts, cart_items, orders, order_items

### Hard rules
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

### Schema changes
- Every schema change is a golang-migrate file with a working `down`.
- Never AutoMigrate, in any environment, including tests.
- One table (or one tightly-coupled pair) per migration.

### Layer boundaries for models
- GORM structs live in internal/model. No `json` tags on them.
- Request and response types live in internal/dto. Handlers map dto <-> model.
- Repositories accept and return models. Services accept and return models.
  Handlers are the only layer that touches dto.