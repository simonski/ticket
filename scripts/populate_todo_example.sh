#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TK_BIN="${TK_BIN:-$ROOT_DIR/bin/tk}"

if [[ ! -x "$TK_BIN" ]]; then
	echo "tk binary not found at $TK_BIN" >&2
	echo "Build it first: make build-dev" >&2
	exit 1
fi

unset AGENT_ID AGENT_PASSWORD

if [[ -z "${TICKET_URL:-}" || -z "${TICKET_USERNAME:-}" || -z "${TICKET_PASSWORD:-}" ]]; then
	echo "server auth environment variables are required. Set:" >&2
	echo "  TICKET_URL" >&2
	echo "  TICKET_USERNAME" >&2
	echo "  TICKET_PASSWORD" >&2
	exit 1
fi

if ! "$TK_BIN" project use DEMO >/dev/null 2>&1; then
	project_id="$("$TK_BIN" project create -prefix DEMO -title "demo" -description "Example todo application planning workspace" -git-repository https://github.com/example/todo-app -printid)"
	"$TK_BIN" project use "$project_id" >/dev/null
else
	project_id="DEMO"
fi

status_json="$("$TK_BIN" -json status)"
config_file="$(printf '%s\n' "$status_json" | sed -nE 's/.*"config_file": "([^"]+)".*/\1/p')"
db_path="$(printf '%s\n' "$status_json" | sed -nE 's/.*"db_path": "([^"]+)".*/\1/p')"
if [[ -z "$config_file" ]]; then
	echo "could not resolve active config file from 'tk status'" >&2
	exit 1
fi
config_dir="$(dirname "$config_file")"
manifest_file="$config_dir/demo-example.env"

extract_numeric_id() {
	local text="$1"
	local id
	id="$(printf '%s\n' "$text" | sed -nE 's/.*\(id ([0-9]+)\).*/\1/p; s/.*#([0-9]+).*/\1/p' | head -n1)"
	if [[ -z "$id" ]]; then
		echo "could not parse numeric id from: $text" >&2
		exit 1
	fi
	printf '%s' "$id"
}

ensure_label() {
	local name="$1"
	local color="$2"
	local existing_id
	existing_id="$("$TK_BIN" label ls | awk -v n="$name" '$2 == n {print $1; exit}')"
	if [[ -n "$existing_id" ]]; then
		printf '%s' "$existing_id"
		return
	fi
	"$TK_BIN" label create -name "$name" -color "$color" -printid
}

workflow_id="$("$TK_BIN" workflow create -name "demo-flow-$(date +%s)" -d "Reference workflow for a todo app release" -printid)"
design_stage_id="$(extract_numeric_id "$("$TK_BIN" workflow stage-add -id "$workflow_id" -name design -order 0)")"
develop_stage_id="$(extract_numeric_id "$("$TK_BIN" workflow stage-add -id "$workflow_id" -name develop -order 1)")"
test_stage_id="$(extract_numeric_id "$("$TK_BIN" workflow stage-add -id "$workflow_id" -name test -order 2)")"
done_stage_id="$(extract_numeric_id "$("$TK_BIN" workflow stage-add -id "$workflow_id" -name done -order 3)")"

product_role_id="$(extract_numeric_id "$("$TK_BIN" workflow role-add -workflow_id "$workflow_id" -title "Product")")"
engineer_role_id="$(extract_numeric_id "$("$TK_BIN" workflow role-add -workflow_id "$workflow_id" -title "Engineer")")"
qa_role_id="$(extract_numeric_id "$("$TK_BIN" workflow role-add -workflow_id "$workflow_id" -title "QA")")"

"$TK_BIN" workflow stage-role-add -workflow_id "$workflow_id" -stage_id "$design_stage_id" -role_id "$product_role_id" >/dev/null
"$TK_BIN" workflow stage-role-add -workflow_id "$workflow_id" -stage_id "$develop_stage_id" -role_id "$engineer_role_id" >/dev/null
"$TK_BIN" workflow stage-role-add -workflow_id "$workflow_id" -stage_id "$test_stage_id" -role_id "$qa_role_id" >/dev/null
"$TK_BIN" workflow stage-role-add -workflow_id "$workflow_id" -stage_id "$done_stage_id" -role_id "$product_role_id" >/dev/null
"$TK_BIN" project workflow "$workflow_id" >/dev/null

epic_id="$("$TK_BIN" add -type epic -title "Todo app MVP" -printid)"
task_model_id="$("$TK_BIN" add -title "Design task entity and persistence model" -parent "$epic_id" -printid)"
task_api_id="$("$TK_BIN" add -title "Implement task CRUD API endpoints" -parent "$epic_id" -printid)"
task_web_id="$("$TK_BIN" add -title "Implement web board for task management" -parent "$epic_id" -printid)"
task_ci_id="$("$TK_BIN" add -title "Add CLI regression checks for todo workflows" -parent "$epic_id" -printid)"
bug_id="$("$TK_BIN" add -type bug -title "Fix duplicate task ordering edge case" -parent "$epic_id" -printid)"
story_id="$("$TK_BIN" story create -title "As a user, I can add and complete todo items" -printid)"
decision_id="$("$TK_BIN" decision new -printid "Use stage/state lifecycle for todo delivery milestones")"
idea_id="$("$TK_BIN" idea new -printid "Support offline queueing for mobile todo updates")"

frontend_label_id="$(ensure_label frontend '#3b82f6')"
backend_label_id="$(ensure_label backend '#16a34a')"
ops_label_id="$(ensure_label ops '#f59e0b')"

"$TK_BIN" label add -id "$task_web_id" "$frontend_label_id"
"$TK_BIN" label add -id "$task_api_id" "$backend_label_id"
"$TK_BIN" label add -id "$task_ci_id" "$ops_label_id"

"$TK_BIN" dep add -id "$task_api_id" "$task_model_id"
"$TK_BIN" dep add -id "$task_web_id" "$task_api_id"
"$TK_BIN" dep add -id "$task_ci_id" "$task_web_id"

"$TK_BIN" time log -id "$task_api_id" -m 45 -note "API draft implementation"
"$TK_BIN" time log -id "$task_web_id" -m 30 -note "Board interaction scaffolding"
"$TK_BIN" comment add -id "$epic_id" "Example seeded by scripts/populate_todo_example.sh"

"$TK_BIN" update -id "$task_model_id" -status design/success >/dev/null
"$TK_BIN" update -id "$task_api_id" -status develop/active >/dev/null
"$TK_BIN" update -id "$task_web_id" -status design/active >/dev/null
"$TK_BIN" update -id "$bug_id" -status test/active >/dev/null

cat >"$manifest_file" <<EOF
CONFIG_DIR=$config_dir
DB_PATH=$db_path
PROJECT_ID=$project_id
PROJECT_PREFIX=DEMO
Workflow_ID=$workflow_id
EPIC_ID=$epic_id
TASK_MODEL_ID=$task_model_id
TASK_API_ID=$task_api_id
TASK_WEB_ID=$task_web_id
TASK_CI_ID=$task_ci_id
BUG_ID=$bug_id
STORY_ID=$story_id
DECISION_ID=$decision_id
IDEA_ID=$idea_id
EOF

cat <<EOF
Todo example seeded successfully.
Config dir: $config_dir
Database  : $db_path
Project   : demo (DEMO)
Epic      : $epic_id
Manifest  : $manifest_file
EOF
