# Progress

## Next step

12. Finish.

## Stack

<!-- step: branch, PR number (base) -->
- 1: `first-sprint/01-sqlite-pragmas`, PR #4 (base `claude/workflow-clarification-kb7ca1`)
- 2: `first-sprint/02-golangci-lint`, PR #5 (base `first-sprint/01-sqlite-pragmas`)
- 3: `first-sprint/03-frontend-ci-gates`, PR #6 (base `first-sprint/02-golangci-lint`)
- 4: `first-sprint/04-dependabot-govulncheck`, PR #7 (base `first-sprint/03-frontend-ci-gates`)
- 5: `first-sprint/05-replace-nfnt-resize`, PR #8 (base `first-sprint/04-dependabot-govulncheck`)
- 6: `first-sprint/06-jwt-v5`, PR #9 (base `first-sprint/05-replace-nfnt-resize`)
- 7: `first-sprint/07-gocron-v2`, PR #10 (base `first-sprint/06-jwt-v5`)
- 8: `first-sprint/08-ics-feed`, PR #11 (base `first-sprint/07-gocron-v2`)
- 9: `first-sprint/09-trash`, PR #12 (base `first-sprint/08-ics-feed`)
- 10: `first-sprint/10-backups`, PR pending (base `first-sprint/09-trash`)

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
- Step 8 (ICS feed): tests first (service: escaping, 75-octet folding on rune boundaries,
  RRULE mapping, scoping; controller: valid/invalid/regenerated/revoked token, tampered
  token, isolation, header injection; migration up/down + FK cascade), then
  `ics_feeds` (migration 000024), `services/ics_service.go`, `controllers/ics_feed_controller.go`,
  public `GET /api/v1/ics/:file` plus `GET/POST/DELETE /api/v1/ics-feed`, `IcsFeedSettings`
  card on the Integrations page, i18n (5 frontend + 5 backend locales), docs. Vitest
  (4 component tests), new `e2e/ics-feed.spec.ts` and an isolation case. `make precommit`,
  `golangci-lint` (0 issues) and `make test-e2e` (30) green. Prettier reformatted
  `ApiTokensPage.tsx` and `isolation.spec.ts` in full (see step 3 note).
- Step 8 review fixes (amended): the token is no longer logged: `LoggingMiddleware` logs
  the route pattern for the ICS route and gin's access logger skips it (`gin.Default()`
  became `gin.New()` plus `LoggerWithConfig{Skip}` and `Recovery`); test captures the log.
  RRULEs no longer skip short months or non-leap Feb 29 (verified with python-dateutil).
  `DeleteUser` removes the feed; `ServeIcsFeed` 404s when the owner is gone; concurrent
  create retries once on the unique constraint; reminder dates use `RemindAt.UTC()`;
  the copy timeout is cleared on unmount; docs note proxy logs and recurrence drift.

- Step 9 (Trash): investigation first. `DeleteContact` already soft-deleted the contact,
  its notes, reminders and relationships in one transaction, but hard-deleted the
  `activity_contacts` rows and the photo files, so a restore would have lost both; and
  nothing ever purged. Tests first (service, controller, scheduler, CardDAV sync), then
  `services/trash_service.go` (list, restore, purge, expired purge), `controllers/trash_controller.go`,
  routes `GET/DELETE /trash/contacts`, `POST /trash/contacts/:id/restore`, `DELETE /trash/contacts/:id`,
  daily `trash-purge` scheduler job (03:30 in the reminder timezone, not run at start; main also runs it once
  at startup), `TRASH_RETENTION_DAYS` (default 30, below 1 falls back to 30), `api/trash.ts`,
  `useTrash`, `TrashPage` + sidebar entry, i18n (5 locales; the delete confirmation text no longer
  says "cannot be undone"), docs. No migration (`deleted_at` is already indexed on all tables). Three raw SQL
  queries ignored `deleted_at` and were fixed: `GetCircles`, the activity search join on contacts and the
  relationship-birthday join. Vitest (page 7, hook 2), `e2e/trash.spec.ts` (3) and an isolation case.
  `make precommit`, `golangci-lint` (0 issues) and `make test-e2e` (34) green. Prettier reformatted
  `App.tsx` in full (see step 3 note).
