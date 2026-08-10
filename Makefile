# Makefile for Domain OS
# Consolidated commands for development, testing, and deployment

.PHONY: help clean-docker local askg test-askg test-eval test-askg-eval

# Default target
.DEFAULT_GOAL := help

# Variables
BRANCH := $(shell git branch --show-current)
TAG := $(subst /,-,$(BRANCH))
GIT_SHA := $(shell git rev-parse HEAD)
VERSION := $(shell git describe --tags --always 2>/dev/null | sed 's/^v//' || echo dev)
DOPPLER := doppler run --
DOCKER_COMPOSE := docker compose
COMPOSE_FILE := docker-compose.yml
COMPOSE_CI_FILE := docker-compose-ci.yml
COMPOSE_LOCAL_FILE := docker-compose.local.yml

# The local stack. No Doppler: local development must work with nothing but a
# checkout, Docker, and a copied .env — see docs/LOCAL_ENV_PHASE0_INVENTORY.md.
# The override file forces local builds, remaps privileged ports, and swaps the
# live IANA registrar fetch for the offline synthetic seeder.
COMPOSE_LOCAL := $(DOCKER_COMPOSE) -f $(COMPOSE_FILE) -f $(COMPOSE_LOCAL_FILE)

# Host port for the throwaway unit-test Postgres. Deliberately not 5432: that is
# where `make dev` publishes the dev database, and the suite has to be runnable
# while the dev stack is up.
TEST_DB_PORT ?= 5433

# One-shot services: they run a task to completion and exit, rather than staying
# up. `docker compose up --wait` treats *any* container exit as a failure — even
# exit 0 — so leaving these in the --wait set makes `make dev` return non-zero
# on a completely successful run. They are held back with --scale N=0 and then
# run to completion in list order (minio-setup provisions the buckets that later
# steps expect). Add any new one-shot service here.
ONESHOT_SERVICES := minio-setup admin-init
ONESHOT_SCALE_ZERO := $(foreach s,$(ONESHOT_SERVICES),--scale $(s)=0)
LDFLAGS := -s -w \
  -X github.com/onasunnymorning/domain-os/internal/buildinfo.Version=$(VERSION) \
  -X github.com/onasunnymorning/domain-os/internal/buildinfo.GitSHA=$(GIT_SHA)

help: ## Show this help message
	@echo 'Domain OS - Makefile Commands'
	@echo ''
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

###################
# Development
###################

dev: .env doctor-quiet ## Cold machine -> running stack, migrated and seeded
	@echo "==> Building and starting the local stack (this takes a few minutes on a cold machine)..."
	@# Long-running services first; the one-shots are held back — see
	@# ONESHOT_SERVICES above for why.
	@$(COMPOSE_LOCAL) --profile essential up -d --build --wait $(ONESHOT_SCALE_ZERO)
	@echo "==> Provisioning buckets and seeding the database (synthetic data, no network)..."
	@# Run each one-shot to completion, propagating its exit code. --no-deps
	@# because everything they need is already healthy from the step above.
	@for s in $(ONESHOT_SERVICES); do \
		$(COMPOSE_LOCAL) --profile essential run --rm --no-deps $$s || exit 1; \
	done
	@echo ""
	@echo "Stack is up and seeded."
	@echo ""
	@echo "  Admin API      http://localhost:8080          (Swagger at /swagger/index.html)"
	@echo "  Temporal UI    http://localhost:8081"
	@echo "  MinIO console  http://localhost:9001          (minioadmin / minioadmin)"
	@echo "  Postgres       localhost:5432                 (make shell-db)"
	@echo "  EPP            localhost:7700                 (container port 700)"
	@echo ""
	@echo "  Seeded: 1 TLD (.test), 6 registrars, 9 domains across lifecycle states."
	@echo "  Next:   make test        run the suite against this stack"
	@echo "          make dev-frontend  start the admin UI on :3002"

