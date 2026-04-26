#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TK_BIN="${TK_BIN:-$ROOT_DIR/bin/tk}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ticket-testharness.XXXXXX")"
TICKET_HOME_DIR="$WORK_DIR/home"
REPO_DIR="$WORK_DIR/repo"
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

export TICKET_HOME="$TICKET_HOME_DIR"
mkdir -p "$TICKET_HOME"
mkdir -p "$REPO_DIR/.git"
cd "$REPO_DIR"
unset AGENT_ID AGENT_PASSWORD

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

reset_local_repo() {
	rm -rf "$REPO_DIR/.ticket" "$TICKET_HOME_DIR"
	mkdir -p "$REPO_DIR/.git" "$TICKET_HOME_DIR"
	unset AGENT_ID AGENT_PASSWORD
	run "$TK_BIN" initdb -password admin >/dev/null
	run "$TK_BIN" project init -prefix HAR -title "Harness Project" >/dev/null
}

reset_local_repo

log "scenario: basic count assertions"
parent_id="$("$TK_BIN" new foo -id 1 -printid)"
expect_ticket_suffix "$parent_id" "1" "parent ticket id"

run "$TK_BIN" ls -count -expect_equals 1
expect_fail "$TK_BIN" ls -count -expect_equals 0

child_id="$("$TK_BIN" new child -id 2 -parent "$parent_id" -printid)"
expect_ticket_suffix "$child_id" "2" "child ticket id"

run "$TK_BIN" ls -count -expect_equals 2

reset_local_repo

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

reset_local_repo

log "scenario: broad admin lifecycle and snapshot restore"
lifecycle_ticket_id="$("$TK_BIN" add "Lifecycle ticket" -printid)"
run "$TK_BIN" comment add -id "$lifecycle_ticket_id" "First comment from harness" >/dev/null
ticket_output="$("$TK_BIN" get -id "$lifecycle_ticket_id")"
expect_contains "$ticket_output" "First comment from harness" "ticket detail includes comment"

idea_id="$("$TK_BIN" idea new -printid "Add dark mode")"
run "$TK_BIN" idea revise -id "$idea_id" >/dev/null
idea_output="$("$TK_BIN" get -id "$idea_id")"
expect_contains "$idea_output" "(revised)" "idea revise alias"

decision_id="$("$TK_BIN" decision new -printid "Use PostgreSQL for production")"
decision_list_output="$("$TK_BIN" decision ls)"
expect_contains "$decision_list_output" "Use PostgreSQL for production" "decision ls alias"
expect_ticket_suffix "$decision_id" "3" "decision ticket id"

snapshot_file="$WORK_DIR/harness-snapshot.json"
run "$TK_BIN" export -o "$snapshot_file" >/dev/null
run "$TK_BIN" add "Temporary snapshot noise" >/dev/null
run "$TK_BIN" ls -count -expect_equals 4
run "$TK_BIN" import -i "$snapshot_file" >/dev/null
run "$TK_BIN" ls -count -expect_equals 3

log "scenario: remote multi-project and agent request flow"
SERVER_DB="$WORK_DIR/server.db"
SERVER_HOME="$WORK_DIR/server-home"
SERVER_PORT=$((20000 + RANDOM % 20000))
SERVER_ADDR="127.0.0.1:$SERVER_PORT"
SERVER_URL="http://$SERVER_ADDR"
SERVER_LOG="$WORK_DIR/server.log"

mkdir -p "$SERVER_HOME"
run env TICKET_HOME="$SERVER_HOME" "$TK_BIN" initdb -password adminpass >/dev/null
SERVER_DB="$SERVER_HOME/ticket.db"
env TICKET_HOME="$SERVER_HOME" "$TK_BIN" server -f "$SERVER_DB" -addr "$SERVER_ADDR" >"$SERVER_LOG" 2>&1 &
SERVER_PID=$!
wait_for_http "$SERVER_URL/api/healthz" "server startup"

export TICKET_HOME="$WORK_DIR/remote-client-home"
mkdir -p "$TICKET_HOME"
rm -rf "$REPO_DIR/.ticket"
mkdir -p "$REPO_DIR/.git"

unset AGENT_ID AGENT_PASSWORD

run "$TK_BIN" remote add harness "$SERVER_URL" >/dev/null
run "$TK_BIN" project remote harness >/dev/null
run "$TK_BIN" login -username admin -password adminpass >/dev/null
srv_project_id="$("$TK_BIN" project create -prefix SRV -title "Server Harness" -printid)"
ops_project_id="$("$TK_BIN" project create -prefix OPS -title "Ops Harness" -printid)"
run "$TK_BIN" project use SRV >/dev/null

remote_ticket_id="$("$TK_BIN" add -project "$srv_project_id" "Remote agent ticket" -printid)"
run "$TK_BIN" update -id "$remote_ticket_id" -status develop/idle >/dev/null
agent_id="$("$TK_BIN" agent create -password agentpass123 -printid)"
initial_agent_config="$("$TK_BIN" agent config-ls -id "$agent_id")"
expect_contains "$initial_agent_config" "(no config)" "initial agent config"
run "$TK_BIN" agent config-set -id "$agent_id" llm codex >/dev/null
run "$TK_BIN" agent config-set -id "$agent_id" poll_seconds 7 >/dev/null
agent_wrong_project_output="$("$TK_BIN" agent request -agent-id "$agent_id" -password agentpass123 -project-id "$ops_project_id")"
expect_contains "$agent_wrong_project_output" "\"NONE\"" "agent wrong project request status"
run "$TK_BIN" agent reset-password -id "$agent_id" -password newpass123 >/dev/null
expect_fail "$TK_BIN" agent request -agent-id "$agent_id" -password agentpass123 -project-id "$srv_project_id" -id "$remote_ticket_id"
agent_request_output="$("$TK_BIN" agent request -agent-id "$agent_id" -password newpass123 -project-id "$srv_project_id" -id "$remote_ticket_id")"
expect_contains "$agent_request_output" "\"status\"" "remote agent request response shape"
expect_contains "$agent_request_output" "\"NEW\"" "remote agent request status"
expect_contains "$agent_request_output" "Remote agent ticket" "remote agent request ticket"
expect_contains "$agent_request_output" "\"llm\": \"codex\"" "remote agent request config propagation"
run "$TK_BIN" agent config-rm -id "$agent_id" llm >/dev/null

run "$TK_BIN" project use OPS >/dev/null
run "$TK_BIN" ls -count -expect_equals 0
run "$TK_BIN" add "Ops project ticket" >/dev/null
run "$TK_BIN" ls -count -expect_equals 1
run "$TK_BIN" project use SRV >/dev/null
run "$TK_BIN" ls -count -expect_equals 1

log "all script harness scenarios passed"
