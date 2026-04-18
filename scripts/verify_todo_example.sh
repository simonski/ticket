#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SEED_SCRIPT="$ROOT_DIR/scripts/populate_todo_example.sh"
TK_BIN="${TK_BIN:-$ROOT_DIR/bin/tk}"

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
assert_contains "$status_output" "current project  : demo (DEMO)" "status project"

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

echo "todo example verification passed"
