# Prompt: scaffold Claude Code agents and skills

Paste everything below the line into Claude Code, from your project root.

---

Set up a Claude Code agent and skill configuration for this project. Work in
phases and stop for my confirmation between each one.

## Phase 0 — Inspect before you write

Do not guess any convention. Read the repository first and report what you
find. Do not create a single file in this phase.

Detect and report:

- **Backend**: language, framework, version (check `pom.xml`, `build.gradle`,
  `package.json`, `go.mod`, `requirements.txt`, `pyproject.toml`)
- **Frontend**: framework and build tool — React + Vite, Next.js, Vue, or
  something else. Check `package.json` scripts and config files
- **Database**: engine, and the migration tool — Flyway, Liquibase, Prisma,
  Alembic, Drizzle, or none. Find where migration files live and how they
  are named
- **Messaging / cache**: RabbitMQ, Kafka, Redis, or none
- **Build and test commands**: read the README, `Makefile`, `package.json`
  scripts, and CI config. Report the exact commands, not guesses
- **Local run**: is there a `docker-compose.yml`? What does it start?
- **Existing conventions**, inferred from reading actual source files:
  - Dependency injection style used in the backend
  - Whether DTOs are records, classes, or interfaces
  - The error response shape actually returned by controllers
  - How the frontend fetches data (fetch, axios, react-query, server
    components)
  - Test framework and where tests live
  - Naming and casing at the API boundary (camelCase vs snake_case)

Report all of this as a short summary. Where you cannot determine something
from the code, say "not determined" rather than assuming. Then stop and wait
for me to confirm or correct before continuing.

## Phase 1 — CLAUDE.md

Create `CLAUDE.md` at the project root containing only facts that are true in
every session:

- One-paragraph project description
- Build, test, lint, and local-run commands, exactly as detected
- Project layout: where backend, frontend, migrations, and tests live
- Always-true rules, written as specific instructions

Keep it under 150 lines. Every line loads in every session, so include only
things you would otherwise have to re-explain. Rules must be concrete —
"use constructor injection, never field `@Autowired`" rather than "follow
good practices."

Stop and wait for my review.

## Phase 2 — Skills

Create these five, each at `.claude/skills/<name>/SKILL.md`.

### backend-conventions

```yaml
---
name: backend-conventions
description: <what it covers and when it applies>
paths: "<glob matching the backend source directory>"
user-invocable: false
---
```

Body: the backend conventions you detected in Phase 0. API design, DTO style,
error handling, transaction boundaries, logging. Base every rule on code you
actually read, and cite the file you took it from in a comment.

### frontend-conventions

Same shape, with `paths` matching the frontend source and extensions. Cover
component structure, data fetching, error and loading states, form handling,
and type conventions.

### db-conventions

`paths` matching the migration and entity directories. Cover the migration
tool's naming and versioning scheme, the rule that applied migrations are
never edited, index and constraint requirements, and nullability defaults.

### commit

```yaml
---
name: commit
description: Stage and commit current changes
disable-model-invocation: true
allowed-tools: Bash(git add *) Bash(git commit *) Bash(git status *) Bash(git diff *)
---
```

Body must start with a dynamic context line so the skill sees the real diff:

```
Changes: !`git diff HEAD --stat`
```

Then instructions to write a conventional-commit message, stage the relevant
files, and commit. Include an explicit rule never to commit `.env` files or
credentials.

### local-setup

Default invocation, no restrictions. Step-by-step instructions to get the
project running from a clean checkout, using the exact commands detected in
Phase 0. Include verification steps so I know each step worked.

Stop and wait for my review.

## Phase 3 — Subagents

Create six at `.claude/agents/<name>.md`, plus one personal agent at
`~/.claude/agents/security-engineer.md`.

Every agent needs `name`, `description`, `tools`, and `model`. Use these
exactly:

| File | tools | model |
| :--- | :--- | :--- |
| `api-contract-check.md` | `Read, Grep, Glob, Bash` | `sonnet` |
| `fullstack-tracer.md` | `Read, Grep, Glob, Bash` | `inherit` |
| `code-reviewer.md` | `Read, Grep, Glob, Bash` | `inherit` |
| `db-migration.md` | `Read, Write, Grep, Glob, Bash` | `sonnet` |
| `test-writer.md` | `Read, Write, Edit, Grep, Glob, Bash` | `sonnet` |
| `perf-investigator.md` | `Read, Grep, Glob, Bash` | `sonnet` |
| `security-engineer.md` | `Read, Grep, Glob` | `sonnet` |

Two agents need extra fields:

- `db-migration.md` gets `skills: [db-conventions]` so the conventions are
  preloaded, and a dynamic context line listing the most recent existing
  migrations so it knows the next version number. It has `Write` but
  deliberately not `Edit`, so it can create a migration but cannot modify an
  applied one.
- No other agent gets a `skills:` field. Preloading injects the full skill
  body at every startup, and the read-only agents do not write code, so it
  would be dead weight.

What each agent should do:

- **api-contract-check** — compare backend contracts against frontend
  consumers. Diff the branch, then for each changed controller or DTO find
  the frontend code consuming it, and vice versa. Look for field name and
  casing mismatches, nullability disagreements, enum values the frontend
  does not handle, status codes it does not branch on, and error envelope
  drift. Report backend file:line, frontend file:line, the mismatch, and the
  minimal fix.
- **fullstack-tracer** — trace a symptom across layers. Entry point, network
  call, controller, service, repository, queue, migration. Gather logs and
  container state before concluding. Report the originating layer with an
  evidence chain, and state clearly what could not be verified.
- **code-reviewer** — review the branch diff. Report findings as CRITICAL /
  WARNING / SUGGESTION with file references and a concrete fix for each.
- **db-migration** — create the next migration following the preloaded
  conventions.
- **test-writer** — write tests matching the existing test style, which it
  should read first rather than assume.
- **perf-investigator** — measure before recommending. Look for N+1 queries,
  missing indexes, unbounded result sets, and cache misses.
- **security-engineer** — check auth flows, input handling, and data
  exposure. Injection, missing authorization checks, hardcoded secrets,
  unsafe deserialization. Read-only.

Every agent body should state what to do when invoked, in numbered steps,
and should tell the read-only ones explicitly not to modify files.

## Constraints that apply throughout

- `category` is **not** a valid Claude Code frontmatter field. Do not use it.
- Do not write a `## Boundaries` or `Will Not` prose section in any agent.
  The `tools` field enforces restrictions; prose only requests them.
- Keep each agent body under 60 lines and each `SKILL.md` under 150.
- Write `description` fields as trigger text using words I would naturally
  type, not resume language. "Use when a UI symptom does not match the API
  response" is good. "Systematically investigate complex problems through
  evidence-based analysis" is not.
- Make every description distinct enough that two agents never claim the same
  territory, otherwise delegation becomes unpredictable.
- Reference real paths and real commands from this repository. If you find
  yourself writing a placeholder, stop and ask me instead.

## Finally

Print a tree of everything created, then tell me:

1. To restart Claude Code, because `.claude/agents/` and `.claude/skills/`
   did not exist when this session started and a running session does not
   watch a newly created top-level directory.
2. To run `claude plugin validate .claude/skills` and
   `claude plugin validate .claude/agents` to catch frontmatter that does
   not parse.
3. To run `/agents` and `/skills` after restarting to confirm everything
   loaded.