# Progress

## Next step

2. Go lint in CI.

## Stack

<!-- step: branch, PR number (base) -->

## Log

- Step 0 (environment check): `make precommit` green on a clean tree. `make test-e2e`
  fails out of the box (Playwright wants chromium_headless_shell-1243, not installed;
  `/opt/pw-browsers` has 1194). Workaround, nothing committed: a directory outside the
  repo with `chromium_headless_shell-1243/chrome-headless-shell-linux64/chrome-headless-shell`
  symlinked to `/opt/pw-browsers/chromium_headless_shell-1194/chrome-linux/headless_shell`
  plus an `INSTALLATION_COMPLETE` file, then `PLAYWRIGHT_BROWSERS_PATH=<dir> make test-e2e`.
  All 28 E2E tests pass this way.
- Step 1 (SQLite pragmas): `InitDB` now opens both connections with
  `journal_mode(WAL)`, `busy_timeout(5000)`, `synchronous(NORMAL)`; the GORM (app)
  connection also has `foreign_keys(1)`. Set through the driver's `_pragma` DSN
  parameter so every pooled connection gets them. New test
  `backend/database/pragmas_test.go`. `make precommit` and E2E (28) green with foreign
  keys on.

## Decisions

- Step 1: foreign keys are enabled only on the app connection, not on the one that runs
  migrations, because migrations may recreate tables and enforcement would cascade or
  fail. The migration connection is now closed after migrating (it was leaked before).

## For review

- Step 1: SQLite now runs in WAL mode, so the database directory holds `-wal` and
  `-shm` files next to the `.db` file. The data directory must not be on a network
  filesystem, and file-copy backups must include them (or use `VACUUM INTO`, which step
  10 uses). To mention in the step 10/11 docs.
- Step 1: foreign keys are enforced from now on. On start, `InitDB` logs a warning per
  `table -> parent` with the number of orphan rows an existing database holds.

## Questions

## Blocked
