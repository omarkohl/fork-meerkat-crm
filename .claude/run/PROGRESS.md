# Progress

## Next step

3. Frontend lint/format/type gates in CI.

## Stack

<!-- step: branch, PR number (base) -->
- 1: `first-sprint/01-sqlite-pragmas`, PR #4 (base `claude/workflow-clarification-kb7ca1`)
- 2: `first-sprint/02-golangci-lint`, PR pending (base `first-sprint/01-sqlite-pragmas`)

## Log

- Step 2 (Go lint): `golangci-lint run` reports 0 issues; a probe with an unchecked
  `os.Open` was flagged. `make precommit` green.

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

- Step 2: golangci-lint v2.5.0 baseline in `backend/.golangci.yml`, run by a new
  `.github/workflows/lint.yml`. Not added to `make precommit` (needs the binary). The workflow's `pull_request`
  trigger has no branch filter so stacked PRs (base not `main`) are linted too.
  Fixed real findings: SMTP implicit TLS now sets `MinVersion` TLS 1.2, scheduler
  `Do` errors are logged, validator registration errors panic at init, `VACUUM` in the
  migration driver used `Query` (leaked rows) and now uses `Exec`. Excluded with
  comments: revive doc-comment/unused-parameter/var-naming rules, staticcheck
  style checks, gosec G101/G104 (global; the known G115/G201/G202/G301/G304 hits carry
  line-level `//nolint:gosec` with a reason), errcheck on Close/Seek/Remove,
  and errcheck/gosec in tests. Write-path `Close` errors (photo and vcard output files)
  are therefore still unchecked; see Questions.

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

- Photo directories are created with `os.ModePerm` (0777) in `controllers/photo_controller.go`
  and `carddav/vcard_mapper.go` (gosec G301, suppressed for now); should the mode be tightened?
- `middleware/auth.go` and `relationship_controller.go` convert a JWT float64 claim and an
  `Atoi` URL param to `uint` without range checks (G115, suppressed); harmless today
  (signed token, `user_id`-scoped lookup).
- Lint baseline: `Close` errors on files opened for writing (`controllers/photo_controller.go`,
  `carddav/vcard_mapper.go`) are ignored; should they be checked?

## Blocked
