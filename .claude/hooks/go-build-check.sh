#!/usr/bin/env bash
# Compiles the backend after Claude edits a .go file.
# On failure: exit 2, which shows the compiler error TO CLAUDE so it fixes it now.

set -uo pipefail

input=$(cat)
file=$(jq -r '.tool_input.file_path // empty' <<<"$input")

# Only care about Go files.
case "$file" in
  *.go) ;;
  *) exit 0 ;;
esac

cd "${CLAUDE_PROJECT_DIR}/backend" 2>/dev/null || exit 0

# No go.mod yet (early in the project) — nothing to build.
[ -f go.mod ] || exit 0

if ! output=$(go build ./... 2>&1); then
  {
    echo "Backend build failed after editing ${file}:"
    echo "$output"
  } >&2
  exit 2
fi

exit 0