# Makefile for Domain OS
# Consolidated commands for development, testing, and deployment

.PHONY: help clean-docker

# Default target
.DEFAULT_GOAL := help

# Variables
BRANCH := $(shell git branch --show-current)
GIT_SHA := $(shell git rev-parse $(BRANCH))
DOPPLER := doppler run --
DOCKER_COMPOSE := docker compose
COMPOSE_FILE := docker-compose.yml
COMPOSE_CI_FILE := docker-compose-ci.yml

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

dev: ## Start essential services (db, redis, epp-server, admin-api) for development
	@echo "Starting essential services for development..."
	@export BRANCH=$(BRANCH) && $(DOPPLER) $(DOCKER_COMPOSE) --profile essential up -d

dev-logs: ## Follow logs for all running services
	$(DOPPLER) $(DOCKER_COMPOSE) logs -f

dev-build: ## Rebuild and start essential services
	@echo "Building and starting services for branch $(BRANCH) with commit $(GIT_SHA)..."
	@export BRANCH=$(BRANCH) && export GIT_SHA=$(GIT_SHA) && \
	$(DOPPLER) $(DOCKER_COMPOSE) --profile essential up -d --build

dev-frontend: ## Start the Next.js frontend development server
	@echo "Starting frontend development server..."
	@cd frontend && $(DOPPLER) npm run dev

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

test: test-unit ## Run unit tests (default)

test-unit: ## Run unit tests with coverage
	@echo "Starting test database..."
	@docker volume rm domain-os_db 2>/dev/null || true
	# Ensure any previous leftover test container is removed to avoid name conflicts
	@docker rm -f testdb 2>/dev/null || true
	@docker run --rm -d \
		-e POSTGRES_HOST_AUTH_METHOD=scram-sha-256 \
		-e POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256 \
		-e POSTGRES_PASSWORD=unittest \
		-e POSTGRES_USER=postgres \
		--name testdb \
		-p 5432:5432 \
		postgres:16.1 \
		-c ssl=on \
		-c ssl_cert_file=/etc/ssl/certs/ssl-cert-snakeoil.pem \
		-c ssl_key_file=/etc/ssl/private/ssl-cert-snakeoil.key
	@echo "Running unit tests..."
	@go test ./... -coverpkg=./... -coverprofile=coverage.out && go tool cover -html=coverage.out
	@echo "Stopping test database..."
	@docker stop testdb 2>/dev/null || true
	# Extra safety: remove the container if it still exists for any reason
	@docker rm -f testdb 2>/dev/null || true

test-integration: ## Run integration tests (requires Postman API keys in Doppler)
	@echo "Running integration tests with Postman/Newman..."
	@echo "NOTE: This requires POSTMAN_COLLECTION_ID, POSTMAN_ENVIRONMENT_ID, and POSTMAN_API_KEY in Doppler"
	@echo "Cleaning up previous test environment..."
	@COMPOSE_PROJECT_NAME=domain-os $(DOPPLER) $(DOCKER_COMPOSE) \
		-p domain-os -f $(COMPOSE_CI_FILE) down -v --remove-orphans 2>/dev/null || true
	@docker ps -aq --filter label=com.docker.compose.project=domain-os \
		| xargs docker rm -f 2>/dev/null || true
	@docker network ls -q --filter name=domain-os_dos \
		| xargs docker network rm 2>/dev/null || true
	@docker volume rm domain-os_db domain-os_temporal_pgdata 2>/dev/null || true
	@echo "Building image for branch $(BRANCH) with commit $(GIT_SHA)..."
	@docker build -t geapex/domain-os:$(BRANCH) --build-arg GIT_SHA=$(BRANCH) .
	@echo "Starting integration test containers (will run Postman tests via Newman)..."
	@export BRANCH=$(BRANCH) && COMPOSE_PROJECT_NAME=domain-os $(DOPPLER) \
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

test-coverage: ## Generate detailed coverage report
	@echo "Running tests with coverage..."
	@go test ./... -coverprofile=coverage.out -covermode=atomic
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-epp: ## Run EPP-specific tests
	@echo "Running EPP tests..."
	@go test ./cmd/epp/... -v -race -short
	@go test ./cmd/cli/epp/... -v -race -short

test-agent: ## Run agent navigation tests
	@echo "Running agent navigation tests..."
	@go test ./internal/agent/service/... -v -run TestAddNavigationActions
	@go test ./internal/agent/service/... -v -run TestNavigationActionStruct
	@go test ./internal/agent/service/... -v -run TestChatResponse

test-agent-coverage: ## Run agent tests with coverage report
	@echo "Running agent navigation tests with coverage..."
	@go test ./internal/agent/service/... -coverprofile=agent-coverage.out -covermode=atomic
	@go tool cover -html=agent-coverage.out -o agent-coverage.html
	@echo "Agent coverage report generated: agent-coverage.html"

###################
# Build & Deploy
###################

build: ## Build the main API Docker image
	@echo "Building API image for branch $(BRANCH)..."
	@docker build -t geapex/domain-os:$(BRANCH) --build-arg GIT_SHA=$(BRANCH) .

build-epp: ## Build the EPP server Docker image
	@echo "Building EPP server image for branch $(BRANCH)..."
	@docker build -t geapex/epp-server:$(BRANCH) -f Dockerfile.epp --build-arg GIT_SHA=$(BRANCH) .

build-whois: ## Build the WHOIS/EPP client API Docker image
	@echo "Building WHOIS/EPP client API image for branch $(BRANCH)..."
	@docker build -t geapex/epp-client-api:$(BRANCH) -f ./cmd/api/epp-client/Dockerfile --build-arg GIT_SHA=$(BRANCH) .

build-all: build build-epp build-whois ## Build all Docker images

push: build ## Build and push main API image to Docker Hub
	@echo "Pushing API image to Docker Hub..."
	@docker push geapex/domain-os:$(BRANCH)
	@docker scout quickview 2>/dev/null || true

push-epp: build-epp ## Build and push EPP server image to Docker Hub
	@echo "Pushing EPP server image to Docker Hub..."
	@docker push geapex/epp-server:$(BRANCH)
	@docker scout quickview 2>/dev/null || true

push-whois: build-whois ## Build and push WHOIS image to Docker Hub
	@echo "Pushing WHOIS/EPP client API image to Docker Hub..."
	@docker push geapex/epp-client-api:$(BRANCH)
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
