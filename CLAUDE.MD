# Ecommerce Monorepo

Learning-focused Go + Next.js ecommerce project. The backend is a Gin/GORM API
over PostgreSQL and Redis, with versioned schema migrations written in Go. The
frontend is not scaffolded yet. Explain design decisions briefly before writing
code.

## Commands

- Infra (postgres + redis): `docker compose up -d`
- API: `cd backend && go run ./cmd/api`
- Migrate: `cd backend && go run ./cmd/migrate up` (also `down N`, `status`)
- New migration: append an entry to `all()` in
  `internal/migration/migrations.go`. There is no generator.
- Tests: `cd backend && go test ./...`
- Vet: `cd backend && go vet ./...`
- Frontend: not scaffolded — no `package.json` yet.

## Layout

- `backend/cmd/` — `api` (server) and `migrate` (schema CLI) entrypoints
- `backend/internal/` — handler, service, repository, model, dto, middleware,
  domain, config, database, migration, router, util
- `backend/internal/migration/migrations.go` — the ordered schema migrations.
  `backend/migrations/` is empty and unused.
- Tests live beside the code they cover (`*_test.go`)
- `frontend/` — Next.js App Router, TypeScript, Tailwind (not yet created)

## Global rules

- Plan before implementing. Show the plan, wait for my approval.
- One feature per session. Do not touch files outside the current task.
- Ask before adding any new dependency.
- Never write real secrets into code or committed files. Use `.env`.
- Money is `int64` cents everywhere, including JSON. Never float.
- After finishing a task, tell me what to test manually.

## Backend non-negotiables

- Layering is strict: handler → service → repository. Never skip a layer.
- Transactions are started in the service layer through the `Tx` interface in
  `internal/repository`. Services never import gorm.
- Never `panic` in a request path. Recovery middleware is a last resort only.
- `gofmt` always. Errors wrapped with `fmt.Errorf("doing x: %w", err)`.
- Context is the first parameter on every service and repository method.

## Teaching mode

When you introduce a Go or Next.js pattern I likely haven't used, add 2–3
lines explaining why it's used. I come from a Java/Spring background.