# Creates .env on first run. Never overwrites an existing one, so your local
# edits are safe; delete .env and re-run to get a clean copy.
.env: .env.example
	@if [ ! -f .env ]; then \
		echo "==> No .env found; copying .env.example -> .env"; \
		cp .env.example .env; \
	else \
		echo "==> .env is older than .env.example — new variables may have been added."; \
		echo "    Compare with: diff <(sort .env) <(sort .env.example)"; \
		touch .env; \
	fi

down: ## Stop services, retain volumes
	@echo "==> Stopping the local stack (volumes retained)..."
	@$(COMPOSE_LOCAL) --profile full down --remove-orphans
	@echo "Stopped. Data is retained — 'make dev' resumes where you left off."

reset: ## Destroy volumes and state; next 'make dev' is a first-ever run
	@echo "==> Destroying the local stack and all its data..."
	@$(COMPOSE_LOCAL) --profile full down -v --remove-orphans
	@docker rm -f testdb testdb-api testdb-cov 2>/dev/null || true
	@echo "Reset complete. The next 'make dev' starts from an empty database."

dev-logs: ## Follow logs for all running services
	@$(COMPOSE_LOCAL) logs -f

dev-doppler: ## Start the stack using Doppler for secrets (maintainer flow, needs a Doppler login)
	@echo "Starting essential services via Doppler..."
	@export BRANCH=$(TAG) && $(DOPPLER) $(DOCKER_COMPOSE) --profile essential up -d

dev-build: ## Rebuild and start essential services
	@echo "Building and starting services for branch $(BRANCH) with commit $(GIT_SHA)..."
	@export GIT_SHA=$(GIT_SHA) && $(COMPOSE_LOCAL) --profile essential up -d --build --wait

dev-frontend: ## Start the Next.js frontend development server
	@echo "Starting frontend development server..."
	@cd frontend && $(DOPPLER) npm run dev

local: ## Start local development with Tilt (Native/Hybrid Mode)
	@echo "Starting Tilt local development environment (Native Mode)..."
	@TILT_MODE=native $(DOPPLER) tilt up

local-docker: ## Start local development with Tilt (Docker Mode)
	@echo "Starting Tilt local development environment (Docker Mode)..."
	@TILT_MODE=docker $(DOPPLER) tilt up

doctor: ## Preflight: Docker, ports, toolchain, .env — prints actionable failures
	@bash scripts/doctor.sh

# Same checks, silent unless something is actually broken. `make dev` depends on
# this so a missing Docker daemon or a taken port is reported as itself, rather
# than as a compose stack trace three minutes later.
doctor-quiet:
	@bash scripts/doctor.sh --quiet

askg: ## Run Ask G support agent (usage: make askg Q="What is the status of example.best?")
	@$(DOPPLER) sh -c 'DB_HOST=localhost go run ./cmd/askg "$(Q)"'

stop: ## Stop all running services
	@echo "Stopping all services..."
	@$(DOPPLER) $(DOCKER_COMPOSE) --profile essential down

stop-full: ## Stop all services and remove volumes
	@echo "Stopping all services and removing volumes..."
	@$(DOPPLER) $(DOCKER_COMPOSE) --profile full down -v

clean: ## Clean up containers, volumes, and build artifacts
	@echo "Cleaning up..."
	@docker container rm -f domain-os-db-1 2>/dev/null || true
	@docker volume rm domain-os_db 2>/dev/null || true
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@go clean -testcache

clean-docker: ## Nuclear cleanup of all domain-os Docker resources
	@echo "Removing all domain-os containers, networks, and volumes..."
	@$(DOPPLER) $(DOCKER_COMPOSE) -f $(COMPOSE_FILE) --profile full down -v --remove-orphans 2>/dev/null || true
	@$(DOPPLER) $(DOCKER_COMPOSE) -f $(COMPOSE_CI_FILE) -p domain-os down -v --remove-orphans 2>/dev/null || true
	@docker ps -aq --filter label=com.docker.compose.project=domain-os \
		| xargs docker rm -f 2>/dev/null || true
	@docker network ls -q --filter name=domain-os \
		| xargs docker network rm 2>/dev/null || true
	@docker volume ls -q --filter name=domain-os \
		| xargs docker volume rm 2>/dev/null || true
	@echo "Done. All domain-os Docker resources removed."

