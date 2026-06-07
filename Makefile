.PHONY: help default build build-dev build-bin build-linux caddy setup setup-go setup-node setup-playwright bump-version sync-openapi-version validate-openapi backup-db test test-fast test-all test-go test-go-race test-go-cover test-unit test-api-smoke test-cli test-contract test-store test-integration test-api test-api-cli test-browser test-browser-smoke test-browser-full test-playwright test-quickstart test-quickstart-bin test-tk-test test-todo-example test-todo-example-bin testscripts testscripts-bin test-final-shell-bin lint vulncheck ci-bootstrap ci-bootstrap-verify ci-bootstrap-browser ci-bootstrap-publish ci ci-verify ci-browser ci-publish clean release release-prepare release-build release-checksums release-formula homebrew release-sbom release-publish release-clean docker docker-push publish docker-up docker-down deploy

VERSION_FILE  := cmd/tk/VERSION
VERSION       := $(shell cat $(VERSION_FILE) 2>/dev/null | tr -d '[:space:]')
GITHUB_REPO   := simonski/ticket
GHCR_IMAGE    := ghcr.io/simonski/ticket
DIST_DIR      := dist
HOMEBREW_TAP_REPO := https://github.com/simonski/homebrew-tap.git
RELEASE_DARWIN_ARM64 := $(DIST_DIR)/tk_$(VERSION)_darwin_arm64.tar.gz
RELEASE_DARWIN_AMD64 := $(DIST_DIR)/tk_$(VERSION)_darwin_amd64.tar.gz
RELEASE_LINUX_AMD64  := $(DIST_DIR)/tk_$(VERSION)_linux_amd64.tar.gz
RELEASE_LINUX_ARM64  := $(DIST_DIR)/tk_$(VERSION)_linux_arm64.tar.gz
RELEASE_TARBALLS := $(RELEASE_DARWIN_ARM64) $(RELEASE_DARWIN_AMD64) $(RELEASE_LINUX_AMD64) $(RELEASE_LINUX_ARM64)

EXE_DEV_URL ?= ticket.exe.xyz

default: help

