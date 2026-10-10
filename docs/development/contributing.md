---
title: Contributing
parent: Development
nav_order: 5
---

# Contributing

## Getting Started

1. Fork the repository and clone your fork locally.
2. Follow the setup steps in [Backend](backend.md) and [Frontend](frontend.md).
3. Create a feature branch from `main`.

## Development Workflow

1. Make your changes with tests.
2. Run `make precommit` from the repository root: checks gofmt and Prettier formatting of the files you changed (`make format` fixes it), `go vet`, ESLint, `tsc` and the Go and Vitest unit tests. Run `make test-e2e` (Playwright, no Docker needed) when you change the UI or API behaviour. `make help` lists all targets.
3. Open a pull request against `main`.

## Code Style

**Backend:** Standard Go formatting (`gofmt`), checked together with `golangci-lint` (config in `backend/.golangci.yml`: errcheck, staticcheck, gosec, revive, bodyclose, sqlclosecheck). Run `cd backend && golangci-lint run` locally. Follow existing controller and service patterns with thin controllers, logic in services, always scope queries by `user_id`.

**Frontend:** Prettier formatting (`make format` fixes the files you changed), ESLint without warnings and `tsc --noEmit`. TypeScript strict mode and MUI for all UI components. All user-facing strings through `i18next`. Custom hooks for data fetching, not inline `useEffect` + `fetch`.

## Continuous integration

Pull requests run these GitHub Actions workflows, and all must pass:

- **Lint** (`lint.yml`): golangci-lint, ESLint, `tsc`, and a gofmt/Prettier check of the files the change touches.
- **Unit tests** and **E2E tests** (`unit-tests.yml`, `e2e-tests.yml`).
- **Security** (`security.yml`): `govulncheck` and `yarn audit` for production dependencies; also runs weekly.

Dependabot (`.github/dependabot.yml`) opens weekly update PRs for Go modules, npm, GitHub Actions, Docker and the docs' Ruby gems, with minor and patch updates grouped.

## Pull Requests

- Keep PRs focused with one feature or fix per PR.
- AI tools may assist coding but you are responsible for the code quality. Do not open hands-off vibe-coded PRs. In those cases rather open a feature request instead.
- Describe what changed and why, not how.

## Reporting Issues

Open an issue on [GitHub](https://github.com/fbuchner/meerkat-crm/issues/new/choose). Try to include steps to reproduce for bugs. Use the feature request template for new ideas.
