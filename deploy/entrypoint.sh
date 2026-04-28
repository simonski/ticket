#!/bin/sh
set -eu

TICKET_DATA_DIR="${TICKET_DATA_DIR:-${TICKET_HOME:-/data}}"
TICKET_DB_PATH="${TICKET_DB_PATH:-$TICKET_DATA_DIR/ticket.db}"
TICKET_SERVER_ADDR="${TICKET_SERVER_ADDR:-0.0.0.0:8080}"
export TICKET_HOME="$TICKET_DATA_DIR"

mkdir -p "$TICKET_DATA_DIR"
chown -R ticket:ticket "$TICKET_DATA_DIR"

if [ ! -f "$TICKET_DB_PATH" ]; then
  if [ -z "${TICKET_ADMIN_PASSWORD:-}" ]; then
    echo "TICKET_ADMIN_PASSWORD must be set before first boot" >&2
    exit 1
  fi
  echo "No database found - initialising $TICKET_DB_PATH"
  tk initdb -f "$TICKET_DB_PATH" -password "$TICKET_ADMIN_PASSWORD"
fi

exec tk server -f "$TICKET_DB_PATH" -addr "$TICKET_SERVER_ADDR" "$@"
