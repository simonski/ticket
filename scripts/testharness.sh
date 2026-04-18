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

extract_numeric_id() {
	local text="$1"
	local id
	id="$(printf '%s\n' "$text" | sed -nE 's/.*\(id ([0-9]+)\).*/\1/p; s/.*#([0-9]+).*/\1/p' | head -n1)"
	if [[ -z "$id" ]]; then
		printf 'could not extract numeric id from: %s\n' "$text" >&2
		exit 1
	fi
	printf '%s' "$id"
}

expect_contains() {
	local haystack="$1"
	local needle="$2"
	local label="$3"
	if [[ "$haystack" != *"$needle"* ]]; then
		printf '%s: expected to find %q in output\n%s\n' "$label" "$needle" "$haystack" >&2
		exit 1
	fi
}

run "$TK_BIN" initdb -f "$DB_PATH" -force -password admin >/dev/null
run "$TK_BIN" project use 1 >/dev/null

log "scenario: basic count assertions"
parent_id="$("$TK_BIN" new foo -id 1 -printid)"
expect_ticket_suffix "$parent_id" "1" "parent ticket id"

run "$TK_BIN" ls -count -expect_equals 1
expect_fail "$TK_BIN" ls -count -expect_equals 0

child_id="$("$TK_BIN" new child -id 2 -parent "$parent_id" -printid)"
expect_ticket_suffix "$child_id" "2" "child ticket id"

run "$TK_BIN" ls -count -expect_equals 2

run "$TK_BIN" initdb -f "$DB_PATH" -force -password admin >/dev/null
run "$TK_BIN" project use 1 >/dev/null

log "scenario: sdlc next/previous workflow"
sdlc_id="$("$TK_BIN" sdlc create -name "review-flow" -printid)"

stage_output="$("$TK_BIN" sdlc stage-add -id "$sdlc_id" -name design -order 0)"
design_stage_id="$(extract_numeric_id "$stage_output")"
stage_output="$("$TK_BIN" sdlc stage-add -id "$sdlc_id" -name test -order 1)"
test_stage_id="$(extract_numeric_id "$stage_output")"
stage_output="$("$TK_BIN" sdlc stage-add -id "$sdlc_id" -name done -order 2)"
done_stage_id="$(extract_numeric_id "$stage_output")"

role_output="$("$TK_BIN" sdlc role-add -sdlc_id "$sdlc_id" -title designer)"
designer_role_id="$(extract_numeric_id "$role_output")"
role_output="$("$TK_BIN" sdlc role-add -sdlc_id "$sdlc_id" -title tester)"
tester_role_id="$(extract_numeric_id "$role_output")"

run "$TK_BIN" sdlc stage-role-add -sdlc_id "$sdlc_id" -stage_id "$design_stage_id" -role_id "$designer_role_id" >/dev/null
run "$TK_BIN" sdlc stage-role-add -sdlc_id "$sdlc_id" -stage_id "$test_stage_id" -role_id "$tester_role_id" >/dev/null
run "$TK_BIN" project sdlc "$sdlc_id" >/dev/null

ticket_id="$("$TK_BIN" add "Workflow ticket" -printid)"
ticket_output="$("$TK_BIN" get -id "$ticket_id")"
expect_contains "$ticket_output" "Status              : design/idle" "initial workflow status"

run "$TK_BIN" update -id "$ticket_id" -status design/success >/dev/null
ticket_output="$("$TK_BIN" get -id "$ticket_id")"
expect_contains "$ticket_output" "Status              : test/idle" "ticket after success"

run "$TK_BIN" update -id "$ticket_id" -status test/fail >/dev/null
previous_output="$("$TK_BIN" previous -id "$ticket_id")"
expect_contains "$previous_output" "test/fail -> design/idle" "previous output"

ticket_output="$("$TK_BIN" get -id "$ticket_id")"
expect_contains "$ticket_output" "Status              : design/idle" "ticket after previous"

run "$TK_BIN" update -id "$ticket_id" -status design/success >/dev/null
run "$TK_BIN" update -id "$ticket_id" -status test/success >/dev/null
ticket_output="$("$TK_BIN" get -id "$ticket_id")"
expect_contains "$ticket_output" "Status              : done/idle" "ticket after test success"

run "$TK_BIN" success -id "$ticket_id" >/dev/null
ticket_output="$("$TK_BIN" get -id "$ticket_id")"
expect_contains "$ticket_output" "Status              : done/success" "ticket after done success"

run "$TK_BIN" next -id "$ticket_id" >/dev/null
ticket_output="$("$TK_BIN" get -id "$ticket_id")"
expect_contains "$ticket_output" "Complete            : closed" "ticket complete flag"

log "all script harness scenarios passed"
