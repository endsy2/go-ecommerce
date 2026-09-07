#!/usr/bin/env bash
# Auto-formats the single file Claude just edited.
# Input: PostToolUse JSON on stdin.
# Always exits 0 — formatting problems should never interrupt a session.

set -uo pipefail

input=$(cat)

# Pull the edited file path out of the JSON Claude Code sent us.
file=$(jq -r '.tool_input.file_path // empty' <<<"$input")

# Nothing to do if the tool didn't touch a file, or the file is gone.
[ -z "$file" ] && exit 0
[ -f "$file" ] || exit 0

case "$file" in
  *.go)
    command -v gofmt >/dev/null 2>&1 && gofmt -w "$file"
    ;;
  *.ts|*.tsx|*.js|*.jsx|*.json|*.css)
    if [ -d "${CLAUDE_PROJECT_DIR}/frontend/node_modules" ]; then
      cd "${CLAUDE_PROJECT_DIR}/frontend" || exit 0
      npx --no-install prettier --write "$file" >/dev/null 2>&1
    fi
    ;;
esac

exit 0