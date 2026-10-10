# Progress

## Next step

9.

## Stack

- 1: second-sprint/01-ci-fixes (PR #16, base first-sprint/12-finish)
- 2: second-sprint/02-deleted-users (PR #17)
- 3: second-sprint/03-trash-followups (PR #18)
- 4: second-sprint/04-file-robustness (PR #19)
- 5: second-sprint/05-ics-last-fetched (PR #20)
- 6: second-sprint/06-api-reference (PR #21)
- 7: second-sprint/07-supply-chain (PR #22)

## Log

- 0/1: `make precommit` and `make test-e2e` green on the first-sprint stack (CI of PR #15 green except
  govulncheck). Step 1: `x/net` v0.58.0 -> v0.60.0 (four HTTP/2 advisories, fixed version needs Go 1.26,
  so go.mod now says 1.26.0), golangci-lint v2.5.0 -> v2.14.0 (older builds refuse a Go 1.26 module),
  two new findings fixed (gosec G706 in cmd/migrate, staticcheck QF1012). govulncheck cannot reach
  vuln.go.dev from the sandbox; CI is the check.

- 2: JWT and API-token auth now check that the user row exists (soft-deleted users get 401 at once);
  `DeleteUser` hard-deletes the user's API tokens (the ICS feed was already removed). New `idutil`
  package (`FromFloat`, `Parse`) replaces the claim type switch and the uint conversions of URL ids
  (admin users, trash, relationships); the G115 suppressions in auth and relationships are gone (the two
  in `scheduler.go` are validated HH:MM and belong to a later step). Tests: middleware (deleted, unknown,
  bad/missing user_id), API token of deleted user, DeleteUser removes tokens/feed and leaves other users' alone.
  Review fixes: DeleteUser also hard-deletes CardDAV connection and links, calendar subscriptions and
  links, webhooks and deliveries; scheduled syncs skip rows of deleted users; `database/migrate.go`
  G115 fixed with a range check; remaining URL ids use `idutil.Parse`; auth user lookup answers 500 on a
  DB error (401 only when the user is gone). The G115 suppressions in `scheduler.go` remain (validated HH:MM).
- 3: migration 25 adds `carddav_contact_links.purged_at` (backfilled for links whose contact row is
  already gone); purge sets it. Sync: a purged link is a tombstone (pull-only: skip, keep; push modes: delete
  the remote object and drop the link, instead of re-importing it as before); a link whose contact row
  vanished without a purge is recreated from the server when the remote card changed (an unchanged remote
  card in a pushing mode is still deleted remotely, because the local-gone pass ignores `purged_at`; kept
  as is, it is the existing behaviour for a row that disappeared). New `contact.restored` webhook event
  (API validation, UI list, five locales, docs). Tests for all of it.
- CI note: the Go jobs use `go-version: '1.26'` with `check-latest` (fix for the govulncheck stdlib findings,
  commit on branch 01, merged up).
- 4: new `photofile` package (`EnsureDir` 0750, `SaveJPEG` with 0640, returns the `Close` error and removes a
  partial file) replaces the duplicated photo writing in `controllers/photo_controller.go` and
  `carddav/vcard_mapper.go`; four gosec suppressions gone (one G304 remains, in `photofile`). New
  `services.Background` runs the four startup tasks under one context and a WaitGroup; `main.go` waits for
  it (5 s) after the HTTP server stops and before the DB closes. Tests for modes, Close/partial-file
  handling and Background; docs note on directory modes. `SaveJPEG` uses O_EXCL (names are UUIDs, nothing
  overwrites) and fsyncs before close; the thumbnail is built before the photo file is written.
- 5: migration 26 adds `ics_feeds.last_fetched_at` (NULL = never). `ServeIcsFeed` stamps it with one conditional
  `UPDATE ... WHERE last_fetched_at IS NULL OR last_fetched_at < now-1min` (no race, at most one write a minute),
  after the token and owner checks and a successful build; so 200 and 304 both count, 404s and 500s do not. A failed
  write is logged and ignored. `GET/POST /ics-feed` return `last_fetched_at`; the Integrations card shows
  "Last fetched: <date and time>." or "Not fetched yet." (five locales). Tests: migration up/down/up, throttle,
  304, 404, per-user, reset on regenerate; Vitest; e2e. The e2e run needed `PLAYWRIGHT_BROWSERS_PATH` with
  symlinks to `/opt/pw-browsers` (no config change).
- 6: `docs/api-reference.md` now covers every route: OIDC, enabled-contact-fields, Monica import, webhooks,
  calendar subscriptions, CardDAV connection (28 routes were missing). New `routes/api_reference_test.go` registers
  the real routes (`RegisterRoutes` with OIDC and CardDAV switched on), reads the doc tables and fails in both
  directions (undocumented route, documented route that does not exist); parameter names are ignored and
  `<token>.ics` stands for `:file`. The `/api-tokens` path in the doc was already correct (nothing to fix).
- 7: all 16 distinct actions in the six workflows are pinned by full SHA with a `# vX.Y.Z` comment (resolved with
  `git ls-remote`, annotated tags dereferenced; latest release of the major each used). `docker/build-push-action`
  is v6.19.2 everywhere (publish was v5). Publish now attaches SBOM and `provenance: mode=max`. New
  `docker-build.yml`: builds the three images without pushing and scans each with Trivy (HIGH/CRITICAL, fixable only,
  `.trivyignore` for exceptions) on PRs touching Docker/backend/frontend, on main and weekly. `docs.yml` builds the
  site on PRs touching `docs/**` (no deploy; permissions moved to the jobs). Base images: `golang:1.26-alpine`,
  `node:22-alpine` (frontend was `node:lts-alpine`), `alpine:3.22` (was `latest` and `3.20`, EOL),
  `nginx-unprivileged:1.30-alpine`. actionlint is clean. Contributing docs updated.
  Review fixes: the `docs.yml` edit of the first commit had silently not applied (trailing whitespace in the
  original header), so the PR trigger, concurrency group and job permissions are now really in place; the docs
  concurrency group is per PR (`pages-<ref>`) or `pages-deploy`; build job has `pages: read`; nginx base is
  `1.30-alpine` (1.29 is stale; existence not checkable from here, the proxy blocks Docker Hub); `.dockerignore`
  added to the build paths; publish cache scoped per image.
- 8: Unit Tests workflow: Go runs with `-race -coverprofile` (passes locally; cgo is needed for `-race`, the runner
  has gcc, the SQLite driver stays pure Go), Vitest with the new `test:coverage` script
  (`@vitest/coverage-v8` 3.2.7, same as vitest; `yarn.lock` updated). Totals go to `$GITHUB_STEP_SUMMARY`, reports
  are uploaded as artifacts (`go-coverage`, `frontend-coverage`). Local totals: Go 51.6%, frontend 17.1% lines.
  New `TestMigrationsRoundTripOnSeededDatabase`: all up, seed 15 tables, all down (schema empty), all up, schema
  compared (columns, fks, indexes): it passed at once, so no down migration needed fixing. New CI job
  `CardDAV sync (Radicale)` runs `TestContactSync` with `MEERKAT_CARDDAV_IT=1`. Docs: `docs/development/testing.md`.

## Decisions

- Step 5: the stamp is written synchronously (a single indexed UPDATE), not in a goroutine like API-token
  `last_used_at`, so tests are deterministic and no write can outlive the DB at shutdown. Regenerating replaces
  the row, so the new URL starts with no fetch time. The card does not poll: it shows the value at page load.
- Step 4: the startup tasks are not interruptible (they take no context); the context only stops tasks
  that have not begun. Shutdown therefore waits up to 5 s and logs a warning if one is still running.
  Making the sync and reminder jobs context-aware is a larger change and left for later.
- There is no admin "disable user" feature, so only deletion is covered. One extra `COUNT` query per
  authenticated request (primary key lookup; fine for SQLite).
- Claim `user_id` is accepted only as a whole number float64 in 1..MaxUint32 (what JSON decoding of
  our own tokens yields); other types are refused as invalid tokens.
- Step 3: switching a connection from pull-only to a pushing direction makes the next sync delete the
  remote cards of purged contacts (same as for any locally deleted contact in those modes). The
  `contact.restored` payload is the contact, like `contact.created`.

## For review

- Step 8: Radicale runs via `docker run` after checkout, not as a `services:` container, because it needs `docker/radicale` mounted as config (service containers start before checkout). Image `tomsquest/docker-radicale:3.5.4.0` (also pinned in `docker-compose.carddav-test.yml`, was `latest`): the tag could not be checked against Docker Hub from here, and the job has never run. Fix the tag if the first CI run says it does not exist.
- Step 8: the frontend coverage is low (17% of statements, only 9 test files; the UI is covered by Playwright). No threshold is enforced, numbers are informational.
- Step 7: base image tags (`golang:1.26-alpine`, `alpine:3.22`, `nginx-unprivileged:1.30-alpine`) and the Trivy/docs/Docker workflows could not be tested here (no registry access, docker daemon unusable). The first PR run of `Docker build and scan` is the check: if a tag does not exist or Trivy finds fixable HIGH/CRITICAL issues in a base image, adjust the tag. Tags are minor-level, not digest-pinned (Dependabot keeps them current).
- Step 7: the Trivy job is a required-looking check on every Docker-related PR and weekly; a new advisory in Alpine/nginx can turn it red without any code change.
- Step 6: the test allowlist has two entries, both for the CardDAV server (`/carddav/*path`, `/.well-known/carddav`): WebDAV for CardDAV clients, documented in `docs/carddav.md`, not REST. Response shapes in the new doc sections are summarised from the handlers, not exhaustive.
- Step 2: every authenticated request now does a user lookup; confirm that cost is acceptable.
- Step 3: `WebhooksSettings.tsx` was not Prettier-clean, so the format check forced a full reformat of the file (the real change is one line, `contact.restored`).
- Step 3: the sync never writes `purged_at` back (`saveLink` and the link saves omit it), so a purge during a running sync keeps its tombstone.

- Step 5: `IcsFeedSettings.test.tsx` and `e2e/ics-feed.spec.ts` were reformatted by Prettier (small diffs only).

## Questions

## Blocked
