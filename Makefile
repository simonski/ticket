.PHONY: help default build setup setup-go setup-node setup-playwright bump-version test test-go test-go-cover test-unit test-integration test-playwright test-tk-test lint clean release release-build release-checksums release-formula release-sbom release-publish release-clean docker-build docker-push docker-up docker-down

VERSION_FILE  := cmd/tk/VERSION
VERSION       := $(shell cat $(VERSION_FILE) 2>/dev/null | tr -d '[:space:]')
GITHUB_REPO   := simonski/ticket
GHCR_IMAGE    := ghcr.io/simonski/ticket
DIST_DIR      := dist
HOMEBREW_TAP_REPO := https://github.com/simonski/homebrew-tap.git

default: help

help:
	@printf "Available targets:\n\n"
	@printf "  make build           Build tk binary into ./bin/tk.\n"
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
	@printf "  make test-tk-test    Run executable documentation tests.\n"
	@printf "  make lint            Run golangci-lint on all packages.\n"
	@printf "  make clean           Remove built binaries from ./bin.\n"
	@printf "\n"
	@printf "Docker targets:\n\n"
	@printf "  make docker-build    Build the Docker image.\n"
	@printf "  make docker-up       Start the service via Docker Compose.\n"
	@printf "  make docker-down     Stop the service via Docker Compose.\n"
	@printf "\n"
	@printf "Release targets:\n\n"
	@printf "  make release         Full release: build → checksums → SBOM → formula → GitHub release → homebrew tap.\n"
	@printf "  make release-publish Same as release (all-in-one).\n"
	@printf "  make release-build   Cross-compile binaries and pack tarballs into ./dist.\n"
	@printf "  make release-checksums  Write SHA256 checksums for all dist tarballs.\n"
	@printf "  make release-formula Generate homebrew/ticket.rb from the formula template.\n"
	@printf "  make release-clean   Remove the ./dist directory.\n"
	@printf "\n"

build: 
	@$(MAKE) bump-version
	@mkdir -p bin
	go build -o ./bin/tk ./cmd/tk

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
INTEGRATION_TEST_PKGS := ./cmd/tk ./internal/client ./internal/server ./internal/store ./libticket

test: test-unit test-integration test-playwright

test-go:
	TICKET_FAST_HASH=1 go test ./...

test-unit:
	TICKET_FAST_HASH=1 go test $(UNIT_TEST_PKGS)

test-integration:
	TICKET_FAST_HASH=1 go test $(INTEGRATION_TEST_PKGS)

test-go-cover:
	@export TICKET_FAST_HASH=1; set -e; \
	for entry in \
		"./cmd/tk 55" \
		"./libticket 65" \
		"./internal/client 55" \
		"./internal/store 70" \
		"./internal/server 55" \
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

test-tk-test: build
	go run ./cmd/tk-test QUICKSTART_CLIENT.md QUICKSTART_SERVER.md

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

release-checksums:
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

release-formula:
	@echo "Generating homebrew/ticket.rb for v$(VERSION)..."
	@darwin_arm64=$$(shasum -a 256 $(DIST_DIR)/tk_$(VERSION)_darwin_arm64.tar.gz | cut -d' ' -f1); \
	 darwin_amd64=$$(shasum -a 256 $(DIST_DIR)/tk_$(VERSION)_darwin_amd64.tar.gz | cut -d' ' -f1); \
	 linux_amd64=$$(shasum -a 256  $(DIST_DIR)/tk_$(VERSION)_linux_amd64.tar.gz  | cut -d' ' -f1); \
	 linux_arm64=$$(shasum -a 256  $(DIST_DIR)/tk_$(VERSION)_linux_arm64.tar.gz  | cut -d' ' -f1); \
	 sed \
		-e "s/__VERSION__/$(VERSION)/g" \
		-e "s/__DARWIN_ARM64_SHA256__/$$darwin_arm64/g" \
		-e "s/__DARWIN_AMD64_SHA256__/$$darwin_amd64/g" \
		-e "s/__LINUX_AMD64_SHA256__/$$linux_amd64/g" \
		-e "s/__LINUX_ARM64_SHA256__/$$linux_arm64/g" \
		homebrew/ticket.rb.tmpl > homebrew/ticket.rb
	@echo "Formula written to homebrew/ticket.rb"

release-publish: release-build release-checksums release-sbom release-formula
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
	@echo "Updating homebrew tap..."
	@TAP_DIR=$$(mktemp -d) && \
		trap "rm -rf $$TAP_DIR" EXIT && \
		git clone $(HOMEBREW_TAP_REPO) "$$TAP_DIR" && \
		cp homebrew/ticket.rb "$$TAP_DIR/Formula/ticket.rb" && \
		git -C "$$TAP_DIR" add Formula/ticket.rb && \
		git -C "$$TAP_DIR" commit -m "ticket $(VERSION)" && \
		git -C "$$TAP_DIR" push
	@echo "Homebrew tap updated."
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

docker-build:
	docker build -t ticket:$(VERSION) -t ticket:latest .

docker-push: docker-build
	docker tag ticket:$(VERSION) $(GHCR_IMAGE):$(VERSION)
	docker tag ticket:latest $(GHCR_IMAGE):latest
	docker push $(GHCR_IMAGE):$(VERSION)
	docker push $(GHCR_IMAGE):latest

docker-up:
	VERSION=$(VERSION) docker compose up -d

docker-down:
	VERSION=$(VERSION) docker compose down

# ─── clean ────────────────────────────────────────────────────────────────────

clean:
	@rm -rf bin

install: build
	cp ./bin/tk $$(go env GOPATH)/bin/tk

dev:
    # prints out the env vars I need to set to go into a ticket dev mode
	@echo ""
	@echo "Run the following:\n"
	@echo "export TICKET_URL=file://`pwd`/ticket.db"
	@echo "export TICKET_CONFIG_DIR=`pwd`"
	@echo "\nAnd you are now in a position to extend ticket itself.\n"
