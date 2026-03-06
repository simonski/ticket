.PHONY: help default build setup setup-go setup-node setup-playwright tools bump-version test test-go test-go-cover test-unit test-integration test-playwright clean

VERSION_FILE := cmd/ticket/VERSION

default: help

help:
	@printf "Available targets:\n\n"
	@printf "  make build           Build ticket into ./bin/ticket and symlink ./tk.\n"
	@printf "                       Also increments the patch version in ./VERSION.\n"
	@printf "  make tools           Build helper binaries in the repo root.\n"
	@printf "  make setup           Install development dependencies (Go + Node).\n"
	@printf "  make setup-go        Download and cache Go module dependencies.\n"
	@printf "  make setup-node      Install Node dependencies.\n"
	@printf "  make setup-playwright Install Chromium for Playwright.\n"
	@printf "  make test            Run all tests.\n"
	@printf "  make test-go         Run Go tests.\n"
	@printf "  make test-unit       Run unit-oriented Go test packages.\n"
	@printf "  make test-integration Run integration-oriented Go test packages.\n"
	@printf "  make test-go-cover   Run Go tests with package coverage thresholds.\n"
	@printf "  make test-playwright Run browser/frontend smoke checks.\n"
	@printf "  make clean           Remove built binaries from ./bin.\n"
	@printf "\n"

build:
	@$(MAKE) bump-version
	@mkdir -p bin
	go build -o ./bin/ticket ./cmd/ticket
	@ln -sf ./bin/ticket ./tk

setup: setup-go setup-node setup-playwright

setup-go:
	go mod download

setup-node:
	npm install

setup-playwright:
	npx playwright install chromium

tools:
	@mkdir -p bin
	go build -o bin/parser ./tools/parser
	go build -o bin/wiggum ./tools/wiggum

bump-version:
	@if [ ! -f "$(VERSION_FILE)" ]; then \
		printf "0.1.0\n" > "$(VERSION_FILE)"; \
	else \
		version=$$(tr -d '[:space:]' < "$(VERSION_FILE)"); \
		major=$${version%%.*}; \
		rest=$${version#*.}; \
		minor=$${rest%%.*}; \
		patch=$${rest##*.}; \
		patch=$$((patch + 1)); \
		printf "%s.%s.%s\n" "$$major" "$$minor" "$$patch" > "$(VERSION_FILE)"; \
	fi

UNIT_TEST_PKGS := ./internal/config ./internal/password ./tools/parser ./tools/wiggum ./web
INTEGRATION_TEST_PKGS := ./cmd/ticket ./internal/client ./internal/server ./internal/store ./libticket ./libtickethttp

test: test-unit test-integration test-playwright

test-go:
	go test ./...

test-unit:
	go test $(UNIT_TEST_PKGS)

test-integration:
	go test $(INTEGRATION_TEST_PKGS)

test-go-cover:
	@set -e; \
	for entry in \
		"./cmd/ticket 55" \
		"./libticket 65" \
		"./libtickethttp 75" \
		"./internal/client 55" \
		"./internal/store 70" \
		"./internal/config 70" \
		"./tools/parser 75"; do \
		pkg=$${entry% *}; \
		min=$${entry#* }; \
		out=$$(go test "$$pkg" -cover | tail -n 1); \
		printf "%s\n" "$$out"; \
		pct=$$(printf "%s" "$$out" | sed -n 's/.*coverage: \([0-9.]*\)%.*/\1/p'); \
		if [ -z "$$pct" ]; then \
			printf "could not parse coverage for %s\n" "$$pkg" >&2; \
			exit 1; \
		fi; \
		awk -v pct="$$pct" -v min="$$min" 'BEGIN { if (pct + 0 < min + 0) exit 1 }' || { \
			printf "coverage threshold failed for %s: got %s%%, need %s%%\n" "$$pkg" "$$pct" "$$min" >&2; \
			exit 1; \
		}; \
	done

test-playwright:
	npm install
	npx playwright install chromium
	npx playwright test

clean:
	@rm -rf bin
	@rm -f tk
	@rm -f parser

dev:
    # prints out the env vars I need to set to go into a ticket dev mode
	@echo ""
	@echo "Run the following:\n"
	@echo "export TICKET_MODE=local"
	@echo "export TICKET_HOME=`pwd`"
	@echo "\nAnd you are now in a position to extend ticket itself.\n"
