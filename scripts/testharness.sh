#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TK_BIN="${TK_BIN:-$ROOT_DIR/bin/tk}"
source "$ROOT_DIR/scripts/lib/token_auth.sh"
export TICKET_FAST_HASH="${TICKET_FAST_HASH:-1}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ticket-testharness.XXXXXX")"
REPO_DIR="$WORK_DIR/repo"
TICKET_HOME_DIR="$WORK_DIR/home"
SHARED_SERVER_URL="${TICKET_TEST_SERVER_URL:-}"
SHARED_SERVER_PASSWORD="${TICKET_TEST_SERVER_PASSWORD:-adminpass}"
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
mkdir -p "$REPO_DIR"
git -C "$REPO_DIR" init -q
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

log "scenario: remote multi-project and agent request flow"
if [[ -n "$SHARED_SERVER_URL" ]]; then
	SERVER_URL="$SHARED_SERVER_URL"
else
	SERVER_HOME="$WORK_DIR/server-home"
	SERVER_PORT=$((20000 + RANDOM % 20000))
	SERVER_ADDR="127.0.0.1:$SERVER_PORT"
	SERVER_URL="http://$SERVER_ADDR"
	SERVER_LOG="$WORK_DIR/server.log"

	mkdir -p "$SERVER_HOME"
	run env TICKET_HOME="$SERVER_HOME" "$TK_BIN" initdb -password "$SHARED_SERVER_PASSWORD" >/dev/null
	env TICKET_HOME="$SERVER_HOME" "$TK_BIN" server -f "$SERVER_HOME/ticket.db" -addr "$SERVER_ADDR" >"$SERVER_LOG" 2>&1 &
	SERVER_PID=$!
	wait_for_http "$SERVER_URL/api/healthz" "server startup"
fi

export TICKET_HOME="$WORK_DIR/remote-client-home"
mkdir -p "$TICKET_HOME"
rm -rf "$REPO_DIR/.ticket"

unset AGENT_ID AGENT_PASSWORD

export TICKET_URL="$SERVER_URL"
export TICKET_USERNAME="admin"
unset TICKET_TOKEN
export TICKET_PASSWORD="$SHARED_SERVER_PASSWORD"
use_token_auth
run "$TK_BIN" whoami >/dev/null
repo_remote="git@example.com:example/harness.git"
run git remote add origin "$repo_remote" >/dev/null
srv_project_id="$("$TK_BIN" project create -prefix SRV -title "Server Harness" -printid)"
ops_project_id="$("$TK_BIN" project create -prefix OPS -title "Ops Harness" -printid)"
repo_project_id="$("$TK_BIN" project create -prefix REP -title "Repo Harness" -git-repository "$repo_remote" -printid)"
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
run "$TK_BIN" count -project_id "$repo_project_id" -expect_equals 0
run "$TK_BIN" config rm project_id >/dev/null
run "$TK_BIN" add "Repo inferred ticket" >/dev/null
run "$TK_BIN" count -project_id "$repo_project_id" -expect_equals 1
run "$TK_BIN" count -project_id "$srv_project_id" -expect_equals 1

log "scenario: self-registration with public/private aliases"
export TICKET_HOME="$WORK_DIR/self-register-home"
mkdir -p "$TICKET_HOME"
unset TICKET_USERNAME TICKET_PASSWORD TICKET_PROJECT TICKET_TOKEN
register_output="$("$TK_BIN" register -username harness-user -email harness@example.com)"
expect_contains "$register_output" "registered user harness-user" "register output"
registered_password="$(printf '%s\n' "$register_output" | sed -n 's/^password: //p' | head -n1)"
if [[ -z "$registered_password" ]]; then
	printf 'register output missing generated password:\n%s\n' "$register_output" >&2
	exit 1
fi

export TICKET_USERNAME="harness-user"
export TICKET_PASSWORD="$registered_password"
use_token_auth
run "$TK_BIN" project use private >/dev/null
run "$TK_BIN" count -project_id private -expect_equals 0
run "$TK_BIN" add "Private alias ticket" >/dev/null
run "$TK_BIN" count -project_id private -expect_equals 1
export TICKET_PROJECT="private"
run "$TK_BIN" count -expect_equals 1
run "$TK_BIN" count -project_id public -type story -expect_equals 0
run "$TK_BIN" add -project public -type story "Public alias story" >/dev/null
run "$TK_BIN" count -project_id public -type story -expect_equals 1
unset TICKET_PROJECT

log "all script harness scenarios passed"