- Step 9 review fixes (amended): purge re-selects inside its transaction
  (`id IN ? AND user_id = ? AND deleted_at IS NOT NULL`, same `deleted_at` as listed) and purges/counts/removes photos
  only for that set, so a concurrent restore (or restore plus new delete) keeps its dependants and photo files
  (tests with a stale listing). New `models.TrashContact(tx, userID, id)` is the one soft-delete used by the
  controller, the built-in CardDAV `DeleteAddressObject` and the sync's remote deletion (reminders no longer
  stay live there); sync's edit-beats-delete now reuses the restore logic (dependants come back). Pull-only sync
  skips a link whose contact was purged instead of re-importing it. Purge selects only the needed columns;
  `purgedOn` reads as future tense in all locales; docs note the pre-trash contacts caveat. Tests for each.

- Step 10 (built-in backups): tests first (service: `VACUUM INTO` snapshot of a file DB in WAL mode with rows
  that live only in the `-wal` file, both users' data, photos tarball incl. symlink skip and backup dir inside
  the photo dir, 0700/0600, same-second names, retention pruning, strict name validation; controller: non-admin
  403 on all four endpoints, create/list/download/delete, retention, path traversal; scheduler: job optional,
  daily at `BACKUP_TIME`, not run at start, `RunAll`). Then `services/backup_service.go`,
  `controllers/backup_controller.go`, admin routes `GET/POST /admin/backups`, `GET/DELETE /admin/backups/:name`,
  config (`BACKUP_ENABLED`, `BACKUP_TIME`, `BACKUP_RETENTION_COUNT`, `BACKUP_DIR`), `backup` scheduler job wired in
  `main.go`, `api/backups.ts`, `BackupsPage` (Settings > Backups, admins only), i18n (5 locales; no backend
  strings needed), docs (deployment: WAL files note and restore procedure; api-reference; env tables; .env
  examples). Vitest (7), `e2e/backups.spec.ts` (2), isolation case. `make precommit`, `golangci-lint` (0 issues)
  and `make test-e2e` (37) green.
- Step 10 review fixes (amended): create and download lift the server write deadline for their request
  (`http.NewResponseController(...).SetWriteDeadline(time.Time{})`; tested with a recording writer), docs
  updated. Stale `.tmp-*` dirs in `BACKUP_DIR` are removed at the start of each backup (under the mutex). An
  existing `BACKUP_DIR` is chmod-ed to 0700. The archive is `Sync`ed before rename. Real-route test
  (`routes/backup_routes_test.go`) that an admin's API token gets 403 on all four routes. `BACKUP_TIME` is
  validated only when `BACKUP_ENABLED`. Docs: `JWT_SECRET_KEY`/`.env` must be kept with backups (CardDAV/CalDAV
  credentials are encrypted with it), restore steps name the host mount directories, browser download buffers
  the archive in memory (documented, not changed).
- Step 11 (docs): audited README, README-developer, `docs/` (getting-started env table, settings, deployment, data-model,
  api-reference, contributing) against steps 1-10: WAL files, ICS feed, Trash, backups, `BACKUP_*`/`TRASH_RETENTION_DAYS`,
  lint/CI/security gates were already documented by their steps, so no doc text changed. `make precommit` green;
  `make test-e2e` ran without the Playwright workaround on this machine (browsers installed): 54 passed.

## Decisions

- Step 10: enabled by default (`BACKUP_ENABLED=true`, daily 02:30 in the reminder timezone, keep 7): the point of
  the item is that most self-hosters have no backup, and an opt-in switch would leave them without one. It costs
  disk (up to 7 copies of database plus photos); `BACKUP_ENABLED=false` turns it off. Manual create, list,
  download and delete work regardless. A backup is one `meerkat-backup-YYYYMMDD-HHMMSS.tar.gz` (UTC stamp; a
  `-N` suffix for several in one second) holding `meerkat.db` and `photos/...`, written to a temp dir inside
  `BACKUP_DIR` and renamed into place, so a failed run leaves no partial backup. `BACKUP_DIR` defaults to
  `<dir of SQLITE_DB_PATH>/backups` (on the data volume in Docker). Manual create also prunes to the retention
  count. The snapshot is `VACUUM INTO` through the app's own connection (never a file copy). Only regular
  photo files are archived (symlinks skipped, backup dir skipped when inside the photo dir). The file name in
  the URL is matched against `^meerkat-backup-\d{8}-\d{6}(-\d{1,3})?\.tar\.gz$` before touching the file
  system; anything else answers 404 (not 400, to reveal nothing). The endpoints sit in the existing admin group
  (`AdminMiddleware` refuses API tokens too). Logs carry the backup name and size only. The admin UI is a
  separate page `/backups` in the Settings submenu next to Users, not a section of the Users page. The list
  response includes the server directory path (admins only) so they know what to copy. Create and download run
  inside the request with the write deadline lifted, so `HTTP_WRITE_TIMEOUT` does not bound them. No
  restore endpoint: restoring replaces the live database, which stays a documented manual procedure.

