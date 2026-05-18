#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TK_BIN="${TK_BIN:-$ROOT_DIR/bin/tk}"
source "$ROOT_DIR/scripts/lib/token_auth.sh"
export TICKET_FAST_HASH="${TICKET_FAST_HASH:-1}"

WORK_DIR=""
SERVER_PID=""

cleanup() {
	if [[ -n "$SERVER_PID" ]]; then
		kill "$SERVER_PID" 2>/dev/null || true
		wait "$SERVER_PID" 2>/dev/null || true
	fi
	if [[ -n "$WORK_DIR" ]]; then
		rm -rf "$WORK_DIR"
	fi
}
trap cleanup EXIT

usage() {
	cat <<'EOF'
usage: scripts/test_shell.sh <harness|todo-example|final>

  harness       run the CLI shell harness scenarios
  todo-example  seed and verify the reproducible todo example
  final         run both shell suites against one shared server
EOF
}

require_tk_bin() {
	if [[ ! -x "$TK_BIN" ]]; then
		echo "tk binary not found at $TK_BIN" >&2
		echo "Build it first: make build-dev" >&2
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

run_harness() {
	local repo_dir ticket_home_dir shared_server_url shared_server_password server_url
	local server_home server_port server_addr server_log
	local srv_project_id ops_project_id repo_project_id remote_ticket_id
	local agent_id initial_agent_config agent_wrong_project_output agent_request_output
	local register_output registered_password repo_remote

	require_tk_bin
	unset AGENT_ID AGENT_PASSWORD

	WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ticket-testharness.XXXXXX")"
	repo_dir="$WORK_DIR/repo"
	ticket_home_dir="$WORK_DIR/home"
	shared_server_url="${TICKET_TEST_SERVER_URL:-}"
	shared_server_password="${TICKET_TEST_SERVER_PASSWORD:-adminpass}"

	export TICKET_HOME="$ticket_home_dir"
	mkdir -p "$TICKET_HOME" "$repo_dir"
	git -C "$repo_dir" init -q
	cd "$repo_dir"

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

	expect_contains() {
		local haystack="$1"
		local needle="$2"
		local label="$3"
		if [[ "$haystack" != *"$needle"* ]]; then
			printf '%s: expected to find %q in output\n%s\n' "$label" "$needle" "$haystack" >&2
			exit 1
		fi
	}

	log "scenario: remote multi-project and agent request flow"
	if [[ -n "$shared_server_url" ]]; then
		server_url="$shared_server_url"
	else
		server_home="$WORK_DIR/server-home"
		server_port=$((20000 + RANDOM % 20000))
		server_addr="127.0.0.1:$server_port"
		server_url="http://$server_addr"
		server_log="$WORK_DIR/server.log"

		mkdir -p "$server_home"
		run env TICKET_HOME="$server_home" "$TK_BIN" initdb -password "$shared_server_password" >/dev/null
		env TICKET_HOME="$server_home" "$TK_BIN" server -f "$server_home/ticket.db" -addr "$server_addr" >"$server_log" 2>&1 &
		SERVER_PID=$!
		wait_for_http "$server_url/api/healthz" "server startup"
	fi

	export TICKET_HOME="$WORK_DIR/remote-client-home"
	mkdir -p "$TICKET_HOME"
	rm -rf "$repo_dir/.ticket"

	export TICKET_URL="$server_url"
	export TICKET_USERNAME="admin"
	unset TICKET_TOKEN
	export TICKET_PASSWORD="$shared_server_password"
	use_token_auth
	run "$TK_BIN" whoami >/dev/null
	repo_remote="git@example.com:example/harness.git"
	run git remote add origin "$repo_remote" >/dev/null
	srv_project_id="$("$TK_BIN" project create -prefix SRV -title "Server Harness" -printid)"
	ops_project_id="$("$TK_BIN" project create -prefix OPS -title "Ops Harness" -printid)"
	repo_project_id="$("$TK_BIN" project create -prefix REP -title "Repo Harness" -git-repository "$repo_remote" -printid)"
	export TICKET_PROJECT="SRV"

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

	export TICKET_PROJECT="OPS"
	run "$TK_BIN" ls -count -expect_equals 0
	run "$TK_BIN" add "Ops project ticket" >/dev/null
	run "$TK_BIN" ls -count -expect_equals 1
	export TICKET_PROJECT="SRV"
	run "$TK_BIN" ls -count -expect_equals 1
	run "$TK_BIN" count -project_id "$repo_project_id" -expect_equals 0
	unset TICKET_PROJECT
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
	export TICKET_PROJECT="private"
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
}

run_todo_example() {
	local seed_script repo_dir ticket_home_dir shared_server_url shared_server_password
	local server_home server_port server_addr server_url manifest_file status_output
	local list_output labels_output deps_output time_output history_json intervene_json
	local count_before_intervene count_after_intervene

	require_tk_bin
	unset AGENT_ID AGENT_PASSWORD

	WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ticket-todo-example.XXXXXX")"
	seed_script="$ROOT_DIR/scripts/populate_todo_example.sh"
	ticket_home_dir="$WORK_DIR/home"
	repo_dir="$WORK_DIR/repo"
	shared_server_url="${TICKET_TEST_SERVER_URL:-}"
	shared_server_password="${TICKET_TEST_SERVER_PASSWORD:-adminpass}"

	mkdir -p "$repo_dir/.git" "$ticket_home_dir"
	cd "$repo_dir"
	export TICKET_HOME="$ticket_home_dir"

	if [[ -n "$shared_server_url" ]]; then
		server_url="$shared_server_url"
	else
		server_home="$WORK_DIR/server-home"
		server_port=$((20000 + RANDOM % 20000))
		server_addr="127.0.0.1:$server_port"
		server_url="http://$server_addr"

		mkdir -p "$server_home"
		env TICKET_HOME="$server_home" "$TK_BIN" initdb -password "$shared_server_password" >/dev/null
		env TICKET_HOME="$server_home" "$TK_BIN" server -f "$server_home/ticket.db" -addr "$server_addr" >/dev/null 2>&1 &
		SERVER_PID=$!
		wait_for_http "$server_url/api/healthz" "server startup"
	fi

	export TICKET_URL="$server_url"
	export TICKET_USERNAME="admin"
	unset TICKET_TOKEN
	export TICKET_PASSWORD="$shared_server_password"
	use_token_auth
	"$TK_BIN" whoami >/dev/null

	"$seed_script" >/dev/null

	manifest_file="$TICKET_HOME/demo-example.env"
	if [[ ! -f "$manifest_file" ]]; then
		echo "manifest file not found: $manifest_file" >&2
		exit 1
	fi
	# shellcheck disable=SC1090
	source "$manifest_file"

	assert_contains() {
		local haystack="$1"
		local needle="$2"
		local label="$3"
		if [[ "$haystack" != *"$needle"* ]]; then
			echo "$label: expected to find '$needle'" >&2
			echo "$haystack" >&2
			exit 1
		fi
	}

	export TICKET_PROJECT="DEMO"
	status_output="$("$TK_BIN" status)"
	assert_contains "$status_output" "TICKET_URL" "status url"

	list_output="$("$TK_BIN" ls)"
	assert_contains "$list_output" "$EPIC_ID" "ticket list epic"
	assert_contains "$list_output" "$TASK_API_ID" "ticket list api task"
	assert_contains "$list_output" "$BUG_ID" "ticket list bug"

	labels_output="$("$TK_BIN" label ls)"
	assert_contains "$labels_output" "frontend" "labels list frontend"
	assert_contains "$labels_output" "backend" "labels list backend"

	deps_output="$("$TK_BIN" get -id "$TASK_WEB_ID" -v)"
	assert_contains "$deps_output" "$TASK_API_ID" "web task dependency"

	time_output="$("$TK_BIN" time total -id "$TASK_API_ID")"
	assert_contains "$time_output" "45" "api task time total"

	count_before_intervene="$("$TK_BIN" ls -count | tr -cd '0-9')"
	"$TK_BIN" fail -id "$BUG_ID" >/dev/null
	intervene_json="$("$TK_BIN" -json intervene -id "$BUG_ID" -outcome split-work -m "Split into follow-up for investigation")"
	if [[ "$intervene_json" != *'"follow_up": {'* ]]; then
		echo "intervene split-work did not return a follow_up ticket" >&2
		echo "$intervene_json" >&2
		exit 1
	fi

	count_after_intervene="$("$TK_BIN" ls -count | tr -cd '0-9')"
	if [[ -z "$count_before_intervene" || -z "$count_after_intervene" ]]; then
		echo "could not parse ticket counts around intervention scenario" >&2
		echo "before=$count_before_intervene after=$count_after_intervene" >&2
		exit 1
	fi
	if (( count_after_intervene <= count_before_intervene )); then
		echo "intervention split-work did not increase open ticket count" >&2
		echo "before=$count_before_intervene after=$count_after_intervene" >&2
		echo "$intervene_json" >&2
		exit 1
	fi

	history_json="$("$TK_BIN" -json history "$BUG_ID")"
	assert_contains "$history_json" "ticket_intervention_decided" "intervention history event"
	assert_contains "$history_json" "split-work" "intervention decision payload"

	echo "todo example verification passed"
}

run_final() {
	local server_home server_port server_addr server_url

	require_tk_bin
	WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ticket-final-shell.XXXXXX")"
	server_home="$WORK_DIR/server-home"
	server_port=$((20000 + RANDOM % 20000))
	server_addr="127.0.0.1:$server_port"
	server_url="http://$server_addr"

	mkdir -p "$server_home"
	env TICKET_HOME="$server_home" "$TK_BIN" initdb -password adminpass >/dev/null
	env TICKET_HOME="$server_home" "$TK_BIN" server -f "$server_home/ticket.db" -addr "$server_addr" >/dev/null 2>&1 &
	SERVER_PID=$!
	wait_for_http "$server_url/api/healthz" "shared server startup"

	env \
		TK_BIN="$TK_BIN" \
		TICKET_FAST_HASH="$TICKET_FAST_HASH" \
		TICKET_TEST_SERVER_URL="$server_url" \
		TICKET_TEST_SERVER_PASSWORD="adminpass" \
		"$ROOT_DIR/scripts/test_shell.sh" harness

	env \
		TK_BIN="$TK_BIN" \
		TICKET_FAST_HASH="$TICKET_FAST_HASH" \
		TICKET_TEST_SERVER_URL="$server_url" \
		TICKET_TEST_SERVER_PASSWORD="adminpass" \
		"$ROOT_DIR/scripts/test_shell.sh" todo-example
}

main() {
	local command="${1:-}"
	case "$command" in
		harness)
			run_harness
			;;
		todo-example)
			run_todo_example
			;;
		final)
			run_final
			;;
		""|-h|--help|help)
			usage
			;;
		*)
			printf 'unknown shell test suite: %s\n\n' "$command" >&2
			usage >&2
			exit 1
			;;
	esac
}

main "$@"
