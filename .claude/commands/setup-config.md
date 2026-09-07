---
description: Build the agent and skill config for this project
disable-model-invocation: true
---

Set up the Claude Code agent and skill configuration for this project. This
project already has a CLAUDE.md with detailed rules — most of your job is
splitting it correctly, not inventing content. Work in phases and stop for
my confirmation between each one.

## Phase 0 — Read and report

Write nothing in this phase.

**Read the existing `CLAUDE.md` in full.** It is the source of truth for
every backend and database rule. Do not invent conventions, and do not
replace my wording with generic advice.

Then inspect the repo and report:

- **Backend**: confirm Go version, Gin version, GORM version from `go.mod`.
  Confirm the actual package layout under `internal/` matches what CLAUDE.md
  describes (`handler/`, `service/`, `repository/`, `model/`, `dto/`,
  `middleware/`). Report any drift.
- **Frontend**: framework, build tool, and source directory. Check
  `package.json`. Report the data-fetching approach actually used in the
  code — fetch, axios, react-query, server components — by reading a real
  component, not by guessing.
- **Migrations**: confirm golang-migrate, the `migrations/` path, and the
  file naming pattern. List the last six migration files.
- **Commands**: find the real build, run, test, and migrate commands from
  the README, Makefile, `docker-compose.yml`, and `package.json` scripts.
  Report exact commands. Say "not found" rather than guessing.
- **Testcontainers**: confirm whether repository tests actually use it.

**Then flag this contradiction in CLAUDE.md and ask me to resolve it:**

> `repository/` is described as the only place that imports GORM, but
> services are also described as starting transactions and passing down
> `*gorm.DB`. A service cannot pass `*gorm.DB` without importing gorm.

Offer me two options and wait for my choice:

- **A**: relax the wording — repository is the only layer that *writes GORM
  queries*; services import gorm only for the `*gorm.DB` type.
- **B**: define a `Tx` interface in `internal/repository` so services never
  import gorm.

Report all of the above, then stop.

## Phase 1 — Split CLAUDE.md

The current CLAUDE.md loads in full on every turn, including content that
only matters in specific files. Split it.

**Rewrite `CLAUDE.md` to contain only:**

- One-paragraph project description
- A `## Commands` section with the real commands found in Phase 0 —
  run API, docker compose, `go test ./...`, `go vet ./...`, migrate up,
  create migration, and the frontend dev and build commands
- Project layout: where backend, frontend, migrations, and tests live
- The strict layering rule: handler → service → repository, never skip
- The resolved transaction rule from Phase 0
- Never `panic` in a request path
- `gofmt` always; errors wrapped with `fmt.Errorf("doing x: %w", err)`
- Context is the first parameter on every service and repository method

Target **under 60 lines**. Everything else moves to a skill.

While splitting, remove these duplications — each currently appears twice in
my file:

- "Never AutoMigrate"
- "Schema changes via golang-migrate"
- "Stock changes must be atomic with locking"
- "Models have no JSON tags"

Keep each rule once, in the skill where it belongs.

Show me the new CLAUDE.md and stop.

## Phase 2 — Skills

Create five skills at `.claude/skills/<name>/SKILL.md`.

### backend-conventions

```yaml
---
name: backend-conventions
description: Go backend conventions for this project — layering, error handling, API shape, testing
paths: "internal/**"
user-invocable: false
---
```

Move from CLAUDE.md, preserving my wording:

- The full layer responsibilities (what each package may and may not do)
- Error handling: typed domain errors, the central error-to-HTTP mapper,
  handlers never build error JSON inline
- API section: `/api/v1/...`, the `data`/`meta` and `error` response shapes,
  binding-tag validation returning 422 with field-level errors, mandatory
  pagination with max limit 100
- Testing: table-driven service tests, testcontainers for repository tests,
  `go test ./...` and `go vet ./...` before declaring done
- Manual dependency wiring in `cmd/api/main.go`, no DI framework
- Preload to avoid N+1; flag any loop that queries inside it
- Every query touching user data scoped by user/tenant

### db-conventions

```yaml
---
name: db-conventions
description: Database and migration rules — schema, money, historical records, locking
paths: "migrations/**, internal/model/**, internal/repository/**"
user-invocable: false
---
```

Move the entire `## Data model` section verbatim, plus the migration rules.
Do not summarize or reword these — they are precise on purpose:

- MVP table list, nothing else without asking
- uuid primary keys with `gen_random_uuid()`
- Money as `int64` cents / `BIGINT`, columns ending `_cents`, never float,
  never NUMERIC, never a string; format only at the API boundary
- `order_items` stores its own `product_name` and `unit_price_cents`; never
  join to products to display a past order
- `cart_items` stores only `product_id` and quantity; reads current price at
  read time
- Products soft-deleted with `deleted_at`, never hard-deleted
- `created_at` / `updated_at` timestamptz not null on every table
- Foreign keys declared with explicit ON DELETE; do not rely on GORM
  associations for integrity
- `products.stock` is the source of truth; changes only inside a transaction
  with row locking or a conditional UPDATE; never read-compute-write
- `orders.status` fixed set: pending, paid, shipped, delivered, cancelled;
  adding one requires a migration and my approval
- Every schema change is a golang-migrate pair with a working `down`
- Never AutoMigrate, in any environment, including tests
- One table or one tightly-coupled pair per migration
- GORM structs in `internal/model` with no json tags; dto types in
  `internal/dto`; handlers are the only layer touching dto

