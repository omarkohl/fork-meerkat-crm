.PHONY: help install dev format format-check lint typecheck i18n-check test test-e2e precommit build

YARN ?= corepack yarn

help: ## Show this help message
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

install: ## Install backend and frontend dependencies
	cd backend && go mod download
	cd frontend && $(YARN) install --frozen-lockfile

dev: ## Run backend and frontend dev servers (no setup; optional overrides in dev.env)
	YARN="$(YARN)" ./scripts/dev.sh

PRETTIER = frontend/node_modules/.bin/prettier

format: ## gofmt and Prettier (TS, JSON, YAML, Markdown, ...) on all files
	cd backend && gofmt -w .
	$(PRETTIER) --write --log-level warn .

format-check: ## Fail if any file is not gofmt/Prettier-clean
	@bad=$$(cd backend && gofmt -l .); \
	if [ -n "$$bad" ]; then echo "Not gofmt-clean:"; echo "$$bad"; exit 1; fi
	$(PRETTIER) --check --log-level warn .

lint: ## go vet + golangci-lint + eslint
	cd backend && go vet ./...
	cd backend && golangci-lint run
	cd frontend && npx eslint --max-warnings=0 src

typecheck: ## TypeScript type check
	cd frontend && npx tsc --noEmit

i18n-check: ## Locale files: missing/extra keys fail, unused/duplicate keys warn (scripts/i18n-check.mjs)
	node --test scripts/*.test.mjs
	node scripts/i18n-check.mjs

test: ## Backend and frontend unit tests
	cd backend && go test ./...
	cd frontend && $(YARN) test

test-e2e: ## Playwright E2E against a local stack (no Docker; see scripts/e2e-stack.mjs)
	node scripts/e2e-stack.mjs -- npx playwright test $(ARGS)

precommit: format-check lint typecheck i18n-check test ## All checks except E2E

build: ## Build backend binary and frontend bundle
	cd backend && go build -o meerkat .
	cd frontend && REACT_APP_API_URL= $(YARN) build
