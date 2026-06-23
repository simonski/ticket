#!/usr/bin/env bash
#
# Coverage gate (TK-116).
#
# Each package is measured INCLUDING the test suites that exercise it, not just
# its own *_test.go files. The libticket HTTP contract suite drives
# internal/server and internal/client over real HTTP, and the cmd/tk CLI tests
# drive internal/store through the local service — none of which the old
# per-package `go test <pkg> -cover` counted. Measuring with -coverpkg across the
# driving suites gives a truer figure (e.g. internal/client 57%->67%,
# internal/store 67%->72%). See docs/TESTING.md.
#
# Usage: scripts/coverage.sh
set -uo pipefail
export TICKET_FAST_HASH=1

fail=0

# check <target-pkg> <min%> <driver-test-pkg...>
check() {
  target="$1"; min="$2"; shift 2
  prof="$(mktemp)"
  if ! go test -coverpkg="${target}/..." -coverprofile="$prof" "$@" >/dev/null 2>&1; then
    printf "  FAIL %-22s tests did not pass\n" "$target"
    rm -f "$prof"; fail=1; return
  fi
  pct="$(go tool cover -func="$prof" | tail -1 | grep -oE '[0-9]+\.[0-9]+' | head -1)"
  rm -f "$prof"
  if awk "BEGIN{exit !(${pct:-0} >= ${min})}"; then
    printf "  ok   %-22s %6s%% (>= %s%%)\n" "$target" "$pct" "$min"
  else
    printf "  FAIL %-22s %6s%% (need %s%%)\n" "$target" "${pct:-0}" "$min"
    fail=1
  fi
}

echo "coverage gate (integration-aware):"
check ./cmd/tk          58 ./cmd/tk/...
check ./libticket       67 ./libticket/...
check ./internal/client 65 ./internal/client/... ./libticket/...
check ./internal/store  70 ./internal/store/... ./libticket/... ./internal/server/... ./cmd/tk/...
check ./internal/server 60 ./internal/server/... ./libticket/...
check ./internal/config 80 ./internal/config/...

if [ "$fail" -ne 0 ]; then
  echo "coverage gate: FAIL"
  exit 1
fi
echo "coverage gate: PASS"
