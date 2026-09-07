# Ecommerce Monorepo

Learning-focused project. Explain design decisions briefly before writing code.

## Layout
- `backend/` — Go 1.23, Gin, GORM, PostgreSQL, Redis
- `frontend/` — Next.js App Router, TypeScript, Tailwind
- `docker-compose.yml` — postgres + redis for local dev

See `backend/CLAUDE.md` and `frontend/CLAUDE.md` for stack-specific rules.

## Global rules
- Plan before implementing. Show the plan, wait for my approval.
- One feature per session. Do not touch files outside the current task.
- Ask before adding any new dependency.
- Never write real secrets into code or committed files. Use `.env`.
- Money is `int64` cents everywhere, including JSON. Never float.
- After finishing a task, tell me what to test manually.

## Commands
- Infra: `docker compose up -d`
- Backend: `cd backend && go run ./cmd/api`
- Backend tests: `cd backend && go test ./...`
- Frontend: `cd frontend && npm run dev`

## Teaching mode
When you introduce a Go or Next.js pattern I likely haven't used, add 2–3
lines explaining why it's used. I come from a Java/Spring background.