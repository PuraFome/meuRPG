# Developer entry point for MeuRPG. Run `make help` to see all targets.
#
# Every target is written to work from the repo root, no matter which
# directory the underlying tool actually needs to run in.

# Default DATABASE_URL for host-run commands (`make run`, `make migrate`):
# it points at the CockroachDB container started by `make up`, reached from
# the host via localhost. Override on the command line if needed, e.g.:
#   make run DATABASE_URL=postgresql://root@localhost:26257/otherdb?sslmode=disable
DATABASE_URL ?= postgresql://root@localhost:26257/meurpg?sslmode=disable

# Sub-command for `make migrate`, e.g.: make migrate MIGRATE_ARGS=status
MIGRATE_ARGS ?= up

COMPOSE_FILE := deploy/local/compose.yaml

# sqlc is pinned: its version is written into every file it generates, so
# every machine and CI must run the same one. `go run pkg@version` builds
# exactly that version, checked against Go's checksum database, with no
# install step. The first run compiles it (about a minute); later runs use
# Go's build cache.
SQLC := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1

.PHONY: help proto sqlc proto-lint lint test run migrate up down logs docker-build web-install web-test web-build e2e

help: ## Show this help message
	@echo "MeuRPG - available targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

# buf's TypeScript plugin runs from web/node_modules/.bin (see
# proto/buf.gen.yaml), so `proto` depends on it as a real file target:
# installed once, then only reinstalled when package.json/package-lock.json
# change, instead of on every `make proto`.
web/node_modules/.bin/protoc-gen-es: web/package.json web/package-lock.json
	cd web && npm ci --ignore-scripts

proto: web/node_modules/.bin/protoc-gen-es ## Generate Go and TypeScript code from proto/ (writes into backend/gen and web/src/gen)
	cd proto && buf generate

sqlc: ## Generate Go code from the SQL queries (backend/sqlc.yaml) into backend/internal/*/*db
	cd backend && $(SQLC) generate

proto-lint: ## Lint proto files and check formatting
	cd proto && buf lint
	cd proto && buf format --diff --exit-code

lint: proto-lint ## Run all linters (proto + Go)
	cd backend && golangci-lint run

test: ## Run backend Go tests with the race detector
	cd backend && go test -race ./...

run: ## Run the API server on the host (needs `make up` for the DB)
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

migrate: ## Run DB migrations on the host, e.g. `make migrate MIGRATE_ARGS=status`
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate $(MIGRATE_ARGS)

up: ## Start the local stack (CockroachDB, migrate, API) in the background
	docker compose -f $(COMPOSE_FILE) up --build -d

down: ## Stop the local stack and remove its containers
	docker compose -f $(COMPOSE_FILE) down

logs: ## Follow logs from the local stack
	docker compose -f $(COMPOSE_FILE) logs -f

docker-build: ## Build the backend+web Docker image standalone (no compose)
	docker build \
		-f backend/Dockerfile \
		--build-arg VERSION=dev \
		--build-arg COMMIT=$$(git rev-parse --short HEAD) \
		-t meurpg-backend:dev \
		.

web-install: ## Install web/ dependencies (no install scripts)
	cd web && npm ci --ignore-scripts

web-test: ## Run the Angular unit tests (run `make web-install` first if needed)
	cd web && npm test

web-build: ## Build the Angular app for production (run `make web-install` first if needed)
	cd web && npm run build

# e2e/ dependencies (Playwright), installed like web/'s: once, then again
# only when package.json/package-lock.json change. No install scripts.
e2e/node_modules/.bin/playwright: e2e/package.json e2e/package-lock.json
	cd e2e && npm ci --ignore-scripts

e2e: e2e/node_modules/.bin/playwright ## Start the local stack, run the Playwright tests against it, and report
	docker compose -f $(COMPOSE_FILE) up --build -d
	cd e2e && npx playwright test; status=$$?; \
		echo "Report: e2e/playwright-report/index.html (cd e2e && npx playwright show-report). The stack is still up: make down"; \
		exit $$status
