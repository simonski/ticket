#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TK_BIN="${TK_BIN:-$ROOT_DIR/bin/tk}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ticket-testharness.XXXXXX")"
TICKET_HOME_DIR="$WORK_DIR/home"
DB_PATH="$WORK_DIR/harness.db"

cleanup() {
	rm -rf "$WORK_DIR"
}
trap cleanup EXIT

if [[ ! -x "$TK_BIN" ]]; then
	echo "tk binary not found at $TK_BIN" >&2
	exit 1
fi

export TICKET_HOME="$TICKET_HOME_DIR"
export TICKET_URL="$DB_PATH"
mkdir -p "$TICKET_HOME"

log() {
	printf '\n==> %s\n' "$1"
}

run() {
	printf '+'
	printf ' %q' "$@"
	printf '\n'
	"$@"
}

expect_fail() {
	printf '+'
	printf ' %q' "$@"
	printf '  # expect failure\n'
	if "$@"; then
		echo "expected command to fail but it succeeded" >&2
		exit 1
	fi
}

expect_equal() {
	local got="$1"
	local want="$2"
	local label="$3"
	if [[ "$got" != "$want" ]]; then
		printf '%s: got %q, want %q\n' "$label" "$got" "$want" >&2
		exit 1
	fi
}

expect_ticket_suffix() {
	local got="$1"
	local suffix="$2"
	local label="$3"
	case "$got" in
		*-"$suffix") ;;
		*)
			printf '%s: got %q, want ticket key ending in -%s\n' "$label" "$got" "$suffix" >&2
			exit 1
			;;
	esac
}

scenario_basic_count_assertions() {
	log "scenario: basic count assertions"
	run "$TK_BIN" initdb -f "$DB_PATH" -force -password admin >/dev/null

	local parent_id
	parent_id="$("$TK_BIN" new foo -id 1 -printid)"
	expect_ticket_suffix "$parent_id" "1" "parent ticket id"

	run "$TK_BIN" ls -count -expect_equals 1
	expect_fail "$TK_BIN" ls -count -expect_equals 0

	local child_id
	child_id="$("$TK_BIN" new child -id 2 -parent "$parent_id" -printid)"
	expect_ticket_suffix "$child_id" "2" "child ticket id"

	run "$TK_BIN" ls -count -expect_equals 2
}

scenario_basic_count_assertions

log "all script harness scenarios passed"