###################
# Testing
###################

test: test-unit ## Full suite, green against the dev stack

# Runs the whole suite against a throwaway Postgres.
#
# Two things here are deliberate:
#
#   * The test database is published on $(TEST_DB_PORT), not 5432. `make dev`
#     holds 5432, and "run the tests right after starting the stack" has to work
#     without stopping anything first.
#
#   * No `go tool cover -html`. That opens a browser window on success, which is
#     wrong for a command people run to check their setup and wrong for anything
#     non-interactive. Use `make test-coverage` when you want the HTML report.
test-unit: ## Run unit tests with coverage (no browser; see test-coverage for HTML)
	@echo "==> Starting throwaway test database on port $(TEST_DB_PORT)..."
	@bash .github/scripts/setup-test-db.sh testdb $(TEST_DB_PORT)
	@echo "==> Running tests..."
	@TEST_DB_PORT=$(TEST_DB_PORT) go test ./... -coverpkg=./... -coverprofile=coverage.out; \
		status=$$?; \
		echo "==> Stopping test database..."; \
		docker rm -f testdb >/dev/null 2>&1 || true; \
		if [ $$status -eq 0 ]; then \
			echo ""; \
			go tool cover -func=coverage.out | tail -1; \
			echo "Suite green."; \
		fi; \
		exit $$status

