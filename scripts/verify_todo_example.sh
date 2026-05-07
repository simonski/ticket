#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SEED_SCRIPT="$ROOT_DIR/scripts/populate_todo_example.sh"
TK_BIN="${TK_BIN:-$ROOT_DIR/bin/tk}"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ticket-todo-example.XXXXXX")"
TICKET_HOME_DIR="$WORK_DIR/home"
REPO_DIR="$WORK_DIR/repo"

cleanup() {
	rm -rf "$WORK_DIR"
}
trap cleanup EXIT

mkdir -p "$REPO_DIR/.git" "$TICKET_HOME_DIR"
cd "$REPO_DIR"
export TICKET_HOME="$TICKET_HOME_DIR"
unset AGENT_ID AGENT_PASSWORD

"$SEED_SCRIPT" >/dev/null

status_json="$("$TK_BIN" -json status)"
config_file="$(printf '%s\n' "$status_json" | sed -nE 's/.*"config_file": "([^"]+)".*/\1/p')"
if [[ -z "$config_file" ]]; then
	echo "could not resolve active config file from 'tk status'" >&2
	exit 1
fi
manifest_file="$(dirname "$config_file")/demo-example.env"
if [[ ! -f "$manifest_file" ]]; then
	echo "manifest file not found: $manifest_file" >&2
	exit 1
fi

# shellcheck disable=SC1090
source "$manifest_file"

"$TK_BIN" project use DEMO >/dev/null

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

status_output="$("$TK_BIN" status)"
assert_contains "$status_output" "project          : DEMO — demo" "status project"

list_output="$("$TK_BIN" ls)"
assert_contains "$list_output" "$EPIC_ID" "ticket list epic"
assert_contains "$list_output" "$TASK_API_ID" "ticket list api task"
assert_contains "$list_output" "$BUG_ID" "ticket list bug"

labels_output="$("$TK_BIN" label ls)"
assert_contains "$labels_output" "frontend" "labels list frontend"
assert_contains "$labels_output" "backend" "labels list backend"

deps_output="$("$TK_BIN" get -id "$TASK_WEB_ID")"
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
