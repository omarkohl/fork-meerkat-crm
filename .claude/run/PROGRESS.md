# Progress

## Next step

5.

## Stack

<!-- step: branch, PR number (base) -->

## Log

- Step 0: `make precommit` green. `make test-e2e` runs (54 passed, no workaround needed).
- Step 1: `docs/adr/` with README index and ADRs 0001-0008 (record decisions, machine checks, testing strategy, SQLite/WAL, user_id isolation, migrations, AppError, five locales); linked from `architecture.md`.
- Step 2: committed `AGENTS.md` (index, hard limits, failure modes, bad/good examples, jj and push rules, issue footers); `CLAUDE.md` is only `@AGENTS.md` and no longer ignored. `make precommit` green.
- Step 3: targets declare `install` as prerequisite; added `test-coverage`, `arch` (placeholder), `seed` (`scripts/seed.mjs`, needs `make dev` running; tested against a temp backend); `test-e2e` is in `precommit`; Checks in AUTONOMOUS-RUNS.md and run-one-test table in testing.md updated. `make precommit` green incl. E2E (54 passed).
- Step 4: Makefile split into `install-go`/`install-frontend`, `lint-go`/`lint-frontend`, `test-go`/`test-frontend`, `test-coverage-go`/`-frontend`, `test-carddav`, `test-e2e-docker`, `vuln-go`, `audit-frontend`, `fuzz`. Lint, unit, e2e, security and fuzz workflows call them (docs, docker-build, docker-publish unchanged: Jekyll and image build, not make). actionlint v1.7.7 clean; Go version in fuzz.yml and the CardDAV job of unit-tests.yml raised 1.26 -> 1.27 (needed: `backend/go.mod` requires 1.27.0); CI itself not run locally. `make precommit` green.

## Decisions

- CI sets `YARN: yarn` (runner yarn) because the Makefile default `corepack yarn` needs a `packageManager` field.
- golangci-lint job uses the action with `install-only: true`, then `make lint-go` (adds `go vet`); loses the action's lint cache.

- `test-coverage` is not in `precommit` yet (step 8 adds thresholds first).

## For review

- Step 4: CI cannot run locally. Check the first PR run: `install-only` of golangci-lint-action v8, `YARN: yarn`, `make fuzz` env vars.

## Questions

- `frontend/package.json` has no `packageManager` field, so `corepack yarn` (Makefile default) needs CI to set `YARN=yarn`. Adding the field would remove that workaround; not done (out of scope).

- `apperrors.ErrNotFound(resource)` appends " not found", so `backend/services/import_session.go:96,100,107` and `monica_import_session.go:132,135,149` produce "Import session expired or not found not found". Not fixed (out of scope).

## Blocked
