#!/bin/sh
set -eu

TICKET_DATA_DIR="${TICKET_DATA_DIR:-${TICKET_HOME:-/data}}"
TICKET_DB_PATH="${TICKET_DB_PATH:-$TICKET_DATA_DIR/ticket.db}"
TICKET_SERVER_ADDR="${TICKET_SERVER_ADDR:-0.0.0.0:8080}"
export TICKET_HOME="$TICKET_DATA_DIR"

random_password() {
  LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 24
}

if [ ! -f "$TICKET_DB_PATH" ]; then
  ADMIN_PASSWORD="${TICKET_ADMIN_PASSWORD:-$(random_password)}"
  echo "No database found — initialising $TICKET_DB_PATH"
  mkdir -p "$TICKET_DATA_DIR"
  tk initdb -f "$TICKET_DB_PATH" -password "$ADMIN_PASSWORD"
  echo "ADMIN PASSWORD: $ADMIN_PASSWORD"
  echo "Save this password now. It will not be printed again unless the database is recreated."
  echo ""
fi

exec tk server -f "$TICKET_DB_PATH" -addr "$TICKET_SERVER_ADDR" "$@"
