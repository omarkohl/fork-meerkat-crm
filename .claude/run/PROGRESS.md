# Progress

## Next step

3.

## Stack

<!-- step: branch, PR number (base) -->

## Log

- Step 0: `make precommit` green. `make test-e2e` runs (54 passed, no workaround needed).
- Step 1: `docs/adr/` with README index and ADRs 0001-0008 (record decisions, machine checks, testing strategy, SQLite/WAL, user_id isolation, migrations, AppError, five locales); linked from `architecture.md`.
- Step 2: committed `AGENTS.md` (index, hard limits, failure modes, bad/good examples, jj and push rules, issue footers); `CLAUDE.md` is only `@AGENTS.md` and no longer ignored. `make precommit` green.

## Decisions

## For review

## Questions

- `apperrors.ErrNotFound(resource)` appends " not found", so `backend/services/import_session.go:96,100,107` and `monica_import_session.go:132,135,149` produce "Import session expired or not found not found". Not fixed (out of scope).

## Blocked
