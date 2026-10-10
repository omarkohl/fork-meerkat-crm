# Progress

## Next step

8. ICS feed export.

## Stack

<!-- step: branch, PR number (base) -->
- 1: `first-sprint/01-sqlite-pragmas`, PR #4 (base `claude/workflow-clarification-kb7ca1`)
- 2: `first-sprint/02-golangci-lint`, PR #5 (base `first-sprint/01-sqlite-pragmas`)
- 3: `first-sprint/03-frontend-ci-gates`, PR #6 (base `first-sprint/02-golangci-lint`)
- 4: `first-sprint/04-dependabot-govulncheck`, PR #7 (base `first-sprint/03-frontend-ci-gates`)
- 5: `first-sprint/05-replace-nfnt-resize`, PR #8 (base `first-sprint/04-dependabot-govulncheck`)
- 6: `first-sprint/06-jwt-v5`, PR #9 (base `first-sprint/05-replace-nfnt-resize`)
- 7: `first-sprint/07-gocron-v2`, PR pending (base `first-sprint/06-jwt-v5`)

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
- Step 2 (Go lint): `golangci-lint run` reports 0 issues; a probe with an unchecked
  `os.Open` was flagged. `make precommit` green.
- Step 3 (frontend CI gates): `.github/workflows/lint.yml` gained `frontend` (ESLint
  `--max-warnings=0`, `tsc --noEmit`) and `format` (gofmt + Prettier, changed files only)
  jobs. Prettier 3 added as a devDependency with `frontend/.prettierrc.json`
  (single quotes, width 120); `make format`/`format-check` now cover frontend files
  changed since BASE. Verified a badly formatted new file fails `make format-check`.
  `make precommit` green.
- Step 4 (Dependabot, govulncheck): `.github/dependabot.yml` now covers gomod, npm
  (`/frontend`, also reads yarn.lock), github-actions, docker (`/`, `/backend`,
  `/frontend`) and bundler (`/docs`), each weekly with minor/patch grouped. New
  `.github/workflows/security.yml` (push to main, PRs, weekly, manual) with `govulncheck`
  and `yarn audit`. YAML parses. govulncheck could not run here (the sandbox proxy blocks
  vuln.go.dev), CI is the first real run. `react-scripts` moved to devDependencies;
  `yarn audit --groups dependencies --level high` then reports 0 (it was 2 critical and
  22 high, all via react-scripts). `yarn.lock` unchanged, `--frozen-lockfile` passes.
- Step 5 (replace nfnt/resize): characterisation tests first
  (`controllers/photo_controller_test.go`, `carddav/photo_test.go`: photo 400x400 when
  larger, unchanged when small, landscape/portrait cropped to the short side, thumbnail
  48x48 JPEG data URL); they passed on nfnt, then again after the swap. Both call sites
  use a small `resizeTo` helper over `draw.CatmullRom.Scale` (`draw.Src`); tests also check the centre pixel colour (±10); `go mod tidy` dropped
  nfnt/resize. `golangci-lint` 0 issues, `go test ./...` green.
- Step 6 (golang-jwt v5): new `middleware/auth_jwt_test.go` (valid, expired, wrong
  secret, tampered signature, alg none, non-HMAC alg, malformed, missing `user_id`) passed
  on v4, then on v5.3.1; the tightening tests (no/string `exp`, HS384, future `iat`) were
  red first. Only the import path changed in `middleware/auth.go`,
  `services/user_service.go` and its test; `jwt.Parse`, `MapClaims`, `ErrTokenExpired`,
  `ErrTokenMalformed`, `ErrSignatureInvalid` are the same in v5. `go mod tidy`,
  `golangci-lint` (0 issues), `make precommit` and `make test-e2e` (28, login included)
  green.
- Step 7 (gocron v2): scheduling lived inline in `main.go` and was untestable, so it
  moved first (on v1) into `services/scheduler.go` (`NewScheduler`, `Start`, `Stop`,
  `RunAll`, `NextRuns`; library types stay inside). `services/scheduler_test.go` pins:
  four named jobs, reminders daily at `REMINDER_TIME` in `REMINDER_TIMEZONE`, 5 min / N h
  intervals, interval jobs run once at start while the reminder job does not, `RunAll`,
  clean `Stop`. Green on v1 (also with `-race`), then migrated to v2.22.0 with the tests
  unchanged apart from the constructor now returning an error. `main.go` shrank.
  `golangci-lint` 0 issues, `make precommit` and `make test-e2e` (28) green.

