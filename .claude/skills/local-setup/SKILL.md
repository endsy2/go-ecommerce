---
name: local-setup
description: Get this project running from a clean checkout — infra, env, migrations, API
---

# Local setup

Run each step, verify it, and stop if a verification fails. Do not continue past
a failed step.

## 1. Start infra

```bash
docker compose up -d
```

Verify — both containers must report `healthy`, not just `running`. Postgres
accepts connections a few seconds after the container starts:

```bash
docker compose ps
```

## 2. Create the backend env file

```bash
cp backend/.env.example backend/.env
```

Verify: `backend/.env` exists and is gitignored (`git check-ignore backend/.env`
prints the path). The defaults match docker-compose, so no edits are needed for
local work. `JWT_SECRET` must be at least 32 characters or the app refuses to
start.

## 3. Apply migrations

Run from `backend/` — `.env` is resolved relative to the working directory.

```bash
cd backend && go run ./cmd/migrate up
```

Verify — every entry should be listed as applied:

```bash
cd backend && go run ./cmd/migrate status
```

## 4. Run the API

```bash
cd backend && go run ./cmd/api
```

In development `POSTGRES_AUTO_MIGRATE=true`, so this also applies pending
migrations on startup. Verify from a second terminal:

```bash
curl http://localhost:8080/health/ready
```

`/health` is liveness only; `/health/ready` is the one that pings Postgres and
Redis. Redis being down degrades the app but does not stop it.

## 5. Run the tests

```bash
cd backend && go test ./...
cd backend && go vet ./...
```

Repository tests start a real Postgres with testcontainers, so Docker must be
running. They are slower than the service tests — that is expected.

## 6. Frontend

Not scaffolded yet. There is no `frontend/package.json`, so there is no install
or dev command. `frontend/CLAUDE.MD` records the intended conventions.
