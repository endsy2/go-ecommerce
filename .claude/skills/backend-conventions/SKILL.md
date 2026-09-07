---
name: backend-conventions
description: Go backend conventions for this project — layering, error handling, API shape, testing
paths: "backend/internal/**"
user-invocable: false
---

# Backend conventions

## Layering — strict

handler → service → repository. Never skip a layer.

- `handler/` — bind + validate request, call service, write response. No business logic, no GORM.
- `service/` — business logic. Owns DB transactions. Returns domain errors.
- `repository/` — the ONLY place that imports GORM. Takes/returns models.
- `model/` — GORM structs. Request/response types live in `dto/`; the tag
  and mapping rules are in db-conventions.
- `middleware/` — auth, logging, recovery, CORS, rate limit.
- Dependencies are wired manually in `cmd/api/main.go`. No DI framework.

Supporting packages: `domain/` (typed errors), `config/`, `database/`,
`router/` (route table), `migration/` (schema), `util/`.

### Existing exceptions — do not add more without asking

- `handler/health.go` holds `*gorm.DB` and `*redis.Client` directly to ping
  them for the readiness probe. Infrastructure, not business logic.
- `model/product.go` imports gorm for the `gorm.DeletedAt` soft-delete type.

## Transactions

Services start transactions through a `Tx` interface defined in
`internal/repository`, never by taking a `*gorm.DB`. Services never import gorm.

Not implemented yet — no transaction exists in the codebase. When you write the
first one, create the interface rather than passing `*gorm.DB` up:

```go
// internal/repository/tx.go
type Tx interface {
    Do(ctx context.Context, fn func(ctx context.Context) error) error
}
```

The gorm-backed implementation lives in `repository/`; the service holds only
the interface.

## Error handling

- Services return typed errors (`ErrNotFound`, `ErrConflict`, `ErrForbidden`)
  from `internal/domain`.
- One central error-to-HTTP mapper in `handler/`. Handlers never build error
  JSON inline. Call `handler.HandleError(c, err)`; binding failures go to
  `handler.HandleBindingError(c, err)`.
- Never `panic` in a request path. Recovery middleware is a last resort only.
- Errors wrapped with `fmt.Errorf("doing x: %w", err)`.
- Repositories translate gorm errors into domain errors. Nothing above
  `repository/` may reference `gorm.ErrRecordNotFound`.

## API

- Routes: `/api/v1/...`. Health probes sit outside the version prefix.
- Response shape: `{ "data": ..., "meta": ... }` or
  `{ "error": { "code", "message" } }`
- Validate input with binding tags; return 422 with field-level errors.
- List endpoints are always paginated (`?page=&limit=`, max limit 100).

## Queries

- Every query that touches user data must be scoped by user/tenant.
- Use `Preload` to avoid N+1. Flag any loop that queries inside it.

## Testing

- Every service function gets a table-driven test.
- Repository tests use a real Postgres via testcontainers, not mocks. They call
  `migration.Run(db)` so CHECK constraints and ON DELETE rules are present —
  a bare `AutoMigrate` would not create them.
- Run `go test ./...` and `go vet ./...` before saying a task is done.

## Style

- `gofmt` always.
- Context is the first parameter on every service and repository method.
