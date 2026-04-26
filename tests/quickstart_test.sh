#!/usr/bin/env bash
#
# Quickstart verification script.
#
# Executes the markdown quickstarts through cmd/tk-test so the checked commands
# always stay aligned with the published docs.
#
# Usage:
#   go build -o ./bin/tk ./cmd/tk && ./tests/quickstart_test.sh
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TICKET_BIN="$ROOT_DIR/bin/tk"

if [[ ! -x "$TICKET_BIN" ]]; then
  echo "FAIL: tk binary not found at $TICKET_BIN (run 'go build -o ./bin/tk ./cmd/tk' first)" >&2
  exit 1
fi

cd "$ROOT_DIR"
go run ./cmd/tk-test -ticket "$TICKET_BIN" QUICKSTART_CLIENT.md QUICKSTART_SERVER.md