- Step 9 review: (a) `TrashContact` lives in `models`, not `services`, because `services` imports the
  `carddav` package, which needs it (import cycle). (b) Pull-only tombstone: when a link points at a missing
  contact row and the connection cannot push, the sync adopts the remote etag and skips (no re-import); the
  kept link is the marker, so purge needs no extra row and startup purge is safe. Two-way/push keep the old
  behaviour (a remote edit after a purge recreates the contact, i.e. edit beats delete; without an edit the
  remote card is deleted). (c) The purge confirms a candidate only if its `deleted_at` is unchanged since the
  listing, so a restore followed by a fresh deletion is not purged early.

- Step 9: scope is contacts only (the only soft-deleted thing a user can see and expect to undo;
  notes, reminders, relationships and activities deleted on their own stay deleted). Delete keeps the
  contact's dependants soft-deleted (as before), now also keeps `activity_contacts` rows and photo files; these
  are the "dependants kept while in trash". Restore revives the contact and the notes, reminders and
  relationships whose `deleted_at` is within 10 s of the contact's (compared in Go; works for contacts
  deleted before this change and does not revive a note the user deleted earlier). Activity joins hide the
  trashed contact because GORM scopes preloads; no schema change. Purge (single, empty, auto) runs in a
  transaction per 200 contacts: it hard-deletes activity links, notes, reminders, reminder completions,
  relationships (rows of the contact), clears `related_contact_id` on other contacts' relationships that point
  at it (foreign keys are enforced and have no cascade there), then the contact; photo files are removed after
  the commit. Only rows with `deleted_at` set and the user's `user_id` can be purged, so a live contact is never
  deletable through the trash API. Auto purge works on every user's trash, so contacts of soft-deleted users are
  eventually removed too. Restore cannot hit a unique conflict: `(user_id, vcard_uid)` is unique over soft-deleted
  rows too, so the row holds its own value. CardDAV: restoring before the deletion is synced changes nothing;
  restoring after it was pushed re-uploads the card under the same UID (no duplicate); purge keeps the
  `carddav_contact_links` row, because a link without a contact is what makes the next sync delete the remote
  object (tested), and the sync drops the link itself. Tests pin all three. Endpoints take no body, so no
  `ValidateJSONMiddleware`. The delete webhook `contact.deleted` still fires on delete; no restore/purge events.
  The trash list is not paginated (trashes are small and bounded by retention). Nav entry "Trash" sits in the
  main sidebar below Network. Photo directory for purge: `cfg.ProfilePhotoDir`, falling back to `PROFILE_PHOTO_DIR`.
  No backend locale strings were needed (API errors are English like the rest).

- Step 8 review: RRULE for day 29-31 starts is `BYMONTHDAY=28..D;BYSETPOS=-1` (D=31 gives
  the last day), which mirrors the app's clamp-to-last-day but not its drift (the app adds
  months to the clamped date, so Jan 31 -> Feb 28 -> Mar 28, the calendar goes back to
  Mar 31). Feb 29 (birthday or yearly reminder) uses `BYMONTH=2;BYMONTHDAY=-1`. Reminder
  dates are the UTC date of `remind_at`, matching `CalculateNextReminderTime`; a reminder
  created at local midnight east of UTC therefore shows the previous day.
