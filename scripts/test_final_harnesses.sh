#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TK_BIN="${TK_BIN:-$ROOT_DIR/bin/tk}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ticket-final-shell.XXXXXX")"
SERVER_HOME="$WORK_DIR/server-home"
SERVER_PORT=$((20000 + RANDOM % 20000))
SERVER_ADDR="127.0.0.1:$SERVER_PORT"
SERVER_URL="http://$SERVER_ADDR"
SERVER_PID=""

cleanup() {
	if [[ -n "$SERVER_PID" ]]; then
		kill "$SERVER_PID" 2>/dev/null || true
		wait "$SERVER_PID" 2>/dev/null || true
	fi
	rm -rf "$WORK_DIR"
}
trap cleanup EXIT

if [[ ! -x "$TK_BIN" ]]; then
	echo "tk binary not found at $TK_BIN" >&2
	exit 1
fi

wait_for_http() {
	local url="$1"
	local label="$2"
	local i
	for ((i = 0; i < 50; i++)); do
		if curl -fsS "$url" >/dev/null 2>&1; then
			return 0
		fi
		sleep 0.2
	done
	printf '%s: timed out waiting for %s\n' "$label" "$url" >&2
	exit 1
}

mkdir -p "$SERVER_HOME"
env TICKET_HOME="$SERVER_HOME" "$TK_BIN" initdb -password adminpass >/dev/null
env TICKET_HOME="$SERVER_HOME" "$TK_BIN" server -f "$SERVER_HOME/ticket.db" -addr "$SERVER_ADDR" >/dev/null 2>&1 &
SERVER_PID=$!
wait_for_http "$SERVER_URL/api/healthz" "shared server startup"

env \
	TK_BIN="$TK_BIN" \
	TICKET_TEST_SERVER_URL="$SERVER_URL" \
	TICKET_TEST_SERVER_PASSWORD="adminpass" \
	"$ROOT_DIR/scripts/testharness.sh"

env \
	TK_BIN="$TK_BIN" \
	TICKET_TEST_SERVER_URL="$SERVER_URL" \
	TICKET_TEST_SERVER_PASSWORD="adminpass" \
	"$ROOT_DIR/scripts/verify_todo_example.sh"
