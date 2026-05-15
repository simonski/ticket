#!/usr/bin/env bash

use_token_auth() {
	if [[ -n "${TICKET_TOKEN:-}" ]]; then
		unset TICKET_PASSWORD
		return 0
	fi
	if [[ -z "${TICKET_URL:-}" || -z "${TICKET_USERNAME:-}" || -z "${TICKET_PASSWORD:-}" ]]; then
		echo "token auth requires TICKET_URL, TICKET_USERNAME, and TICKET_PASSWORD when TICKET_TOKEN is unset" >&2
		return 1
	fi
	local login_json token
	login_json="$("$TK_BIN" -json login -username "$TICKET_USERNAME" -password "$TICKET_PASSWORD")"
	token="$(printf '%s\n' "$login_json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("token", ""))')"
	if [[ -z "$token" ]]; then
		echo "login did not return a session token" >&2
		echo "$login_json" >&2
		return 1
	fi
	export TICKET_TOKEN="$token"
	unset TICKET_PASSWORD
}