- Step 8: one feed per user (table `ics_feeds`, unique `user_id`, SHA-256 of a 32-byte
  base64url token, same scheme as API tokens). Regenerate = replace the row in a
  transaction, revoke = delete the row (no soft delete; hard delete keeps the unique
  constraints simple). Public route `/api/v1/ics/:file` is registered outside the
  protected group with only the per-IP API rate limiter; the handler ignores any
  session user, answers 404 for every failure (bad format, unknown, revoked), sets
  `Cache-Control: private, max-age=900`, `ETag` (304 on match), `noindex`,
  `Referrer-Policy: no-referrer`. The token format is checked before the DB lookup,
  and the stored hash is compared with `subtle.ConstantTimeCompare`. The management
  endpoints take no JSON body, so no `ValidateJSONMiddleware` (like `/reminders/:id/complete`).
  The server never builds an absolute URL (it does not know its public address
  behind a proxy); the frontend builds it from `API_BASE_URL` and `window.location.origin`.
  Content: non-archived contact birthdays (yearly RRULE; `--MM-DD` starts in 1972, a leap
  year after 1970) and non-completed reminders as all-day events, recurrences weekly,
  monthly, quarterly (`INTERVAL=3`), six-months (`INTERVAL=6`), yearly mapped to RRULE,
  `once` without. Relationship-only birthdays (no contact) and anniversaries are not
  exported. `DTSTAMP` is the row's `updated_at`, so output is deterministic and the
  ETag only changes with data. Event titles use the owner's language (`ics.*` keys in
  `backend/i18n/locales`).

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

- Step 10: backups are ON by default (daily, keep 7) and contain every user's data including password hashes;
  check you agree with the default and with the warning text on the Backups page. The Settings > Backups
  menu entry is shown to admins only. Restore is manual (docs/deployment.md), there is no restore button.
- Step 9 review: `App.tsx` stays Prettier-reformatted in full: `make format-check` checks every file changed since the base, so a route and nav entry in it cannot be a minimal diff.
- Step 9: UI is a top-level "Trash" page (sidebar, under Network) with one card listing deleted contacts
  (avatar, name, "Deleted <date> - Will be deleted permanently on <date>"), icon buttons Restore and Delete permanently
  per row, and a red "Empty trash" button in the card header; both destructive actions confirm in a dialog,
  restore does not. Dates use the user's date format. The contact delete confirmation now says the contact moves
  to the Trash (it used to say it cannot be undone). The delete itself changed behaviour: photos of a deleted
  contact stay on disk until purge.
- Step 8: UI is a "Calendar feed (ICS)" card on the Integrations page (`/api-tokens`),
  below Webhooks, not on the Settings page, because API tokens and webhooks live there.
  Generate shows the URL once in a dialog with a copy button (same pattern as API
  tokens); Regenerate and Revoke ask for confirmation. Existing users lose nothing; the
  feature is off until someone generates a URL. The URL is a bearer secret in a query-free
  path: it can land in reverse proxy access logs and calendar-provider caches (Google
  Calendar fetches it server side, so an instance behind a VPN will not work with it).
- Step 8: reminders are exported with the date part of `remind_at` (as stored, reminders
  are midnight in the creator's timezone), not as timed events, so they show as all-day.

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

- `README-developer.md` names `make migrate-status`, which does not exist (pre-existing, not touched).

- Step 10: `docker-compose.yml` does not list the new `BACKUP_*` variables (it passes the env file through like
  the other settings); worth confirming that a bind-mounted `./data` volume is writable by the container user,
  as the default backup directory lives there.

- CardDAV pull-only connections: a purged contact leaves its link behind (see Decisions). If the remote card is
  edited later, the sync's "link points at a vanished row" branch re-creates the contact from the remote, whereas
  a soft-deleted tombstone used to block that. It only happens after a purge (manual or 30 days) in pull-only mode.
  Should purge drop such links and remember the remote UID somewhere instead?
- No `contact.restored` / purge webhook events were added; the webhook event list is unchanged. Wanted?
- Deleting a user (admin) soft-deletes their contacts but leaves them to the 30-day auto purge, which now also
  removes their photo files; the user's `api_tokens` gap above is unchanged.
- E2E tests never use "Empty trash" on the seeded user (it would race with parallel specs); the empty-trash
  UI path is covered by Vitest and the API path by Go tests and the isolation spec.
- API tokens have the same gap as the ICS feed had: users are soft-deleted and `DeleteUser`
  does not remove their `api_tokens`, so a deleted user's API token may still authenticate
  (not fixed here).

- `docs/api-reference.md` lists the API token endpoints as `/admin/api-tokens`, but the
  routes are `/api-tokens` (pre-existing, not touched).
- The ICS feed has no "last fetched" indicator (the API-token list has `last_used_at`);
  not added to keep the step small. Worth it?

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
- `middleware/auth.go` converts an `int` JWT claim to `uint` without a range check (G115,
  suppressed); harmless today (signed token).
- Lint baseline: `Close` errors on files opened for writing (`controllers/photo_controller.go`,
  `carddav/vcard_mapper.go`) are ignored; should they be checked?

## Blocked
