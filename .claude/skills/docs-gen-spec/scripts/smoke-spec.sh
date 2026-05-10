#!/usr/bin/env bash
# smoke-spec.sh — end-to-end check that a docs-generator spec renders cleanly.
#
# Runs three layers in order, fails fast on the first red light:
#   1. JSON Schema validate
#   2. Semantic lint (errors only — warnings ignored)
#   3. Boot the server, fetch /docs, assert browser-side invariants:
#        - HTTP 200
#        - top-level JSON.parse(...) calls are guarded by `|| []`
#        - every auth-method radio group has a default-checked option
#        - no `undefined<token>` artifacts in the credentials panel
#
# Exit codes:
#   0 — all green
#   1 — at least one check failed
#   2 — usage / setup error (missing spec, no docs-gen, no python3, ...)

set -euo pipefail

usage() {
  cat <<EOF
Usage: smoke-spec.sh <path-to-spec.yaml>
       smoke-spec.sh --help

Runs validate + lint + render-time invariant checks against a docs-generator
spec. See script header for the full invariant list.

Requires either:
  - \`docs-gen\` on PATH, or
  - this script to live inside a docs-generator checkout (so \`go run ./cmd/server\` works).
Also requires \`curl\` and \`python3\` (for free-port discovery).
EOF
}

if [[ $# -lt 1 || "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 2
fi

SPEC="$1"
[[ -f "$SPEC" ]] || { echo "ERROR: spec not found: $SPEC" >&2; exit 2; }
SPEC_ABS="$(cd "$(dirname "$SPEC")" && pwd)/$(basename "$SPEC")"

# Resolve script path through any symlink chain so we can find the docs-generator
# repo root even when invoked via the global ~/.claude/skills/ symlink.
script_path="${BASH_SOURCE[0]}"
while [[ -L "$script_path" ]]; do
  d="$(cd -P "$(dirname "$script_path")" && pwd)"
  script_path="$(readlink "$script_path")"
  [[ "$script_path" != /* ]] && script_path="$d/$script_path"
done
SCRIPT_DIR="$(cd -P "$(dirname "$script_path")" && pwd)"
# Layout: <repo>/.claude/skills/docs-gen-spec/scripts/smoke-spec.sh → climb 4 levels.
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"

# Decide how to invoke docs-gen. Prefer installed binary; fall back to source.
if command -v docs-gen >/dev/null 2>&1; then
  RUN_CWD="$PWD"
  RUN=(docs-gen)
elif [[ -d "$REPO_ROOT/cmd/server" ]]; then
  RUN_CWD="$REPO_ROOT"
  RUN=(go run ./cmd/server)
else
  echo "ERROR: neither \`docs-gen\` on PATH nor docs-generator repo at $REPO_ROOT" >&2
  exit 2
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "ERROR: curl is required" >&2
  exit 2
fi

# Pick a free port.
pick_port() {
  if command -v python3 >/dev/null 2>&1; then
    python3 -c "import socket; s=socket.socket(); s.bind(('',0)); p=s.getsockname()[1]; s.close(); print(p)"
  elif command -v python >/dev/null 2>&1; then
    python -c "import socket; s=socket.socket(); s.bind(('',0)); p=s.getsockname()[1]; s.close(); print(p)"
  else
    echo "ERROR: python3 (or python) required for free-port discovery" >&2
    exit 2
  fi
}
PORT="$(pick_port)"

pass() { printf "  \033[32m✓\033[0m %s\n" "$1"; }
fail() { printf "  \033[31m✗\033[0m %s\n" "$1"; FAILED=1; }

FAILED=0

# Working dir for `go run ./cmd/server` — but use absolute spec path so cd doesn't break refs.
cd "$RUN_CWD"

echo "[1/3] validate"
VALIDATE_LOG="$(mktemp -t docsgen-smoke-validate.XXXXXX)"
if "${RUN[@]}" validate "$SPEC_ABS" >"$VALIDATE_LOG" 2>&1; then
  pass "schema valid"
else
  cat "$VALIDATE_LOG" >&2
  fail "schema invalid"
fi
rm -f "$VALIDATE_LOG"

echo "[2/3] lint (errors only)"
LINT_LOG="$(mktemp -t docsgen-smoke-lint.XXXXXX)"
if "${RUN[@]}" lint "$SPEC_ABS" >"$LINT_LOG" 2>&1; then
  pass "lint clean"
else
  cat "$LINT_LOG" >&2
  fail "lint error(s)"
fi
rm -f "$LINT_LOG"

# If schema or lint already failed, skip render — output would be misleading.
if [[ "$FAILED" -ne 0 ]]; then
  echo
  echo "✗ smoke FAILED early: $SPEC_ABS"
  exit 1
fi

echo "[3/3] render"
LOG="$(mktemp -t docsgen-smoke-server.XXXXXX)"
HTML="$(mktemp -t docsgen-smoke-html.XXXXXX)"
SERVER_PID=""

cleanup() {
  if [[ -n "$SERVER_PID" ]]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -f "$LOG" "$HTML"
}
trap cleanup EXIT

"${RUN[@]}" -spec "$SPEC_ABS" -port "$PORT" >"$LOG" 2>&1 &
SERVER_PID=$!

# Wait up to 30s for /docs/health (`go run` may need to compile first).
ready=0
for _ in $(seq 1 150); do
  if curl -sf "http://localhost:$PORT/docs/health" >/dev/null 2>&1; then
    ready=1
    break
  fi
  # Server died early?
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    break
  fi
  sleep 0.2
done

if [[ "$ready" -ne 1 ]]; then
  fail "server failed to become ready on port $PORT in 30s"
  echo "--- server log ---" >&2
  cat "$LOG" >&2
  echo "--- end log ---" >&2
  exit 1
fi
pass "server boot (port $PORT)"

http_status="$(curl -s -o "$HTML" -w "%{http_code}" "http://localhost:$PORT/docs")"
if [[ "$http_status" == "200" ]]; then
  pass "GET /docs → 200"
else
  fail "GET /docs returned HTTP $http_status"
fi

# Invariant: top-level config parses (`baseUrlsConfig`, `authModesConfig`) MUST
# include the `|| []` guard. A bare `JSON.parse("null")` here crashes the page
# in `loadCredentials`.
unsafe_top_parse="$(grep -E '^[[:space:]]*const (baseUrlsConfig|authModesConfig) = JSON\.parse' "$HTML" | grep -vF '|| []' || true)"
if [[ -z "$unsafe_top_parse" ]]; then
  pass "top-level JSON.parse calls guarded by \`|| []\`"
else
  echo "  unsafe lines:" >&2
  printf '    %s\n' "$unsafe_top_parse" >&2
  fail "top-level JSON.parse(...) without \`|| []\` guard"
fi

# Invariant: each auth-method radio group must have at least one default-checked
# radio. Without one, updateAuthDisplay() crashes on `:checked.value`.
groups="$(grep -oE 'name="auth-method-[0-9]+-[0-9]+"' "$HTML" | sort -u || true)"
if [[ -z "$groups" ]]; then
  pass "no inline tester radio groups (skipped)"
else
  missing=""
  while IFS= read -r g; do
    [[ -z "$g" ]] && continue
    if ! grep -E "${g}[^>]*checked" "$HTML" >/dev/null; then
      missing+=" $g"
    fi
  done <<<"$groups"
  if [[ -z "$missing" ]]; then
    pass "every auth-method radio group has a default-checked option"
  else
    echo "  groups missing default check:$missing" >&2
    fail "auth-method radio group(s) without a default-checked radio"
  fi
fi

# Invariant: credentials panel must not render literal `undefined<value>` —
# that artifact comes from omitting `prefix` in an auth_modes[] entry.
if grep -E '<small[^>]*>[^<]*: undefined' "$HTML" >/dev/null 2>&1; then
  fail "credentials panel rendered 'undefined' (missing \`prefix\` on an auth_modes[] entry)"
else
  pass "no \`undefined\` artifacts in credentials panel"
fi

echo
if [[ "$FAILED" -ne 0 ]]; then
  echo "✗ smoke FAILED: $SPEC_ABS"
  exit 1
fi

echo "✓ smoke passed: $SPEC_ABS"
exit 0