## Decisions

- Step 7: v1 ran interval jobs immediately at start (found by the characterisation
  test); v2's `DurationJob` waits a full interval by default, so `WithStartImmediately`
  is set on those three jobs to keep behaviour. main still also fires the reminder,
  calendar and contact tasks once at startup, as before (rate-limited services). The
  `HH:MM` reminder time is parsed with `Sscanf` (config already validates it). No
  singleton mode, as in v1. `Stop` now uses `Shutdown` with a 5 s stop timeout (v2's default is 10 s, as long as Docker's grace period while HTTP is still serving); a timeout is logged as a warning. `RunAll`/`NextRuns` are test-only helpers (`services/scheduler_export_test.go`).

- Step 6: the only issuer (`services/user_service.go`) signs HS256 and always sets `exp`,
  so the parser is tightened with `WithValidMethods(["HS256"])` and
  `WithExpirationRequired()` (the HMAC key-func check stays). `WithIssuedAt()` restores
  v4's rejection of a future `iat`, which v5 only applies when asked. Rejected tokens all
  answer 401 with "Invalid token" except expired, malformed and bad signature, which keep
  their specific messages; tests pin these, including no `exp`, string `exp`, HS384, future
  `iat`. Existing sessions are unaffected (their tokens are HS256 with `exp`).

- Step 5: `x/image/draw` has no Lanczos kernel, so CatmullRom (bicubic) is used; output
  sizes are identical, pixels differ slightly. The helper is duplicated in `controllers`
  and `carddav` (5 lines) rather than adding a shared package, since both already
  duplicate the crop/save logic.

- Step 4: both security jobs block. govulncheck is pinned `@v1.8.0` and run via `go run`
  (reports only reachable vulnerabilities; v1.1.4 failed locally with "requires newer Go
  version"). The yarn audit covers production dependencies at high severity or above;
  `react-scripts` (build tooling) moved to devDependencies to get a green baseline. The
  Dockerfiles run a full `yarn install --frozen-lockfile` (no `--production`), so builds
  are unaffected.

- Step 3: the `format` CI job takes the base from the pull request's base SHA (the
  previous branch for stacked PRs) or the push's `before` SHA, with full history. The
  `pull_request` triggers of `unit-tests.yml` and `e2e-tests.yml` lost their
  `branches: [main]` filter too, so stacked PRs run all checks. ESLint and tsc already
  existed in `make lint`/`typecheck`; CI now gates them.

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

- Step 3: 87 of 113 frontend TS files do not pass the Prettier config (single quotes,
  width 120), so touching one forces a whole-file reformat in that change. The config
  is kept as is; the alternative is a one-off repo-wide reformat commit, which the plan
  rules out.

- Step 1: SQLite now runs in WAL mode, so the database directory holds `-wal` and
  `-shm` files next to the `.db` file. The data directory must not be on a network
  filesystem, and file-copy backups must include them (or use `VACUUM INTO`, which step
  10 uses). To mention in the step 10/11 docs.
- Step 1: foreign keys are enforced from now on. On start, `InitDB` logs a warning per
  `table -> parent` with the number of orphan rows an existing database holds.

## Questions

- The startup goroutines in `main.go` (`go reminderTask()` etc.) are not tracked during
  shutdown, so the database can close under a running startup sync; pre-existing, not
  covered by the scheduler's stop timeout.
- `middleware/auth.go` still accepts `int`/`uint` `user_id` claim types that a JSON-decoded
  token can never contain (only `float64`); candidate for simplification, left alone.
- Most Dockerfile base image tags float (`golang:alpine`, `node:lts-alpine`,
  `alpine:latest`, `nginxinc/nginx-unprivileged:alpine`), so Dependabot's docker
  ecosystem only sees the few pinned ones (`node:22-alpine`, `alpine:3.20`). Pinning was
  left alone on purpose.
- Photo directories are created with `os.ModePerm` (0777) in `controllers/photo_controller.go`
  and `carddav/vcard_mapper.go` (gosec G301, suppressed for now); should the mode be tightened?
- `middleware/auth.go` and `relationship_controller.go` convert a JWT float64 claim and an
  `Atoi` URL param to `uint` without range checks (G115, suppressed); harmless today
  (signed token, `user_id`-scoped lookup).
- Lint baseline: `Close` errors on files opened for writing (`controllers/photo_controller.go`,
  `carddav/vcard_mapper.go`) are ignored; should they be checked?

## Blocked
