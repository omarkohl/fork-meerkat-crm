.PHONY: help install dev seed format format-check lint typecheck i18n-check test test-coverage test-e2e arch precommit build

YARN ?= corepack yarn

help: ## Show this help message
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

install: ## Install backend and frontend dependencies
	cd backend && go mod download
	cd frontend && $(YARN) install --frozen-lockfile

dev: install ## Run backend and frontend dev servers (no setup; optional overrides in dev.env)
	YARN="$(YARN)" ./scripts/dev.sh

seed: ## Add a demo user (demo / DemoPassword123!) and contacts to the running "make dev" backend
	set -a; [ ! -f dev.env ] || . ./dev.env; set +a; node scripts/seed.mjs

PRETTIER = frontend/node_modules/.bin/prettier

format: install ## gofmt and Prettier (TS, JSON, YAML, Markdown, ...) on all files
	cd backend && gofmt -w .
	$(PRETTIER) --write --log-level warn .

format-check: install ## Fail if any file is not gofmt/Prettier-clean
	@bad=$$(cd backend && gofmt -l .); \
	if [ -n "$$bad" ]; then echo "Not gofmt-clean:"; echo "$$bad"; exit 1; fi
	$(PRETTIER) --check --log-level warn .

lint: install ## go vet + golangci-lint + eslint
	cd backend && go vet ./...
	cd backend && golangci-lint run
	cd frontend && npx eslint --max-warnings=0 src

typecheck: install ## TypeScript type check
	cd frontend && npx tsc --noEmit

i18n-check: install ## Locale files: missing/extra keys fail, unused/duplicate keys warn (scripts/i18n-check.mjs)
	node --test scripts/*.test.mjs
	node scripts/i18n-check.mjs

test: install ## Backend and frontend unit tests
	cd backend && go test ./...
	cd frontend && $(YARN) test

test-coverage: install ## Tests with coverage (backend coverage.out, frontend coverage/)
	cd backend && go test -coverprofile=coverage.out ./...
	cd frontend && $(YARN) test:coverage

test-e2e: install ## Playwright E2E against a local stack (no Docker; see scripts/e2e-stack.mjs)
	node scripts/e2e-stack.mjs -- npx playwright test $(ARGS)

arch: install ## Architecture rules (module-graph checks; placeholder until they exist)
	@echo "arch: no rules yet"

precommit: format-check lint typecheck i18n-check arch test test-e2e ## All checks, including E2E

build: install ## Build backend binary and frontend bundle
	cd backend && go build -o meerkat .
	cd frontend && REACT_APP_API_URL= $(YARN) build
