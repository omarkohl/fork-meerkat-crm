# Progress

## Next step

4.

## Stack

<!-- step: branch, PR number (base) -->

## Log

- Step 0: `make precommit` green. `make test-e2e` runs (54 passed, no workaround needed).
- Step 1: `docs/adr/` with README index and ADRs 0001-0008 (record decisions, machine checks, testing strategy, SQLite/WAL, user_id isolation, migrations, AppError, five locales); linked from `architecture.md`.
- Step 2: committed `AGENTS.md` (index, hard limits, failure modes, bad/good examples, jj and push rules, issue footers); `CLAUDE.md` is only `@AGENTS.md` and no longer ignored. `make precommit` green.
- Step 3: targets declare `install` as prerequisite; added `test-coverage`, `arch` (placeholder), `seed` (`scripts/seed.mjs`, needs `make dev` running; tested against a temp backend); `test-e2e` is in `precommit`; Checks in AUTONOMOUS-RUNS.md and run-one-test table in testing.md updated. `make precommit` green incl. E2E (54 passed).

## Decisions

- `test-coverage` is not in `precommit` yet (step 8 adds thresholds first).

## For review

## Questions

- `apperrors.ErrNotFound(resource)` appends " not found", so `backend/services/import_session.go:96,100,107` and `monica_import_session.go:132,135,149` produce "Import session expired or not found not found". Not fixed (out of scope).

## Blocked
