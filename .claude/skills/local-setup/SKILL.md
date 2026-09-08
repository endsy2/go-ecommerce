---
name: local-setup
description: Get this project running from a clean checkout — infra, env, migrations, API
---

# Local setup

Run each step, verify it, and stop if a verification fails. Do not continue past
a failed step.

## 1. Create the backend env file

First, not last: compose reads this file for the API service and refuses to
start the stack without it.

```bash
cp backend/.env.example backend/.env
```

Verify: `backend/.env` exists and is gitignored (`git check-ignore backend/.env`
prints the path). The defaults match docker-compose, so no edits are needed for
local work. `JWT_SECRET` must be at least 32 characters or the app refuses to
start.

## 2. Start the stack

Three services: postgres, redis, and the API.

```bash
docker compose up -d
```

Verify — all three containers must report `healthy`, not just `running`.
Postgres accepts connections a few seconds after its container starts, and the
API waits for it before booting:

```bash
docker compose ps
```

The API's health definition lives in `backend/Dockerfile`, not in
docker-compose.yml, so `docker inspect` reports the same state this does.

To work on the API with a fast edit loop, skip the container and run it on the
host against the same databases — see step 4.

## 3. Check the schema

`POSTGRES_AUTO_MIGRATE` defaults to true in development, so the API applied
every pending migration while starting in step 2. Confirm rather than re-apply:

```bash
docker compose exec backend migrate status
```

The image ships `migrate` alongside `api`, so this runs the same versioned
migrations a deploy step would, at the same commit as the running API.

## 4. Verify the API answers

```bash
curl http://localhost:8080/health/ready
```

`/health` is liveness only; `/health/ready` is the one that pings Postgres and
Redis. Redis being down degrades the app but does not stop it, so this still
returns 200 with `"status": "degraded"` in that case.

To work on the API instead of just running it, stop the container and run it on
the host — it uses the same databases through their published ports:

```bash
docker compose stop backend        # frees port 8080
cd backend && go run ./cmd/api
```

After a code change, either rebuild the image (`docker compose up -d --build
backend`) or restart the host process.

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