help:
	@printf "Available targets:\n\n"
	@printf "  make build           Build tk binary into ./bin/tk.\n"
	@printf "                       Also increments the patch version in ./VERSION.\n"
	@printf "  make build-dev       Build tk binary into ./bin/tk without changing the version.\n"
	@printf "  make build-linux     Build a linux/amd64 tk binary into ./bin/tk-linux using BuildKit.\n"
	@printf "  make caddy           Run local HTTPS reverse proxy https://ticket.localhost -> http://localhost:8080.\n"
	@printf "  make sync-openapi-version Sync openapi.yaml version with cmd/tk/VERSION.\n"
	@printf "  make validate-openapi Parse openapi.yaml and require core metadata.\n"
	@printf "  make backup-db       Export and compress a local snapshot under .ticket/backups/.\n"
	@printf "  make setup           Install development dependencies (Go + Node).\n"
	@printf "  make setup-go        Download and cache Go module dependencies.\n"
	@printf "  make setup-node      Install Node dependencies.\n"
	@printf "  make setup-playwright Install Chromium for Playwright.\n"
	@printf "  make test            Run fast default tests (unit).\n"
	@printf "  make test-fast       Run the fast developer loop (unit + API smoke + JS API).\n"
	@printf "  make test-all        Run all tests (unit + api + browser + docs/harness).\n"
	@printf "  make test-go         Run Go tests.\n"
	@printf "  make test-go-race    Run Go tests with the race detector.\n"
	@printf "  make test-unit       Run unit-oriented Go test packages.\n"
	@printf "  make test-api-smoke  Run fast Go API smoke packages (client + server).\n"
	@printf "  make test-cli        Run CLI package tests.\n"
	@printf "  make test-contract   Run libticket contract tests.\n"
	@printf "  make test-store      Run store package tests.\n"
	@printf "  make test-api-cli    Run CLI/API interface tests (cmd + client + server + contract).\n"
	@printf "  make test-api        Alias for test-api-cli.\n"
	@printf "  make test-browser    Run the fast browser smoke suite.\n"
	@printf "  make test-browser-full Run the full browser end-to-end suite.\n"
	@printf "  make test-browser-smoke Alias for make test-browser.\n"
	@printf "  make test-integration Run integration-oriented Go test packages.\n"
	@printf "  make test-go-cover   Run Go tests with package coverage thresholds.\n"
	@printf "  make test-playwright Run browser/frontend smoke checks.\n"
	@printf "  make test-quickstart Run executable QUICKSTART/TUTORIAL docs tests.\n"
	@printf "  make test-tk-test    Alias for make test-quickstart.\n"
	@printf "  make test-todo-example Validate the seeded todo tutorial scenario.\n"
	@printf "  make testscripts     Run the shell-based CLI harness scenarios.\n"
	@printf "  make lint            Run golangci-lint on all packages.\n"
	@printf "  make vulncheck       Run govulncheck on all Go packages.\n"
	@printf "  make ci-bootstrap    Install dependencies for the full local/CI parity flow.\n"
	@printf "  make ci-verify       Run the same verify sequence as the GitHub verify job.\n"
	@printf "  make ci-browser      Run the same browser sequence as the GitHub browser job.\n"
	@printf "  make ci              Run the same verify + browser flow as GitHub Actions.\n"
	@printf "  make ci-publish      Run the same publish sequence as the GitHub publish job.\n"
	@printf "  make clean           Remove built binaries from ./bin.\n"
	@printf "\n"
	@printf "Docker targets:\n\n"
	@printf "  make docker    Build the local Docker image only.\n"
	@printf "  make publish         Build the image and push versioned + latest tags to GHCR.\n"
	@printf "  make docker-up       Start deploy/compose.yaml using deploy/.env.\n"
	@printf "  make docker-down     Stop deploy/compose.yaml using deploy/.env.\n"
	@printf "\n"
	@printf "exe.dev targets:\n\n"
	@printf "  make deploy          Copy deploy/compose.yaml, deploy/env.template, and deploy/README.md to the configured host.\n"
	@printf "                       Set EXE_DEV_URL=user@host to choose the remote destination.\n"
	@printf "\n"
	@printf "Release targets:\n\n"
	@printf "  make release         Full release: build → checksums → SBOM → formula → GitHub release → homebrew tap.\n"
	@printf "  make release-publish Same as release (all-in-one).\n"
	@printf "  make release-build   Cross-compile binaries and pack tarballs into ./dist.\n"
	@printf "  make release-checksums  Write SHA256 checksums for all dist tarballs.\n"
	@printf "  make release-formula Generate homebrew/ticket.rb from the formula template.\n"
	@printf "  make homebrew        Push the generated formula to simonski/homebrew-tap.\n"
	@printf "  make release-clean   Remove the ./dist directory.\n"
	@printf "\n"

build:
	@$(MAKE) release-prepare
	@$(MAKE) build-bin

build-dev:
	@$(MAKE) build-bin

build-bin:
	@mkdir -p bin
	go build -o ./bin/tk ./cmd/tk

build-linux:
	@mkdir -p ./bin
	DOCKER_BUILDKIT=1 docker buildx build --platform linux/amd64 --target artifact --output type=local,dest=./bin .
	@chmod +x ./bin/tk-linux

caddy:
	caddy run --config ./deploy/Caddyfile


setup: setup-go setup-node setup-playwright

setup-go:
	go mod download
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest

lint:
	golangci-lint run ./...
	gosec ./...

vulncheck:
	govulncheck ./...

setup-node:
	npm ci

setup-playwright:
	@if ! npx playwright install --list 2>/dev/null | grep -q '/chromium-'; then npx playwright install chromium; fi

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

sync-openapi-version:
	@perl -0pi -e 's/^(  version: ).*/$${1}$(VERSION)/m' docs/api/openapi.yaml

release-prepare:
	@$(MAKE) bump-version
	@$(MAKE) sync-openapi-version

validate-openapi:
	@ruby -e 'require "yaml"; doc = YAML.safe_load(File.read("docs/api/openapi.yaml"), permitted_classes: [], aliases: true); abort("docs/api/openapi.yaml missing openapi version") unless doc.is_a?(Hash) && !doc["openapi"].to_s.empty?; info = doc["info"].is_a?(Hash) ? doc["info"] : {}; abort("docs/api/openapi.yaml missing info.title") if info["title"].to_s.empty?; abort("docs/api/openapi.yaml missing info.version") if info["version"].to_s.empty?'

backup-db:
	./scripts/backup_ticket_db.sh

