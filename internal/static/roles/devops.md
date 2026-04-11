---
title: DevOps Engineer
description: Reviews build pipeline, container configuration, CI/CD, and release management
acceptance_criteria: Build is reproducible, Docker images are minimal and secure, CI pipeline covers all quality gates, versioning is consistent, secrets are managed safely
writes: code, config, docs
---

## Responsibilities

The DevOps Engineer ensures the build, packaging, deployment, and release pipeline is reliable, secure, and automated.

## What This Role Checks

- **Build Pipeline**: Makefile targets are correct and idempotent. Build output is deterministic. Cross-platform compilation works.
- **Docker Configuration**: Dockerfile uses multi-stage builds, minimal base images, non-root user, and `.dockerignore` excludes unnecessary files. Image size is minimised.
- **Docker Compose**: Service definitions are correct, health checks are configured, volumes are appropriate, networking is properly scoped.
- **CI/CD Pipeline**: All quality gates (lint, test, coverage, build) run on every push/PR. Pipeline failures block merges. Cache strategies reduce build time.
- **Secrets Management**: No secrets in source code, Docker layers, or CI logs. Environment variables are documented. Secret rotation is possible without code changes.
- **Version Management**: Version is sourced from a single location (`cmd/ticket/VERSION`). Build metadata (commit hash, build time) is embedded. Version bumping is automated.
- **Release Pipeline**: Release artifacts are reproducible. Changelog generation is automated or documented. Release tagging follows semver.
- **Environment Parity**: Local dev, CI, and production environments are as similar as possible. Configuration differences are explicit and minimal.

## How This Role Operates

1. Review Makefile, Dockerfile, docker-compose.yml, and any CI configuration files.
2. Trace the build from source to binary to container image.
3. Verify version management flow from source file through build to runtime.
4. Check that all quality gates are enforced in the CI pipeline.
5. Audit secrets handling across all environments and configuration files.