### frontend-conventions

```yaml
---
name: frontend-conventions
description: Frontend conventions for this project
paths: "<globs matching the frontend source, from Phase 0>"
user-invocable: false
---
```

Base every rule on components you actually read in Phase 0. Cover component
structure, data fetching, error and loading states, form handling, and how
the `data`/`meta` and `error` envelopes from the backend are consumed.
Include how money in cents is formatted for display, since the backend never
formats it.

### commit

```yaml
---
name: commit
description: Stage and commit current changes
disable-model-invocation: true
allowed-tools: Bash(git add *) Bash(git commit *) Bash(git status *) Bash(git diff *)
---
```

Body starts with a dynamic context line:

```
Changes: !`git diff HEAD --stat`
```

Then: write a conventional-commit message, stage the relevant files, commit.
Never commit `.env` or credentials.

### local-setup

Default invocation. Step-by-step from clean checkout to running app, using
the exact commands from Phase 0: docker compose for Postgres, migrate up,
`go run ./cmd/api`, frontend install and dev. Include a verification step
after each so I know it worked.

Stop and wait for my review.

## Phase 3 — Subagents

Six project agents at `.claude/agents/`, one personal at `~/.claude/agents/`.

| File | tools | model |
| :--- | :--- | :--- |
| `api-contract-check.md` | `Read, Grep, Glob, Bash` | `sonnet` |
| `fullstack-tracer.md` | `Read, Grep, Glob, Bash` | `inherit` |
| `code-reviewer.md` | `Read, Grep, Glob, Bash` | `inherit` |
| `db-migration.md` | `Read, Write, Grep, Glob, Bash` | `sonnet` |
| `test-writer.md` | `Read, Write, Edit, Grep, Glob, Bash` | `sonnet` |
| `perf-investigator.md` | `Read, Grep, Glob, Bash` | `sonnet` |
| `security-engineer.md` (personal) | `Read, Grep, Glob` | `sonnet` |

### db-migration

Only agent with a `skills:` field — `skills: [db-conventions]`. It creates
files from scratch with no local example in front of it, so the conventions
must be preloaded.

Body includes a dynamic context line:

```
Existing migrations: !`ls migrations | tail -6`
```

Instructions: create the next sequential golang-migrate pair, `.up.sql` and
`.down.sql`. The down must fully reverse the up. Never modify an existing
migration. Never AutoMigrate.

It has `Write` but deliberately not `Edit`, so it cannot alter an applied
migration.

### code-reviewer

Give it this project-specific checklist, not generic review advice:

- Any GORM query outside `internal/repository`
- Loops containing queries — should be `Preload`
- Stock read-then-write without row locking or a conditional UPDATE
- User-scoped queries missing the tenant filter
- float or NUMERIC used for money instead of `int64` cents
- `order_items` joining to products for display data
- New foreign key missing explicit ON DELETE
- Handlers building error JSON inline instead of using the central mapper
- `panic` in a request path
- Missing `context.Context` as first parameter
- Unwrapped errors
- List endpoint without pagination or without the max-limit-100 cap
- New `orders.status` value not backed by a migration

Report as CRITICAL / WARNING / SUGGESTION with file:line and a concrete fix.

### api-contract-check

Compare Go dto structs against frontend types. Look for:

- Field name and casing mismatch between `json` tags on dto structs and
  frontend types
- Nullability: Go pointer or `omitempty` fields the frontend declares as
  required
- Money fields the frontend treats as a float or formatted string instead of
  int cents
- `orders.status` values the frontend does not handle
- Status codes 401, 409, 422 the frontend does not branch on
- Envelope drift: frontend reading a bare body instead of `data`/`meta`
- Fields the frontend reads that the dto no longer returns

Report backend file:line, frontend file:line, the mismatch, minimal fix.

### The other agents

- **fullstack-tracer** — trace a symptom across layers: frontend call →
  handler → service → repository → migration. Gather app logs and container
  state before concluding. Report the originating layer with an evidence
  chain, and say what could not be verified.
- **test-writer** — read existing tests first to match style. Table-driven
  for services, testcontainers for repositories. Cover error paths.
- **perf-investigator** — measure before recommending. N+1, missing indexes,
  unbounded result sets, missing Preload.
- **security-engineer** — auth flows, input handling, data exposure,
  injection, missing authorization checks, hardcoded secrets, tenant
  isolation failures. Read-only.

Every agent body: numbered steps for what to do when invoked. Read-only
agents get an explicit instruction not to modify files.

## Constraints throughout

- `category` is not a valid Claude Code frontmatter field. Do not use it.
- No `## Boundaries` or `Will Not` prose in any agent. The `tools` field
  enforces restrictions; prose only requests them.
- Agent bodies under 60 lines. Skills under 150.
- `description` fields are trigger text in words I would type, not job
  titles.
- No two descriptions may claim the same territory.
- Real paths and real commands only. If you are about to write a
  placeholder, stop and ask me.
- Do not reword the Data model rules. Move them intact.

## Finally

Print a tree of everything created, then tell me to:

1. Restart Claude Code — `.claude/agents/` and `.claude/skills/` did not
   exist at session start, and a running session does not watch a newly
   created top-level directory.
2. Run `claude plugin validate .claude/skills` and
   `claude plugin validate .claude/agents`.
3. Run `/agents` and `/skills` to confirm everything loaded.