UNIT_TEST_PKGS := ./internal/config ./internal/password ./web
API_SMOKE_TEST_PKGS := ./internal/client ./internal/server
CLI_TEST_PKGS := ./cmd/tk
CONTRACT_TEST_PKGS := ./libticket
STORE_TEST_PKGS := ./internal/store
PLAYWRIGHT_SMOKE_SPECS := tests/playwright/auth.spec.js tests/playwright/home.spec.js tests/playwright/navigation.spec.js tests/playwright/tickets.spec.js

test: test-unit

test-fast: test-unit test-api-smoke

test-all: test-unit test-api-cli test-browser-full build-bin test-quickstart-bin test-final-shell-bin

test-go:
	TICKET_FAST_HASH=1 go test ./...

test-go-race:
	TICKET_FAST_HASH=1 go test -race ./...

test-unit:
	TICKET_FAST_HASH=1 go test $(UNIT_TEST_PKGS)

test-api-smoke:
	TICKET_FAST_HASH=1 go test $(API_SMOKE_TEST_PKGS)

test-cli:
	TICKET_FAST_HASH=1 go test $(CLI_TEST_PKGS)

test-contract:
	TICKET_FAST_HASH=1 go test $(CONTRACT_TEST_PKGS)

test-store:
	TICKET_FAST_HASH=1 go test $(STORE_TEST_PKGS)

test-integration:
	@$(MAKE) test-store
	@$(MAKE) test-api-cli

test-api-cli:
	@$(MAKE) test-api-smoke
	@$(MAKE) test-cli
	@$(MAKE) test-contract

test-api: test-api-cli

ci-bootstrap: ci-bootstrap-verify ci-bootstrap-browser

ci-bootstrap-verify: setup-go setup-node

ci-bootstrap-browser: setup-node setup-playwright

ci-bootstrap-publish: setup-go

ci-verify: validate-openapi test-go-cover build-dev lint vulncheck

ci-browser: test-browser-full

ci: ci-verify ci-browser

ci-publish: docker-push release-publish

