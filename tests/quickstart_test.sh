#!/usr/bin/env bash
#
# Quickstart verification script.
#
# Replays every CLI command from QUICKSTART_CLIENT.md and QUICKSTART_SERVER.md
# in isolated temp directories and reports pass/fail for each step.
#
# Usage:
#   make build && ./tests/quickstart_test.sh
#
# Requires: the ticket binary at ./bin/ticket
#
set -euo pipefail

TICKET_BIN="$(cd "$(dirname "$0")/.." && pwd)/bin/ticket"
if [ ! -x "$TICKET_BIN" ]; then
  echo "FAIL: ticket binary not found at $TICKET_BIN (run 'make build' first)" >&2
  exit 1
fi

set +e  # Don't exit on individual test step failures

PASS=0
FAIL=0
RESULTS=()

step() {
  local label="$1"
  shift
  local out
  if out=$("$@" 2>&1); then
    RESULTS+=("  PASS  $label")
    PASS=$((PASS + 1))
  else
    RESULTS+=("  FAIL  $label  |  $out")
    FAIL=$((FAIL + 1))
  fi
}

step_output() {
  local label="$1"
  shift
  local out
  if out=$("$@" 2>&1); then
    RESULTS+=("  PASS  $label")
    PASS=$((PASS + 1))
    echo "$out"
  else
    RESULTS+=("  FAIL  $label")
    FAIL=$((FAIL + 1))
    echo "$out"
    return 1
  fi
}

# ── Local mode (QUICKSTART_CLIENT.md) ─────────────────────────────────

echo "=== QUICKSTART_CLIENT.md ==="
echo ""

LOCAL_DIR=$(mktemp -d)
export TICKET_HOME="$LOCAL_DIR/.ticket"
unset TICKET_URL 2>/dev/null || true

# 1. Initialise a workspace
step "local: tk init" "$TICKET_BIN" init

# 2. Create a project
step "local: tk project create" "$TICKET_BIN" project create -prefix CUS -title "Customer Portal"
step "local: tk project use CUS" "$TICKET_BIN" project use CUS

# 3. Capture work
step "local: tk add (task)" "$TICKET_BIN" add "Customers can reset their password"
step "local: tk bug" "$TICKET_BIN" bug "Reset token expires immediately"
step "local: tk epic" "$TICKET_BIN" epic "Authentication"
step "local: tk idea new" "$TICKET_BIN" idea new "Add dark mode"
step "local: tk idea ls" "$TICKET_BIN" idea ls

# 4. Inspect and organise
step "local: tk list" "$TICKET_BIN" list
step "local: tk get -id CUS-T-1" "$TICKET_BIN" get -id CUS-T-1
step "local: tk attach (set-parent)" "$TICKET_BIN" attach -id CUS-T-1 CUS-E-3

# 5. Lifecycle
step "local: tk active" "$TICKET_BIN" active -id CUS-T-1
step "local: tk complete" "$TICKET_BIN" complete -id CUS-T-1
step "local: tk idle" "$TICKET_BIN" idle -id CUS-T-1

# 6. TUI (just verify the binary accepts -g without crashing when stdin is not a tty)
# Cannot test interactively, so skip with a note
RESULTS+=("  SKIP  local: tk -g (interactive TUI, cannot automate)")

rm -rf "$LOCAL_DIR"

echo ""

# ── Server mode (QUICKSTART_SERVER.md) ────────────────────────────────

echo "=== QUICKSTART_SERVER.md ==="
echo ""

SERVER_DIR=$(mktemp -d)
export TICKET_HOME="$SERVER_DIR/.ticket"
unset TICKET_URL 2>/dev/null || true

# 1. Initialise and start the server
step "server: tk init" "$TICKET_BIN" init

# Start server in background
"$TICKET_BIN" server >/dev/null 2>&1 &
SERVER_PID=$!

cleanup() {
  kill "$SERVER_PID" 2>/dev/null || true
  wait "$SERVER_PID" 2>/dev/null || true
  rm -rf "$SERVER_DIR"
}
trap cleanup EXIT

# Wait for server to be ready
READY=false
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/api/healthz >/dev/null 2>&1; then
    READY=true
    break
  fi
  sleep 0.2
done

if [ "$READY" = false ]; then
  RESULTS+=("  FAIL  server: wait for server ready")
  FAIL=$((FAIL + 1))
  # Print results and exit early
  echo ""
  echo "=== QUICKSTART VERIFICATION REPORT ==="
  echo ""
  for r in "${RESULTS[@]}"; do echo "$r"; done
  echo ""
  echo "Total: $((PASS + FAIL)) | Pass: $PASS | Fail: $FAIL"
  exit 1
fi

step "server: healthz" curl -sf http://localhost:8080/api/healthz

# 2. Register and login (point CLI at running server)
export TICKET_URL=http://localhost:8080

step "server: tk register" "$TICKET_BIN" register -username alice -password secret
step "server: tk login" "$TICKET_BIN" login -username alice -password secret

# 3. Create a project
step "server: tk project create" "$TICKET_BIN" project create -prefix CUS -title "Customer Portal"
step "server: tk project use CUS" "$TICKET_BIN" project use CUS

# 4. Capture and organise work
step "server: tk add (task)" "$TICKET_BIN" add "Customers can reset their password"
step "server: tk bug" "$TICKET_BIN" bug "Reset token expires immediately"
step "server: tk epic" "$TICKET_BIN" epic "Authentication"
step "server: tk list" "$TICKET_BIN" list

# 5. Assign and claim
step "server: tk claim" "$TICKET_BIN" claim -id CUS-T-1

# request needs a ready develop/idle ticket — create one
"$TICKET_BIN" add "Request target" >/dev/null 2>&1
"$TICKET_BIN" complete -id CUS-T-3 >/dev/null 2>&1  # advance design -> develop
"$TICKET_BIN" ready -id CUS-T-3 >/dev/null 2>&1
step "server: tk request" "$TICKET_BIN" request

# 6. Agent create (just verify the command works; don't actually run the agent loop)
AGENT_OUT=$(step_output "server: tk agent create" "$TICKET_BIN" agent create) || true

# 7. Web UI (just verify the page loads)
step "server: web UI loads" curl -sf http://localhost:8080/

# Stop server
kill "$SERVER_PID" 2>/dev/null || true
wait "$SERVER_PID" 2>/dev/null || true
trap - EXIT
rm -rf "$SERVER_DIR"

# ── Report ────────────────────────────────────────────────────────────

echo ""
echo "=== QUICKSTART VERIFICATION REPORT ==="
echo ""
for r in "${RESULTS[@]}"; do echo "$r"; done
echo ""
TOTAL=$((PASS + FAIL))
echo "Total: $TOTAL | Pass: $PASS | Fail: $FAIL"
echo ""

if [ "$FAIL" -gt 0 ]; then
  echo "RESULT: FAIL"
  exit 1
else
  echo "RESULT: PASS"
  exit 0
fi
