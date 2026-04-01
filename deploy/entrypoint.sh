#!/bin/sh
set -e

TICKET_HOME="${TICKET_HOME:-/home/ticket/.ticket}"
export TICKET_HOME

if [ ! -f "$TICKET_HOME/ticket.db" ]; then
  echo "No database found — initialising..."
  mkdir -p "$TICKET_HOME"
  ticket initdb
  echo ""
fi

exec ticket server "$@"
