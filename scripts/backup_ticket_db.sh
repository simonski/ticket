#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TK_BIN="${TK_BIN:-$ROOT_DIR/bin/tk}"
TICKET_HOME_DIR="${TICKET_HOME:-$ROOT_DIR/.ticket}"
BACKUP_DIR="${BACKUP_DIR:-$TICKET_HOME_DIR/backups}"
KEEP_DAYS="${KEEP_DAYS:-30}"

if [[ ! -x "$TK_BIN" ]]; then
	echo "tk binary not found at $TK_BIN" >&2
	echo "Build it first: make build-dev" >&2
	exit 1
fi

if ! [[ "$KEEP_DAYS" =~ ^[0-9]+$ ]]; then
	echo "KEEP_DAYS must be an integer, got: $KEEP_DAYS" >&2
	exit 1
fi

mkdir -p "$BACKUP_DIR"

timestamp="$(date +%Y%m%d-%H%M%S)"
snapshot_file="$BACKUP_DIR/ticket-$timestamp.json"

"$TK_BIN" export -o "$snapshot_file"
gzip -f "$snapshot_file"

find "$BACKUP_DIR" -name "ticket-*.json.gz" -mtime +"$KEEP_DAYS" -delete

echo "backup written: $snapshot_file.gz"