test-go-cover:
	@export TICKET_FAST_HASH=1; set -e; \
	for entry in \
		"./cmd/tk 55" \
		"./libticket 65" \
		"./internal/client 55" \
		"./internal/store 69" \
		"./internal/server 63" \
		"./internal/config 70"; do \
		pkg=$${entry% *}; \
		min=$${entry#* }; \
		set +e; \
		out=$$(go test "$$pkg" -cover 2>&1); \
		status=$$?; \
		set -e; \
		printf "%s\n" "$$out"; \
		if [ "$$status" -ne 0 ]; then \
			exit "$$status"; \
		fi; \
		pct=$$(printf "%s\n" "$$out" | sed -n 's/.*coverage: \([0-9][0-9.]*\)%.*/\1/p' | tail -n 1); \
		if [ -z "$$pct" ]; then \
			printf "could not parse coverage for %s\n" "$$pkg" >&2; \
			exit 1; \
		fi; \
		awk -v pct="$$pct" -v min="$$min" 'BEGIN { if (pct + 0 < min + 0) exit 1 }' || { \
			printf "coverage threshold failed for %s: got %s%%, need %s%%\n" "$$pkg" "$$pct" "$$min" >&2; \
			exit 1; \
		}; \
	done

playwright-ready:
	@if [ ! -d node_modules ]; then $(MAKE) setup-node; fi
	@if ! npx playwright install --list 2>/dev/null | grep -q '/chromium-'; then npx playwright install chromium; fi

test-playwright: playwright-ready
	@PLAYWRIGHT_PORT=$$(python3 -c "import socket; s=socket.socket(); s.bind(('127.0.0.1', 0)); print(s.getsockname()[1]); s.close()"); \
	PLAYWRIGHT_PORT=$$PLAYWRIGHT_PORT npx playwright test -c tests/playwright.config.js

test-browser-smoke: playwright-ready
	@PLAYWRIGHT_PORT=$$(python3 -c "import socket; s=socket.socket(); s.bind(('127.0.0.1', 0)); print(s.getsockname()[1]); s.close()"); \
	PLAYWRIGHT_PORT=$$PLAYWRIGHT_PORT npx playwright test -c tests/playwright.config.js $(PLAYWRIGHT_SMOKE_SPECS)

test-browser-full: test-playwright

test-browser: test-browser-smoke

test-quickstart: build-bin
	@$(MAKE) test-quickstart-bin

test-quickstart-bin:
	TICKET_FAST_HASH=1 go run ./cmd/tk-test docs/QUICKSTART.md docs/TUTORIAL.md

test-tk-test: test-quickstart

test-todo-example: build-bin
	@$(MAKE) test-todo-example-bin

test-todo-example-bin:
	TICKET_FAST_HASH=1 ./scripts/test_shell.sh todo-example

testscripts: build-bin
	@$(MAKE) testscripts-bin

testscripts-bin:
	TICKET_FAST_HASH=1 ./scripts/test_shell.sh harness

test-final-shell-bin:
	TICKET_FAST_HASH=1 ./scripts/test_shell.sh final

# ─── release ──────────────────────────────────────────────────────────────────
# Produces cross-platform tarballs in ./dist, creates a GitHub release, and
# pushes the updated Homebrew formula to the simonski/homebrew-tap repo.
# Prerequisites: go, gh (GitHub CLI), shasum, cyclonedx-gomod, git.
#
# Usage:
#   make release          → full end-to-end release (alias for release-publish)
#   make release-publish  → build + checksums + SBOM + formula + GitHub release + tap update

RELEASE_PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

release-clean:
	@rm -rf $(DIST_DIR)

release-build:
	@$(MAKE) release-clean
	@mkdir -p $(DIST_DIR)
	@echo "Building v$(VERSION) for all platforms..."
	@for platform in $(RELEASE_PLATFORMS); do \
		os=$$(echo $$platform | cut -d/ -f1); \
		arch=$$(echo $$platform | cut -d/ -f2); \
		name=tk_$(VERSION)_$${os}_$${arch}; \
		outdir=$(DIST_DIR)/$$name; \
		mkdir -p $$outdir; \
		printf "  %-32s" "$$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -o $$outdir/tk ./cmd/tk && echo "ok" || exit 1; \
		tar -czf $(DIST_DIR)/$${name}.tar.gz -C $$outdir tk; \
		rm -rf $$outdir; \
	done
	@echo "Tarballs written to $(DIST_DIR)/"

release-checksums: release-build
	@echo "Computing SHA256 checksums..."
	@cd $(DIST_DIR) && \
		for f in *.tar.gz; do \
			shasum -a 256 "$$f"; \
		done | tee checksums.txt
	@echo "Checksums written to $(DIST_DIR)/checksums.txt"

release-sbom:
	@echo "Generating CycloneDX SBOM..."
	@cyclonedx-gomod mod -json -output $(DIST_DIR)/sbom.cdx.json .
	@echo "SBOM written to $(DIST_DIR)/sbom.cdx.json"

release-formula: release-checksums
	@echo "Generating homebrew/ticket.rb for v$(VERSION)..."
	@for f in $(RELEASE_TARBALLS); do \
		if [ ! -f "$$f" ]; then \
			echo "Missing release artifact: $$f"; \
			exit 1; \
		fi; \
	done
	@darwin_arm64=$$(awk '/ tk_$(VERSION)_darwin_arm64.tar.gz$$/{print $$1}' $(DIST_DIR)/checksums.txt); \
	 darwin_amd64=$$(awk '/ tk_$(VERSION)_darwin_amd64.tar.gz$$/{print $$1}' $(DIST_DIR)/checksums.txt); \
	 linux_amd64=$$(awk '/ tk_$(VERSION)_linux_amd64.tar.gz$$/{print $$1}' $(DIST_DIR)/checksums.txt); \
	 linux_arm64=$$(awk '/ tk_$(VERSION)_linux_arm64.tar.gz$$/{print $$1}' $(DIST_DIR)/checksums.txt); \
	 if [ -z "$$darwin_arm64" ] || [ -z "$$darwin_amd64" ] || [ -z "$$linux_amd64" ] || [ -z "$$linux_arm64" ]; then \
		echo "Missing release checksums in $(DIST_DIR)/checksums.txt"; \
		exit 1; \
	 fi; \
	 sed \
		-e "s/__VERSION__/$(VERSION)/g" \
		-e "s/__DARWIN_ARM64_SHA256__/$$darwin_arm64/g" \
		-e "s/__DARWIN_AMD64_SHA256__/$$darwin_amd64/g" \
		-e "s/__LINUX_AMD64_SHA256__/$$linux_amd64/g" \
		-e "s/__LINUX_ARM64_SHA256__/$$linux_arm64/g" \
		 homebrew/ticket.rb.tmpl > homebrew/ticket.rb
	@echo "Formula written to homebrew/ticket.rb"

homebrew: release-formula
	@echo "Updating homebrew tap..."
	@TAP_DIR=$$(mktemp -d) && \
		trap "rm -rf $$TAP_DIR" EXIT && \
		git clone $(HOMEBREW_TAP_REPO) "$$TAP_DIR" && \
		cp homebrew/ticket.rb "$$TAP_DIR/Formula/ticket.rb" && \
		if [ -z "$$(git -C "$$TAP_DIR" status --porcelain -- Formula/ticket.rb)" ]; then \
			echo "Homebrew tap already up to date."; \
			exit 0; \
		fi && \
		git -C "$$TAP_DIR" add Formula/ticket.rb && \
		git -C "$$TAP_DIR" commit -m "ticket $(VERSION)" && \
		git -C "$$TAP_DIR" push
	@echo "Homebrew tap updated."

release-publish: release-build release-checksums release-sbom
	@if gh release view v$(VERSION) --repo $(GITHUB_REPO) >/dev/null 2>&1; then \
		echo "Release v$(VERSION) already exists; aborting."; \
		exit 1; \
	fi
	@echo "Creating GitHub release v$(VERSION)..."
	@gh release create v$(VERSION) \
		--repo $(GITHUB_REPO) \
		--title "v$(VERSION)" \
		--generate-notes \
		$(DIST_DIR)/tk_$(VERSION)_darwin_arm64.tar.gz \
		$(DIST_DIR)/tk_$(VERSION)_darwin_amd64.tar.gz \
		$(DIST_DIR)/tk_$(VERSION)_linux_amd64.tar.gz \
		$(DIST_DIR)/tk_$(VERSION)_linux_arm64.tar.gz \
		$(DIST_DIR)/checksums.txt \
		$(DIST_DIR)/sbom.cdx.json
	@echo "Release v$(VERSION) published."
	@$(MAKE) homebrew
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  v$(VERSION) released"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  Install with:"
	@echo "    brew install simonski/tap/ticket"
	@echo ""

release: release-publish

# ─── docker ───────────────────────────────────────────────────────────────────

docker:
	DOCKER_BUILDKIT=1 docker build -t ticket:$(VERSION) -t ticket:latest .

docker-push: docker
	docker tag ticket:$(VERSION) $(GHCR_IMAGE):$(VERSION)
	docker tag ticket:latest $(GHCR_IMAGE):latest
	docker push $(GHCR_IMAGE):$(VERSION)
	docker push $(GHCR_IMAGE):latest

publish: docker-push

docker-up:
	docker compose --env-file deploy/.env -f deploy/compose.yaml up -d

docker-down:
	docker compose --env-file deploy/.env -f deploy/compose.yaml down

# ─── clean ────────────────────────────────────────────────────────────────────

clean:
	@rm -rf bin

install: build
	cp ./bin/tk $$(go env GOPATH)/bin/tk

dev:
	# prints out the commands needed to put this repo into local ticket dev mode
	@echo ""
	@echo "Run the following:\n"
	@echo "tk initdb ."
	@echo "tk project init -prefix TKT -title ticket"
	@echo "\nAnd you are now in a position to extend ticket itself.\n"


deploy: build-linux
	@echo "Deploying assets to exe.dev..."
	@if [ -z "$(EXE_DEV_URL)" ]; then \
		echo "Error: EXE_DEV_URL environment variable not set"; \
		echo "Usage: EXE_DEV_URL=user@host make deploy"; \
		exit 1; \
	fi
	@echo "Copying deploy bundle and tk binary to $(EXE_DEV_URL):~/"
	@scp deploy/compose.yaml deploy/env.template deploy/README.md deploy/Makefile $(EXE_DEV_URL):~/
	@scp bin/tk-linux $(EXE_DEV_URL):~/tk
	@echo ""
	@echo "✓ Deployed to $(EXE_DEV_URL):~/"
	@echo ""
	@echo "Next steps on remote server:"
	@echo "  ssh $(EXE_DEV_URL)"
	@echo "  make setup   # creates .env, pre-owns ./data"
	@echo "  \$$EDITOR .env  # set TICKET_UID, TICKET_GID, TICKET_ADMIN_PASSWORD"
	@echo "  make up"
	@echo ""
	@echo "This bundle uses ghcr.io/simonski/ticket:latest and a 30-second watchtower poll."