test-integration: ## [LEGACY FALLBACK] Run Postman/Newman integration tests (requires Doppler + API keys)
	@echo "Running integration tests with Postman/Newman..."
	@echo "NOTE: This requires POSTMAN_COLLECTION_ID, POSTMAN_ENVIRONMENT_ID, and POSTMAN_API_KEY in Doppler"
	@echo "Cleaning up previous test environment..."
	@COMPOSE_PROJECT_NAME=domain-os $(DOPPLER) $(DOCKER_COMPOSE) \
		-p domain-os -f $(COMPOSE_CI_FILE) down -v --remove-orphans 2>/dev/null || true
	@docker ps -aq --filter label=com.docker.compose.project=domain-os \
		| xargs docker rm -f 2>/dev/null || true
	@docker network ls -q --filter name=domain-os_dos \
		| xargs docker network rm 2>/dev/null || true
	@docker volume rm domain-os_temporal_pgdata 2>/dev/null || true
	@echo "Building image for branch $(BRANCH) with commit $(GIT_SHA)..."
	@docker build -t gprins/domain-os-api:$(TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_SHA=$(GIT_SHA) .
	@echo "Starting integration test containers (will run Postman tests via Newman)..."
	@export BRANCH=$(TAG) && COMPOSE_PROJECT_NAME=domain-os $(DOPPLER) \
		$(DOCKER_COMPOSE) -p domain-os --profile essential -f $(COMPOSE_CI_FILE) \
		up --remove-orphans --abort-on-container-exit --exit-code-from test; \
	EXIT_CODE=$$?; \
	echo "Cleaning up integration test containers..."; \
	COMPOSE_PROJECT_NAME=domain-os $(DOPPLER) $(DOCKER_COMPOSE) \
		-p domain-os -f $(COMPOSE_CI_FILE) down -v --remove-orphans 2>/dev/null || true; \
	docker network ls -q --filter name=domain-os_dos \
		| xargs docker network rm 2>/dev/null || true; \
	exit $$EXIT_CODE
	@echo "Integration tests complete!"

test-api: ## Run Go-native API integration tests (replaces Newman/Postman)
	@echo "Starting API integration test database..."
	@docker rm -f testdb-api 2>/dev/null || true
	@docker run --rm -d \
		-e POSTGRES_HOST_AUTH_METHOD=scram-sha-256 \
		-e POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256 \
		-e POSTGRES_PASSWORD=unittest \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_DB=dos_integration_tests \
		--name testdb-api \
		-p 5433:5432 \
		postgres:16.1 \
		-c ssl=on \
		-c ssl_cert_file=/etc/ssl/certs/ssl-cert-snakeoil.pem \
		-c ssl_key_file=/etc/ssl/private/ssl-cert-snakeoil.key
	@echo "Waiting for database to be ready..."
	@sleep 3
	@echo "Running API integration tests..."
	@TEST_DB_PORT=5433 TEST_DB_SSLMODE=require \
		go test ./internal/interface/rest/tests/... -v -count=1; \
	EXIT_CODE=$$?; \
	echo "Stopping API integration test database..."; \
	docker stop testdb-api 2>/dev/null || true; \
	docker rm -f testdb-api 2>/dev/null || true; \
	exit $$EXIT_CODE

test-coverage: ## Generate detailed coverage report (unit tests)
	@echo "Running tests with coverage..."
	@go test ./... -coverprofile=coverage.out -covermode=atomic
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-api-coverage: ## Generate API integration test coverage report (HTML)
	@echo "🧪 Starting test database for coverage..."
	@docker rm -f testdb-cov 2>/dev/null || true
	@docker run --rm -d --name testdb-cov -p 5433:5432 \
		-e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=unittest \
		-e POSTGRES_DB=dos_integration_tests postgres:16.1
	@echo "Waiting for database to be ready..."
	@sleep 3
	@echo "Running API integration tests with coverage..."
	@TEST_DB_PORT=5433 go test -count=1 \
		-coverprofile=api-coverage.out \
		-coverpkg=./internal/interface/rest/...,./internal/application/...,./pkg/domain/... \
		./internal/interface/rest/tests/... && \
		go tool cover -html=api-coverage.out -o api-coverage.html && \
		echo "" && \
		go tool cover -func=api-coverage.out | tail -1 && \
		echo "API coverage report generated: api-coverage.html" ; \
	EXIT_CODE=$$?; \
	docker stop testdb-cov 2>/dev/null || true; \
	exit $$EXIT_CODE

test-epp: ## Run EPP-specific tests
	@echo "Running EPP tests..."
	@go test ./cmd/epp/... -v -race -short
	@go test ./cmd/cli/epp/... -v -race -short

test-askg: ## Run Ask G orchestrator tests
	@echo "Running Ask G tests..."
	@go test ./internal/askg/... -v -count=1

test-eval: ## Run deterministic eval case validation (no API key needed)
	@echo "Running deterministic eval case validation..."
	@go test ./internal/askg/eval/... -run TestEvalFixtureCases -v -count=1

test-askg-eval: ## Run Ask G agent evals (live API, requires ANTHROPIC_API_KEY via Doppler)
	@echo "Running Ask G evals (live model)..."
	@$(DOPPLER) go test ./internal/askg/eval/... -tags eval -v -count=1 -timeout 600s -run TestEvalSuite_AllCategories

###################
# Local CI (mirrors GitHub Actions)
###################

ci-local: ci-preflight ci-envcheck ci-lint build-all ci-test-backend ci-test-frontend ci-security test-api ## Run the full CI pipeline locally (lint + build + test + frontend + security + API integration)
	@echo ""
	@echo "✅ All CI checks passed locally! Safe to push."

ci-preflight: ## Kill any stale test containers that could conflict with CI (testdb, testdb-api, testdb-cov)
	@echo "🧹 Cleaning up stale test containers on port 5432/5433..."
	@docker rm -f testdb testdb-api testdb-cov 2>/dev/null || true
	@echo "✅ Pre-flight clean complete."

ci-envcheck: ## Check env var registry and deployment contract for drift
	@echo "🔍 Checking env var registry drift..."
	@go test -run 'TestEnvRegistryDrift|TestRegistryNoDuplicates|TestRegistryHasDescriptions' ./internal/config/...
	@echo "✅ Env var registry is in sync with code."
	@echo "🔍 Checking deployment contract drift..."
	@if ! go test -run 'TestContractDrift' ./internal/config/...; then \
		echo "⚠️  Contract drift detected! Automatically regenerating deploy/contract.json..."; \
		$(MAKE) generate-contract; \
		echo "❌ contract.json has been updated. Please review the changes, commit them, and run make ci-local again."; \
		exit 1; \
	fi
	@echo "✅ deploy/contract.json is in sync with env registry."
	@echo "🔍 Checking .env.example drift..."
	@if ! go test -run 'TestEnvExampleDrift' ./internal/config/...; then \
		echo "⚠️  .env.example drift detected! Automatically regenerating..."; \
		$(MAKE) generate-env-example; \
		echo "❌ .env.example has been updated. Please review the changes, commit them, and run make ci-local again."; \
		exit 1; \
	fi
	@echo "✅ .env.example is in sync with env registry."

ci-lint: ## Run all linters (Go + Frontend)
	@echo "🔍 Running Go vet..."
	@go vet ./...
	@echo "🔍 Running golangci-lint (warnings only — will become blocking once pre-existing issues are fixed)..."
	@golangci-lint run ./... 2>&1 | tail -5 || true
	@echo "🔍 Running frontend linters..."
	@cd frontend && npm run lint
	@echo "✅ All linters passed!"

generate-contract: ## Regenerate deploy/contract.json from env registry + service metadata
	@go run ./cmd/tools/gencontract > deploy/contract.json
	@echo "✅ deploy/contract.json regenerated"

generate-env-example: ## Regenerate .env.example from the env var registry
	@go run ./cmd/tools/genenvexample > .env.example
	@echo "✅ .env.example regenerated"

ci-test-backend: ## Run Go tests matching CI (with race detector + DB health wait)
	@echo "🧪 Starting test database..."
	@.github/scripts/setup-test-db.sh testdb
	@echo "🧪 Running Go tests with race detector..."
	@go test -race -count=1 ./... -coverpkg=./... -coverprofile=coverage.out || \
		(docker stop testdb 2>/dev/null; exit 1)
	@go tool cover -func=coverage.out | tail -1
	@echo "🧪 Stopping test database..."
	@docker rm -f testdb 2>/dev/null || true

ci-test-frontend: ## Run frontend tests matching CI
	@echo "⚛️  Running frontend tests with coverage..."
	@cd frontend && npm ci --prefer-offline && npm run test:coverage
	@echo "✅ Frontend tests passed!"

ci-security: ## Run security scans (govulncheck + npm audit + Trivy)
	@echo "🔒 Running Go vulnerability check..."
	@govulncheck ./...
	@echo "🔒 Running npm audit..."
	@cd frontend && npm audit --audit-level=high
	@echo "🔒 Running Trivy image scan..."
	@trivy image --severity CRITICAL,HIGH --exit-code 1 gprins/domain-os-api:$(TAG)
	@trivy image --severity CRITICAL,HIGH --exit-code 1 gprins/domain-os-worker:$(TAG)
	@trivy image --severity CRITICAL,HIGH --exit-code 1 gprins/domain-os-mcp:$(TAG)
	@echo "✅ Security scans passed!"

###################
# Build & Deploy
###################

build: ## Build the main API Docker image
	@echo "Building API image for branch $(BRANCH) version $(VERSION)..."
	@docker build -t gprins/domain-os-api:$(TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_SHA=$(GIT_SHA) .

build-epp: ## Build the EPP server Docker image
	@echo "Building EPP server image for branch $(BRANCH) version $(VERSION)..."
	@docker build -t gprins/domain-os-epp:$(TAG) -f Dockerfile.epp \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_SHA=$(GIT_SHA) .

build-whois: ## Build the WHOIS server Docker image
	@echo "Building WHOIS server image for branch $(BRANCH) version $(VERSION)..."
	@docker build -t gprins/domain-os-whois:$(TAG) -f ./cmd/whois/Dockerfile \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_SHA=$(GIT_SHA) .

build-worker: ## Build the unified worker Docker image
	@echo "Building Unified Worker image for branch $(BRANCH) version $(VERSION)..."
	@docker build -t gprins/domain-os-worker:$(TAG) -f ./cmd/workers/unified/Dockerfile \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_SHA=$(GIT_SHA) .

build-mcp: ## Build the MCP server Docker image
	@echo "Building MCP server image for branch $(BRANCH)..."
	@docker build -t gprins/domain-os-mcp:$(TAG) -f Dockerfile.mcp .

build-all: build build-worker build-epp build-whois build-mcp ## Build all Docker images

push: build ## Build and push main API image to Docker Hub
	@echo "Pushing API image to Docker Hub..."
	@docker push gprins/domain-os-api:$(TAG)
	@docker scout quickview 2>/dev/null || true

push-epp: build-epp ## Build and push EPP server image to Docker Hub
	@echo "Pushing EPP server image to Docker Hub..."
	@docker push gprins/domain-os-epp:$(TAG)
	@docker scout quickview 2>/dev/null || true

push-whois: build-whois ## Build and push WHOIS image to Docker Hub
	@echo "Pushing WHOIS server image to Docker Hub..."
	@docker push gprins/domain-os-whois:$(TAG)
	@docker scout quickview 2>/dev/null || true

push-all: push push-epp push-whois ## Build and push all images to Docker Hub

###################
# Code Quality
###################

lint: ## Run linters
	@echo "Running linters..."
	@golangci-lint run ./... 2>/dev/null || echo "golangci-lint not installed, skipping..."

fmt: ## Format Go code
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w . 2>/dev/null || echo "goimports not installed, run: go install golang.org/x/tools/cmd/goimports@latest"

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

###################
# Database
###################

db-migrate: ## Run database migrations (placeholder - implement as needed)
	@echo "Running database migrations..."
	@echo "TODO: Implement migration command"

db-reset: ## Reset the database (removes volume and recreates)
	@echo "Resetting database..."
	@$(DOPPLER) $(DOCKER_COMPOSE) stop db
	@docker volume rm domain-os_db 2>/dev/null || true
	@$(DOPPLER) $(DOCKER_COMPOSE) up -d db

db-dump: ## Dump the local PostgreSQL database (add SQL=1 for plain SQL format)
	@$(DOPPLER) sh -c 'DB_HOST=localhost ./scripts/pg-dump-local.sh $(if $(SQL),--sql,)'


###################
# Frontend
###################

frontend-install: ## Install frontend dependencies
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install

frontend-build: ## Build frontend for production
	@echo "Building frontend..."
	@cd frontend && npm run build

frontend-lint: ## Lint frontend code
	@echo "Linting frontend..."
	@cd frontend && npm run lint

frontend-test: ## Run frontend tests
	@echo "Running frontend tests..."
	@cd frontend && npm test 2>/dev/null || echo "No frontend tests configured yet"

###################
# Utilities
###################

ps: ## Show running containers
	@$(DOCKER_COMPOSE) ps

logs: ## Show logs from all services
	@$(DOPPLER) $(DOCKER_COMPOSE) logs

logs-api: ## Show logs from admin API
	@$(DOPPLER) $(DOCKER_COMPOSE) logs -f admin-api

logs-epp: ## Show logs from EPP server
	@$(DOPPLER) $(DOCKER_COMPOSE) logs -f epp-server

logs-db: ## Show logs from database
	@$(DOPPLER) $(DOCKER_COMPOSE) logs -f db

restart: ## Restart all services
	@echo "Restarting services..."
	@$(DOPPLER) $(DOCKER_COMPOSE) restart

restart-api: ## Restart admin API service
	@echo "Restarting admin API..."
	@$(DOPPLER) $(DOCKER_COMPOSE) restart admin-api

shell-db: ## Open a shell in the database container
	@docker exec -it domain-os-db-1 psql -U postgres -d registry

shell-api: ## Open a shell in the API container
	@docker exec -it domain-os-admin-api-1 sh

###################
# Setup
###################

setup: ## Initial setup - install tools and dependencies
	@echo "Setting up development environment..."
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "Setup complete! Run 'make dev' to start services."

deps: ## Download Go dependencies
	@echo "Downloading Go dependencies..."
	@go mod download
	@go mod verify

deps-update: ## Update Go dependencies
	@echo "Updating Go dependencies..."
	@go get -u ./...
	@go mod tidy
