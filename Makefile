.PHONY: help default build setup setup-go setup-node setup-playwright bump-version test test-go test-go-cover test-unit test-integration test-playwright clean release release-build release-checksums release-formula release-publish release-clean docker-build docker-up docker-down

VERSION_FILE  := cmd/ticket/VERSION
VERSION       := $(shell cat $(VERSION_FILE) 2>/dev/null | tr -d '[:space:]')
GITHUB_REPO   := simonski/ticket
DIST_DIR      := dist
HOMEBREW_TAP  := ../homebrew-tap  # local checkout of simonski/homebrew-tap (optional)

default: help

help:
	@printf "Available targets:\n\n"
	@printf "  make build           Build ticket into ./bin/ticket and symlink ./tk.\n"
	@printf "                       Also increments the patch version in ./VERSION.\n"
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
	@printf "Docker targets:\n\n"
	@printf "  make docker-build    Build the Docker image.\n"
	@printf "  make docker-up       Start the service via Docker Compose.\n"
	@printf "  make docker-down     Stop the service via Docker Compose.\n"
	@printf "\n"
	@printf "Release targets:\n\n"
	@printf "  make release         Full release: build → checksums → formula → instructions.\n"
	@printf "  make release-build   Cross-compile binaries and pack tarballs into ./dist.\n"
	@printf "  make release-checksums  Write SHA256 checksums for all dist tarballs.\n"
	@printf "  make release-formula Generate homebrew/ticket.rb from the formula template.\n"
	@printf "  make release-publish Upload tarballs to a GitHub release via gh CLI.\n"
	@printf "  make release-clean   Remove the ./dist directory.\n"
	@printf "\n"

build: 
	@$(MAKE) bump-version
	@mkdir -p bin
	go build -o ./bin/ticket ./cmd/ticket

setup: setup-go setup-node setup-playwright

setup-go:
	go mod download
	go install golang.org/x/vuln/cmd/govulncheck@latest

setup-node:
	npm install

setup-playwright:
	npx playwright install chromium

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

UNIT_TEST_PKGS := ./internal/config ./internal/password ./web
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
		"./internal/config 70"; do \
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

# ─── release ──────────────────────────────────────────────────────────────────
# Produces cross-platform tarballs in ./dist and updates homebrew/ticket.rb.
# Prerequisites: go, gh (GitHub CLI), shasum.
#
# Workflow:
#   make release          → build + checksums + formula
#   make release-publish  → gh release create + upload tarballs
#   (copy homebrew/ticket.rb → simonski/homebrew-tap/Formula/ticket.rb and push)

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
		name=ticket_$(VERSION)_$${os}_$${arch}; \
		outdir=$(DIST_DIR)/$$name; \
		mkdir -p $$outdir; \
		printf "  %-32s" "$$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -o $$outdir/ticket ./cmd/ticket && echo "ok" || exit 1; \
		tar -czf $(DIST_DIR)/$${name}.tar.gz -C $$outdir ticket; \
		rm -rf $$outdir; \
	done
	@echo "Tarballs written to $(DIST_DIR)/"

release-checksums:
	@echo "Computing SHA256 checksums..."
	@cd $(DIST_DIR) && \
		for f in *.tar.gz; do \
			shasum -a 256 "$$f"; \
		done | tee checksums.txt
	@echo "Checksums written to $(DIST_DIR)/checksums.txt"

release-formula:
	@echo "Generating homebrew/ticket.rb for v$(VERSION)..."
	@darwin_arm64=$$(shasum -a 256 $(DIST_DIR)/ticket_$(VERSION)_darwin_arm64.tar.gz | cut -d' ' -f1); \
	 darwin_amd64=$$(shasum -a 256 $(DIST_DIR)/ticket_$(VERSION)_darwin_amd64.tar.gz | cut -d' ' -f1); \
	 linux_amd64=$$(shasum -a 256  $(DIST_DIR)/ticket_$(VERSION)_linux_amd64.tar.gz  | cut -d' ' -f1); \
	 linux_arm64=$$(shasum -a 256  $(DIST_DIR)/ticket_$(VERSION)_linux_arm64.tar.gz  | cut -d' ' -f1); \
	 sed \
		-e "s/__VERSION__/$(VERSION)/g" \
		-e "s/__DARWIN_ARM64_SHA256__/$$darwin_arm64/g" \
		-e "s/__DARWIN_AMD64_SHA256__/$$darwin_amd64/g" \
		-e "s/__LINUX_AMD64_SHA256__/$$linux_amd64/g" \
		-e "s/__LINUX_ARM64_SHA256__/$$linux_arm64/g" \
		homebrew/ticket.rb.tmpl > homebrew/ticket.rb
	@echo "Formula written to homebrew/ticket.rb"

release-publish:
	@echo "Creating GitHub release v$(VERSION)..."
	@gh release create v$(VERSION) \
		--repo $(GITHUB_REPO) \
		--title "v$(VERSION)" \
		--generate-notes \
		$(DIST_DIR)/ticket_$(VERSION)_darwin_arm64.tar.gz \
		$(DIST_DIR)/ticket_$(VERSION)_darwin_amd64.tar.gz \
		$(DIST_DIR)/ticket_$(VERSION)_linux_amd64.tar.gz \
		$(DIST_DIR)/ticket_$(VERSION)_linux_arm64.tar.gz \
		$(DIST_DIR)/checksums.txt
	@echo "Release v$(VERSION) published."

release: release-build release-checksums release-formula
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  Release v$(VERSION) ready"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  Artifacts:   $(DIST_DIR)/"
	@echo "  Formula:     homebrew/ticket.rb"
	@echo "  Tap dir:     $(HOMEBREW_TAP)"
	@echo ""
	@echo "  Step 1 — publish the GitHub release:"
	@echo ""
	@echo "    make release-publish"
	@echo ""
	@echo "  Step 2 — push the formula to the tap repo:"
	@echo ""
	@echo "    cp homebrew/ticket.rb ../homebrew-tap/Formula/ticket.rb"
	@echo "    git -C $(HOMEBREW_TAP) add Formula/ticket.rb"
	@echo "    git -C $(HOMEBREW_TAP) commit -m \"ticket $(VERSION)\""
	@echo "    git -C $(HOMEBREW_TAP) push"
	@echo ""
	@echo "  Users then install with:"
	@echo ""
	@echo "    brew tap simonski/tap"
	@echo "    brew install ticket"
	@echo ""
	@echo "  Or in one line:"
	@echo ""
	@echo "    brew install simonski/tap/ticket"
	@echo ""

# ─── docker ───────────────────────────────────────────────────────────────────

docker-build:
	docker build -t ticket:$(VERSION) -t ticket:latest .

docker-up:
	VERSION=$(VERSION) docker compose up -d

docker-down:
	VERSION=$(VERSION) docker compose down

# ─── clean ────────────────────────────────────────────────────────────────────

clean:
	@rm -rf bin
	@rm -f tk

install: clean build
	go install ./cmd/ticket

dev:
    # prints out the env vars I need to set to go into a ticket dev mode
	@echo ""
	@echo "Run the following:\n"
	@echo "export TICKET_URL=file://`pwd`/ticket.db"
	@echo "export TICKET_CONFIG_DIR=`pwd`"
	@echo "\nAnd you are now in a position to extend ticket itself.\n"
