---
name: commit
description: Stage and commit current changes
disable-model-invocation: true
allowed-tools: Bash(git add:*), Bash(git commit:*), Bash(git status:*), Bash(git diff:*)
---

Changes: !`git status --short`

# Commit

1. Read the changes above. If empty, say so and stop.
2. Run `git diff` (and `git diff --staged`) to see what actually changed.
3. Group the changes into one logical commit. If they are clearly two unrelated
   changes, say so and ask which to commit rather than bundling them.
4. Write a conventional-commit message: `type(scope): subject`, imperative mood,
   under 72 characters. Types: feat, fix, refactor, test, docs, chore, perf.
   Scope is the package or area — `repository`, `migration`, `auth`, `config`.
   Add a body only when the change needs a why, not a what.
5. Stage only the files belonging to that commit with explicit paths.
   Never `git add -A` or `git add .`.
6. Never stage `.env`, anything matching `*.pem` / `*.key`, or a file containing
   a credential. If one appears in the changes, stop and report it.
7. Commit, then run `git status` to confirm a clean result.

Do not push.
