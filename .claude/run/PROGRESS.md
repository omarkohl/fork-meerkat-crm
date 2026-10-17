# Progress

## Next step

7.

## Stack

<!-- step: branch, PR number (base) -->

## Log

- Step 0: `make precommit` green. `make test-e2e` runs (54 passed, no workaround needed).
- Step 1: `docs/adr/` with README index and ADRs 0001-0008 (record decisions, machine checks, testing strategy, SQLite/WAL, user_id isolation, migrations, AppError, five locales); linked from `architecture.md`.
- Step 2: committed `AGENTS.md` (index, hard limits, failure modes, bad/good examples, jj and push rules, issue footers); `CLAUDE.md` is only `@AGENTS.md` and no longer ignored. `make precommit` green.
- Step 3: targets declare `install` as prerequisite; added `test-coverage`, `arch` (placeholder), `seed` (`scripts/seed.mjs`, needs `make dev` running; tested against a temp backend); `test-e2e` is in `precommit`; Checks in AUTONOMOUS-RUNS.md and run-one-test table in testing.md updated. `make precommit` green incl. E2E (54 passed).
- Step 4: Makefile split into `install-go`/`install-frontend`, `lint-go`/`lint-frontend`, `test-go`/`test-frontend`, `test-coverage-go`/`-frontend`, `test-carddav`, `test-e2e-docker`, `vuln-go`, `audit-frontend`, `fuzz`. Lint, unit, e2e, security and fuzz workflows call them (docs, docker-build, docker-publish unchanged: Jekyll and image build, not make). actionlint v1.7.7 clean; Go version in fuzz.yml and the CardDAV job of unit-tests.yml raised 1.26 -> 1.27 (needed: `backend/go.mod` requires 1.27.0); CI itself not run locally. `make precommit` green.
- Step 5: the full reformat (`04bed7d6` "style: format all files with gofmt and Prettier") and `format-check` on all files (no `BASE` logic) were already in the history below this run (`4eb6e3b1`); `make format` changes nothing. So no new reformat commit; this commit only adds `.git-blame-ignore-revs` for `04bed7d6` and a note in contributing.md. `make precommit` green.
- Step 6: enabled `noUncheckedIndexedAccess`, `noImplicitOverride`, `noPropertyAccessFromIndexSignature` in `frontend/tsconfig.json`; fixed 86 errors (bracket access for index signatures incl. `process.env['REACT_APP_...']`, `override` on class members, destructuring with defaults for regex/split parts, `.slice(0, 10)` instead of `.split('T')[0]` in four dialogs, `?.` in tests). No non-null assertions. The three flags are also in `tsconfig.node.json` (E2E and Playwright config) and `make typecheck` now checks that project too. `make precommit` green incl. E2E.

## Decisions

- CI sets `YARN: yarn` (runner yarn) because the Makefile default `corepack yarn` needs a `packageManager` field.
- golangci-lint job uses the action with `install-only: true`, then `make lint-go` (adds `go vet`); loses the action's lint cache.

- `test-coverage` is not in `precommit` yet (step 8 adds thresholds first).

## For review

- Step 4: CI cannot run locally. Check the first PR run: `install-only` of golangci-lint-action v8, `YARN: yarn`, `make fuzz` env vars.

## Questions

- `new Date().toISOString().slice(0, 10)` (and the older `.split('T')[0]`) gives the UTC date, so default dates in AddNote/AddActivity and the reminder `min` can show the wrong day near midnight. Pre-existing; not fixed.

- `frontend/package.json` has no `packageManager` field, so `corepack yarn` (Makefile default) needs CI to set `YARN=yarn`. Adding the field would remove that workaround; not done (out of scope).

- `apperrors.ErrNotFound(resource)` appends " not found", so `backend/services/import_session.go:96,100,107` and `monica_import_session.go:132,135,149` produce "Import session expired or not found not found". Not fixed (out of scope).

## Blocked
