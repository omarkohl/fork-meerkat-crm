# Progress

## Next step

12.

## Stack

- 1: second-sprint/01-ci-fixes (PR #16, base first-sprint/12-finish)
- 2: second-sprint/02-deleted-users (PR #17)
- 3: second-sprint/03-trash-followups (PR #18)
- 4: second-sprint/04-file-robustness (PR #19)
- 5: second-sprint/05-ics-last-fetched (PR #20)
- 6: second-sprint/06-api-reference (PR #21)
- 7: second-sprint/07-supply-chain (PR #22)
- 8: second-sprint/08-test-infra (PR #23)
- 9: second-sprint/09-fuzz (PR #24)
- 10: second-sprint/10-untested-logic (PR #25)
- 11: second-sprint/11-i18n-check (PR #26)

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
  Review fixes: the migration test now steps one migration at a time (down with row-count check and schema
  snapshot per version, then up comparing each version), plus a test that seeds at versions 12/20/23/24 and
  migrates up keeping the rows. It found that the down of migration 8 leaves three `contacts` columns
  (`etag`, `vcard_extra`, `vcard_uid`, documented SQLite limitation in that file); migrations are not edited and
  a new one cannot fix a down, so the test ignores those columns below version 8 (see Questions). The Radicale job
  runs all of `./services/` (not only `TestContactSync`), image `3.8.3.0`; `@vitest/coverage-v8` is `^3.2.4`
  like vitest; coverage config moved above the localStorage comment; `coverage.out` and `coverage.html` ignored.

- 9: Go fuzz targets: `FuzzVCardToContact` (carddav; decode, map, re-encode, decode again), `FuzzCSVImport`,
  `FuzzImportValueParsers`, `FuzzMonicaMapping` (JSON into every Monica type and mapper), `FuzzICSText`,
  `FuzzBuildICSFeed` (services). Each ran 1 to 2 minutes locally (hundreds of thousands of executions). No reachable bug; one
  hardening: `foldICSLine` never ended for 75+ UTF-8 continuation bytes (cut position fell to 0). Callers escape first
  (invalid bytes become U+FFFD), so it was not reachable; fixed with a regression test. A seventh target,
  `FuzzDecodePhoto`, covers the image decode of `SaveContactPhoto` (extracted as the pure `decodePhoto`); 60 s clean. The other finding was a wrong property of mine
  (`NormalizeBirthday` returns unknown formats unchanged, by design). New workflow `fuzz.yml`: nightly 03:17 UTC and
  manual (`fuzztime` input), one matrix job per target, 30 s default, `contents: read`, uploads untracked files under
  `testdata/fuzz` as `fuzz-failing-input-<Target>` on failure; actionlint clean. Docs: `docs/development/testing.md`.
  Separate commit before it: the flaky `IcsFeedSettings` Vitest run (react-transition-group timer after teardown, about 1 in 8)
  came from Testing Library's auto-cleanup not being active (Vitest has no globals); `afterEach(cleanup)` is now in `setupTests.ts`.

- 10: new tests for `birthday_service`, `webhook_service`, `credential_crypto`, `mailer`, OIDC service provisioning and
  controllers for oidc (a fake IdP with signed ID tokens: state, nonce, audience, expiry, cookies), photo, calendar, carddav,
  graph, import and admin users (non-admin refused on every route, own/last admin guards). Bugs found and fixed:
  1. Webhooks with `WEBHOOK_BLOCK_PRIVATE_URLS`: only the URL was checked, so a redirect or a DNS rebinding reached internal
     hosts, and `0.0.0.0` was allowed. The delivery client now checks the resolved address at dial time; unspecified addresses count.
  2. Mailer: `SendEmail` refuses a line break in recipient or subject (defence in depth: `net/smtp` already rejects CR/LF in RCPT and the subject is Q-encoded, so this was probably not exploitable).
  3. OIDC auto-provisioning failed with a unique-constraint error when the derived username belonged to a soft-deleted user.
  4. Photo upload: unsupported/corrupt/empty files answered 500 (now 400), and the decoders allocated from the header
     dimensions (a 10 MB file could ask for gigabytes): `photofile.CheckDimensions` (100 MP) guards the upload, `SaveContactPhoto`
     (CardDAV, VCF import) and so the fuzz target too.
  5. Graph: relationships and activities involving archived contacts produced edges (and activity nodes) pointing at nodes that
     are not drawn.
  6. CalDAV and CardDAV: changing the URL to another host kept the stored password, which the next sync sent to the new host.
     Now it is dropped unless the request carries a new one (same host keeps it).
  7. Import sessions answered 401 for another user's session id (the frontend treats 401 as an expired login and logs out, and it
     confirmed the id exists); now the same 404 as an unknown id (CSV/VCF and Monica).

- 11: `scripts/i18n-check.mjs` (plain Node, no dependencies; tests in `scripts/i18n-check.test.mjs` run with `node --test`) checks `frontend/src/i18n/locales` and `backend/i18n/locales`: a key missing in de/it/es/fr, a key not in en and non-string values are errors; duplicate keys (JSON.parse hides them, so there is a small parser) and keys no source seems to use are warnings. The existing locales had no gaps (0 errors); 27 frontend keys are reported as unused (warnings, left alone). `make i18n-check` is part of `make precommit` (about 0.2 s) and of a new `i18n` job in lint.yml; `lefthook.yml` is optional and documented in contributing.md. No UI strings changed, so no E2E run. Hardening after review: invalid JSON (trailing comma, truncation, BOM, empty file) is an error instead of hanging; plural families need at least `_other` in every locale (the backend has no plurals, so exact keys there); empty values and a different `{{placeholder}}` set than en are errors.

## Decisions

- Step 11: plural keys (`_one`, `_other`, `_many`, ...) are compared by base name, because languages differ in their plural forms. "Unused" means no string literal equals the key and no literal or template prefix (`'a.b.'`, `` `a.${x}` ``) covers it, so dynamic keys are tolerated; it is a heuristic and only warns.
- Step 10 review fixes: (a) webhook blocking client is one package-level client cloned from `http.DefaultTransport` using the shared `newPrivateBlockingDialContext`, which now tries every safe address; a mixed answer (public + private) connects only to the public one. A dial-time block is recorded failed without retry, like the pre-check. (b) `DeleteUser` rewrites the soft-deleted row's email (`deleted+<id>@deleted.invalid`) and username (`deleted-<id>`) and sets `oidc_subject`/`oidc_provider` to NULL (the partial unique index ignores NULL), so the person can log in again as a new account; the username count error in provisioning is no longer ignored. (c) `photofile.MaxPixels` is 100 MP (48/50 MP phones). The header check now fails closed for formats with a registered decoder (an unreadable JPEG/PNG/GIF/HEIC header is refused) and passes unregistered formats to the decoder. HEIC: valid photos still pass (tested with the decoder's `test8.heic`, brands heic and mif1); the `mif1` brand has no registered decoder, so the upload path calls `heic.DecodeConfig` directly and fails closed. (d) A server change with a stored password is a 400 (field `password`) instead of a silent drop; the UI hint says so in all five locales and the dialogs show a translated error. (e) One `sameServer` helper (scheme with webcal as https, lowercase host, default-normalised port) for calendars, CardDAV update and Discover.
- Step 9: the fuzz targets live next to the code as `*_fuzz_test.go` and run over their seeds in every `go test`;
  seed files in `testdata/fuzz` are a handful of hand-written edge cases. The workflow does not run on pull requests.
  Fuzzing does not cover the DB-backed `ParseVCF`/`GenerateCSVPreview` (they call the fuzzed parsers and mappers).
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

- Step 11: the 27 "unused" warnings (e.g. `activities.deleteConfirm`, `relationships.saveFailed`) are candidates for removal from all locales; not done here.
- Step 10: behaviour changes besides pure fixes: changing the CalDAV/CardDAV server (host, port or scheme) with a stored password answers 400 until a new password (or clear) is sent; another user's import session id is 404 instead of 401; photo uploads over 100 megapixels are refused.
- Step 9: the first scheduled run of `Fuzz` is the check of the workflow (matrix, artifact collection with `git ls-files --others`). A nightly red run means a new input crashed a parser: download the artifact, add it under `testdata/fuzz`, fix.
- Step 8: Radicale runs via `docker run` after checkout, not as a `services:` container, because it needs `docker/radicale` mounted as config (service containers start before checkout). Image `tomsquest/docker-radicale:3.8.3.0` (also pinned in `docker-compose.carddav-test.yml`, was `latest`): the tag and the compatibility of `docker/radicale/config` with Radicale 3.8 could not be checked from here, and the job has never run. Fix the tag if the first CI run says it does not exist.
- Step 8: the frontend coverage is low (17% of statements, only 9 test files; the UI is covered by Playwright). No threshold is enforced, numbers are informational.
- Step 7: base image tags (`golang:1.26-alpine`, `alpine:3.22`, `nginx-unprivileged:1.30-alpine`) and the Trivy/docs/Docker workflows could not be tested here (no registry access, docker daemon unusable). The first PR run of `Docker build and scan` is the check: if a tag does not exist or Trivy finds fixable HIGH/CRITICAL issues in a base image, adjust the tag. Tags are minor-level, not digest-pinned (Dependabot keeps them current).
- Step 7: the Trivy job is a required-looking check on every Docker-related PR and weekly; a new advisory in Alpine/nginx can turn it red without any code change.
- Step 6: the test allowlist has two entries, both for the CardDAV server (`/carddav/*path`, `/.well-known/carddav`): WebDAV for CardDAV clients, documented in `docs/carddav.md`, not REST. Response shapes in the new doc sections are summarised from the handlers, not exhaustive.
- Step 2: every authenticated request now does a user lookup; confirm that cost is acceptable.
- Step 3: `WebhooksSettings.tsx` was not Prettier-clean, so the format check forced a full reformat of the file (the real change is one line, `contact.restored`).
- Step 3: the sync never writes `purged_at` back (`saveLink` and the link saves omit it), so a purge during a running sync keeps its tombstone.

- Step 5: `IcsFeedSettings.test.tsx` and `e2e/ics-feed.spec.ts` were reformatted by Prettier (small diffs only).

## Questions

- Step 10: `FindOrProvisionUser` links a verified-email OIDC login to an existing account even if that account is already linked to a different subject (the link is overwritten). Left as is; consider refusing. Not tested/changed.
- Step 10: not covered in depth: the sync paths behind `SyncCardDAVConnection`/`SyncCalendarSubscription` (covered by the services tests) and `DiscoverCardDAVAddressBooks` (needs a CardDAV server).
- The down migration of 000008 leaves `contacts.etag`, `vcard_extra` and `vcard_uid` in place (it predates `DROP COLUMN`). Harmless, but a rollback below version 8 is not byte-identical to a fresh schema. Left as is; the migration test tolerates it.

## Blocked
