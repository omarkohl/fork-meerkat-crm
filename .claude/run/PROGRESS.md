# Progress

## Next step

18.

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
- 12: second-sprint/12-ssrf-switch (PR #27)
- 13: second-sprint/13-security-headers
- 14: second-sprint/14-sessions-audit (PR set by the orchestrator)
- 15: second-sprint/15-totp (PR set by the orchestrator)
- 16: second-sprint/16-fts-search (PR set by the orchestrator)
- 17: second-sprint/17-thumbnails (PR set by the orchestrator)

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

- 12: `BLOCK_PRIVATE_URLS`: when set to a valid boolean it decides for webhooks, CalDAV, CardDAV and Monica and overrides the four older flags; unset (or invalid, with a warning) leaves them as before; default off. The guard is now one package, `backend/netguard` (`IsBlockedIP`, `NewDialContext`): the copies in `services` (dav_http, webhook), `monica` and `httputil` are gone. It resolves, drops blocked addresses and dials the checked IP (each in turn). It now also blocks carrier-grade NAT (100.64.0.0/10), multicast and 255.255.255.255. CardDAV was already guarded (flag defaulted to CALDAV's); discovery and `/.well-known/carddav` share the service's transport, so they are covered (tested). `httputil.FetchImageFromURL` (photo URLs) stays always-on. Startup warning and `GET /admin/system-status` (admin only) + dismissible banner on the Users page (dismissal in localStorage). Tests: config precedence table, netguard, each path blocked/allowed, endpoint isolation.
  Review fixes: guarded transports come from `netguard.NewTransport` and never use `HTTP(S)_PROXY` (the webhook client cloned
  `DefaultTransport`, so a proxy would have been checked instead of the target; tested, documented). The resolver and dialer are
  seams in netguard: tests show a mixed answer dials only the public IP, one lookup per dial, and a rebinding sequence never reaches the
  private address. An invalid `BLOCK_PRIVATE_URLS` now fails closed (true, error logged); true/false/1/0/yes/no/on/off in any case.
  netguard uses `net/netip` prefixes and also blocks 0.0.0.0/8, 240.0.0.0/4, 198.18.0.0/15, 192.0.0.0/24, Teredo, local-use NAT64 and
  NAT64/6to4 addresses embedding a blocked IPv4. The banner is requested for admins only, its dismissal stores a signature of the
  situation (setting, features that allow private URLs, user-count bucket) so a changed situation shows it again;
  `/admin/system-status` gained `features`. Vitest for the banner and an e2e case (`users-banner.spec.ts`). Prettier did not force
  unrelated reformatting this time.

- 13: audit: the backend sent XFO, nosniff, Referrer-Policy and HSTS (COOKIE_SECURE only); both nginx configs already had an enforcing CSP, but copied four times each (every location repeats `add_header`), no Permissions-Policy/COOP/CORP, and the server-level `add_header` was also inherited by proxied API locations, so API responses got the backend's headers and nginx's SPA CSP (duplicates). The e2e server (`scripts/e2e-stack.mjs`) sent no headers at all, so the CSP had never been tested. Now: `frontend/security-headers.conf` is the single source (included per static location in `frontend/nginx.conf` and `docker/nginx.conf`, copied to `/etc/nginx/` by both Dockerfiles); proxied locations add nothing, so the backend's API headers (new: `default-src 'none'; frame-ancestors 'none'`, Permissions-Policy, COOP same-origin, CORP same-site) pass through once. CSP gained `manifest-src`, `worker-src`, `object-src 'none'`, `base-uri`, `form-action`. HSTS: nginx via `$forwarded_proto`/`$hsts_header` maps (empty on plain HTTP, so no header), backend when COOKIE_SECURE, TLS or `X-Forwarded-Proto: https`; nginx now forwards an incoming `X-Forwarded-Proto` instead of overwriting it with its own scheme. The e2e server applies the same snippet; `fixtures.ts` `test` fails any test on a CSP violation (console message or `securitypolicyviolation`); `security-headers.spec.ts` checks the SPA and API headers and that an inline script is blocked. `scripts/security-headers.mjs` + test (run by `make i18n-check`/precommit/CI lint job) verify both configs: no stray `add_header`, include in every static location, none in proxied ones, both Dockerfiles copy the snippet, the configs differ only in the upstream. Go test for the middleware. Docs: deployment.md "Security headers". CSP is ENFORCING: all 42 e2e tests pass under it with no violation (no inline scripts in the CRA build, service worker is unregistered, no external fonts/tiles/images).

- 14: migration 27: `users.token_version`, `sessions` (public_id = the JWT `sid` claim, user agent, IP, last_seen_at, expires_at, revoked_at), `audit_logs` (no foreign key; username copied). Logins (password and OIDC) go through `services.IssueSession`, tokens carry `sid` and `tv`. `AuthMiddleware` (one extra query for the token version, one for the session when `sid` is present) refuses revoked, expired and foreign sessions and a stale `tv`; the session is one read (`session.Find`, a leaf package shared with the services since services imports middleware) and `last_seen_at` is written only when the value read is more than a minute old, so most requests do not take SQLite's write lock. Tokens without `sid`/`tv` (issued before) still work at version 0 and die on "sign out everywhere". API: `GET /sessions`, `DELETE /sessions/:id`, `POST /sessions/revoke-all` (refused for API tokens), `GET /admin/audit-log` (event, username substring, pagination). Password change signs out the other browsers (and starts a new session for this one), password reset and an admin-set password sign out everywhere, logout revokes the session behind the cookie, admin delete removes the user's sessions. Audit events: login, login_failed, logout, password_changed/reset, session_revoked, sign_out_everywhere, export (csv/vcf), API token, ICS feed, backup create/download/delete, admin user update/delete, admin reminders. Failed login for an unknown account stores no identifier. `AUDIT_LOG_RETENTION_DAYS` (365, 0 = forever); the daily 03:30 trash task also purges ended sessions and old entries. UI: "Active sessions" card on the profile page, admin "Audit log" page (event and user filters, pagination), five locales. Tests: services (issue, revoke, touch throttle, purge), middleware (sid, tv, legacy), controllers (login/logout/revoke/isolation/sign-out-everywhere/password change/reset/delete/audit list), migration up/down, config, Vitest for both components, e2e `sessions.spec.ts`. `make precommit`, golangci-lint, `go test -race` and `make test-e2e` (46) pass. Prettier forced a reformat of `SettingsPage.tsx`.

- 14 review fixes: session lookup is one `Take` and `last_seen_at` is updated only when older than a minute (shared `session` package, used by middleware and tested there; the unused service copies are gone). `users.token_version` is read-only for gorm (`gorm:"->"`), raised only by raw SQL in `RevokeAllSessions`, so a `Save` of a stale user cannot undo a sign-out (regression test). Audit details are codes (`reason:wrong_password`, `format:csv`, `user:alice#3;changed:email`), translated in `AuditLogPage` via `utils/auditDetail.ts` in five locales, and the user agent has its own column. Password change/reset and admin password set save the user and revoke sessions in one transaction (the request fails if either fails); exports are audited after the data is loaded; `AuditLogPage` is lazy-loaded.

- 15: TOTP two-factor (RFC 6238, SHA-1, 6 digits, 30 s, one step of drift, hand-written with the standard library; no Go dependency). Migration 28: `users.totp_secret` (AES-GCM with the credential key, like CardDAV passwords), `totp_enabled`, `totp_last_step` (a code works once), table `recovery_codes` (sha256 hash, `used_at`). Enrolment is two steps: `POST /users/me/2fa/setup` (pending secret, returns key and otpauth URI) and `/enable` (needs a valid code, returns ten recovery codes once); `/disable` and `/recovery-codes` need password plus code or recovery code; `GET /users/me/2fa` status; `POST /admin/users/:id/2fa/reset` (signs the user out everywhere, audited); `two_factor_enabled` in the admin user list. Login: password alone on a 2FA account answers `200 {"totp_required": true}` with no cookie; the second request carries `totp_code` or `recovery_code`; a wrong code is `401 INVALID_TOTP_CODE`. Rate limiting reuses the account lockout: wrong codes count on the typed identifier and on a per-account key (`2fa:<id>`), so alternating username and email does not give more guesses, and asking for the code does not call `RecordSuccessfulLogin` (otherwise the password alone would reset a code-guessing lockout). OIDC logins never go through `LoginUser`, so they are exempt (test added); API tokens refuse the 2FA endpoints and are not affected. Audit events: totp_enabled, totp_disabled, totp_recovery_used, totp_recovery_regenerated, admin_totp_reset. UI: card on the settings page (QR code via the `qrcode` npm package rendered as an SVG data URI, which the CSP allows; key for typing; recovery codes dialog with copy), login second step with "use a recovery code", admin Users page badge and reset button, five locales. Tests: services (RFC vectors, window, replay, hashing, encryption), migration up/down, controllers (flow, lockout, replay, isolation, admin reset, disable, regenerate, OIDC exempt), Vitest (settings, login, auth, users) and e2e `two-factor.spec.ts` (TOTP computed with node crypto). `make precommit`, golangci-lint, `go test -race` and `make test-e2e` (47) pass. Prettier forced a reformat of `LoginPage.tsx` and `types/index.ts`.

- 16: FTS5 search. The pure-Go driver (modernc via glebarez) has FTS5 compiled in (checked with a probe before designing: `unicode61 remove_diacritics 2` and prefix queries work). Migration 29: `contacts_fts` (firstname, lastname, nickname, email, phone, and the values of the `emails`/`phones` JSON), `notes_fts` (content), `activities_fts` (title, description, location); plain FTS5 tables with rowid = row id, `AFTER INSERT/UPDATE OF .../DELETE` triggers, built from existing rows by the migration; rows are indexed whatever `deleted_at`, queries filter on the real table and `user_id`. New package `search` (`MatchQuery` quotes every word as a prefix term, so input is never FTS syntax; punctuation-only input falls back to LIKE). Contacts list: FTS join, best match first (bm25) unless `sort` is given; total/pagination unchanged. Unassigned notes and activities (title/description/location, or a linked contact's name via a column filter) use their index. When the FTS tables are missing the old LIKE code runs (tests using AutoMigrate still cover it). 20k contacts: word search 1.3 ms, prefix 3.6 ms, the LIKE search it replaces 72 ms. Tests: migration (build, triggers, down), `search`, controllers (accents, prefix, order-free words, syntax injection, scoping, trash/archive, ranking vs sort, pagination, notes, activities, 20k timing), e2e accent search. `make precommit`, golangci-lint, race and `make test-e2e` pass. Review fixes: phone-like input (only digits and phone punctuation) and Han/Kana/Hangul text go to the LIKE code (no migration: migration 29 is already published), so `795550` finds the middle of `+41795550101` again and CJK substrings work; the contact page sends no `sort` while searching (new "Best match" entry in the sort menu, shown only then) so the server ranks, and notes/activities are documented as date-ordered, not ranked; `search.Available` is remembered per database; terms are capped at 200 characters and 32 words; the 250 ms asserts in the 20k test are gone (times are logged).

- 17: thumbnails out of the database. Migration 30 adds `contacts.photo_thumbnail_file`; the old `photo_thumbnail` column (base64 data URL) is renamed `LegacyThumbnail` in the model and left untouched. `services.ExportLegacyThumbnails` (called in `main.go` after the migrations and before serving) writes every data URL that has no file yet to `<uuid>_thumb.jpg` (mode 0640, includes trashed contacts) and records the name; idempotent (only rows without a file, conditional update so a concurrent upload wins, bad rows are logged and skipped, an existing file name is never replaced), the blob stays as the rollback copy. New uploads, CardDAV/vCard/CSV/Monica imports and contact sync write `<uuid>_photo.jpg` + `<uuid>_thumb.jpg` and clear the legacy copy; replacing a photo deletes the old photo and thumbnail files; purge removes both. `GET /contacts/:id/profile_picture?thumbnail=true` serves the file with `ETag`, `Cache-Control: private, max-age=31536000, immutable` and 304 (and falls back to the legacy blob for a contact not exported yet); file names are checked against traversal. API: `photo_thumbnail` in contact lists, trash, birthdays and graph is now a URL (`/api/v1/contacts/:id/profile_picture?thumbnail=true&v=<uuid>`, empty = none), the version changes with the picture. List queries select `photo_thumbnail_file` instead of the blob (`fields=...photo_thumbnail` still works), birthdays and the full contact fetch omit the legacy column, activity preloads no longer select a thumbnail. The backup already archives the whole photo directory, so thumbnails are included. Frontend: `thumbnailSrc()` (prefixes `REACT_APP_API_URL`) for the avatars. Tests: migration up/down, export (idempotent, bad rows, existing file, trashed), file removal incl. traversal, serving/ETag/304/scoping/traversal/legacy fallback, list size without base64, upload and replace, 20 existing photo tests adapted, Vitest, e2e `thumbnails.spec.ts`. `make precommit`, golangci-lint, `make test-e2e` (50) pass. Prettier forced a reformat of `ContactsPage.tsx` and `DashboardPage.tsx`.

- 17 review fixes: trashed contacts' thumbnails are served to their owner (`Unscoped` lookup, user filter kept); `fields=photo_thumbnail` adds `ID`; replaced photos delete the old files (and the legacy copy) in CardDAV PUT, contact sync, VCF and Monica import, and the new files when the save fails (`carddav.PhotoSwap`, `replaceContactPhoto`); `RunMigrations` refuses a pending `DROP COLUMN photo_thumbnail` while unexported thumbnails exist (documented in development/backend.md; the drop itself stays deferred); API doc paragraph moved below the table; deployment.md mentions orphaned thumbnail files after a downgrade and re-upgrade.

## Decisions

- Step 17: the legacy column is kept (not cleared) after export so that a rollback of the migration or the binary loses nothing; this means the database file does not shrink until a later migration drops `photo_thumbnail` (see Questions). The API field name `photo_thumbnail` stays but holds a URL now, so the frontend `src` code is unchanged; the URL is root-relative (`/api/v1/...`), which assumes the web app and API share an origin (or `REACT_APP_API_URL`, handled by `thumbnailSrc`). Thumbnails need the login cookie like everything else (no public image URLs). The export runs synchronously at startup, because list queries no longer read the blob and a contact without a file shows no thumbnail; for 20k contacts it is a few seconds once.

- Step 16: search is now word-prefix, not substring: `ller` no longer finds `Müller` (it did with LIKE), `555` still finds `+41 79 555 0101` (digits are separate words), `5550101` does not. This is what the plan asks for ("prefix match") and is documented. Only the existing search endpoints changed (contacts list, unassigned notes, activities); there is no new global search endpoint. The activities page still matches contact names only, not their e-mail or phone. Notes and activities keep their date order (a chronological list), contacts rank by relevance when no sort is given. The migration is plain SQL and assumes FTS5 (always true for this driver); the LIKE fallback exists for databases without the tables, not as a runtime switch for a missing extension.

- Step 15: enabling and disabling two-factor sign out the other sessions (not the current one; tokens from before sessions existed cannot be revoked one by one). Enabling, disabling and new recovery codes all re-ask for the password (enabling plus a code); wrong passwords there count on the same lockout as wrong codes. Accounts without a password (OIDC-only) cannot enrol. The admin reset also ends the user's sessions, because the admin cannot verify who asked. Recovery codes are 10 x 50 bits, stored as HMAC-SHA-256 keyed with a key derived from `JWT_SECRET_KEY` separately from the TOTP encryption key (not bcrypt: high entropy, the lockout bounds guessing, and a leaked table cannot be tested offline). Changing `JWT_SECRET_KEY` makes stored TOTP secrets undecryptable (same as CardDAV credentials); documented; recovery codes become unverifiable too and the admin reset recovers. SHA-1 is the algorithm because not every authenticator app supports the others.

- Step 14: tokens from before the upgrade (no `sid`) are accepted so nobody is logged out by the update, which means such a token cannot be revoked one by one, only by "sign out everywhere" or when it expires. Sessions are a table (not only a token version) because listing and revoking single sessions needs rows. Audit rows of deleted accounts are kept until retention (security record); they hold the username, IP and user agent. The IP is `c.ClientIP()`, so behind a proxy it needs `TRUSTED_PROXIES`. Audit writes never fail a request (errors are logged). Not audited: contact/note changes (not security events), CardDAV/ICS fetches.
- Step 13: enforcing, not report-only: production nginx already enforced a CSP (so report-only would have loosened it), the whole e2e suite passes under it, and report-only needs a report endpoint nobody reads. `style-src 'unsafe-inline'` stays (emotion injects style tags; a nonce is not possible with static nginx). COOP `same-origin` and CORP `same-origin` on the SPA, CORP `same-site` on the API (a SPA on a sibling subdomain can still show photos). The backend does not trust `X-Forwarded-Proto` for anything but HSTS.
- Step 12: OIDC discovery and the SMTP host are exempt from the guard: only the administrator sets them (environment), and they are often internal. Documented in deployment.md. The warning counts non-deleted users (more than one) and fires when any of the four features is unblocked.
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

- Step 16: behaviour change for users: substring search became word-prefix search (see Decisions). If you want substring back for short fragments, a trigram index (`tokenize='trigram'`) is the alternative at roughly 3x the index size.
- Step 16: any later migration that rebuilds `contacts`, `notes` or `activities` must recreate the FTS triggers (documented in `docs/development/backend.md`).

- Step 15: the login now answers `200 {"totp_required": true}` (not an error) for 2FA accounts, so the frontend 401 handler is not triggered; a client that ignores the body would think it logged in but has no cookie. The password check happens before the code, so a wrong password and a right password with a missing code are distinguishable (inherent to a two-step form).
- Step 15: `LoginPage.tsx` and `types/index.ts` were reformatted by Prettier (the format check forced it); the `qrcode` package (1.5.4, MIT) is a new frontend dependency.

- Step 14: the audit log stores IP addresses and user agents for the retention period (default 365 days); check that is acceptable for the instances you care about (`AUDIT_LOG_RETENTION_DAYS`).
- Step 14: one extra indexed read per authenticated JWT request (session lookup) next to the user lookup; a write at most once a minute per session.
- Step 13: the nginx changes (maps, `include`, X-Forwarded-Proto forwarding) could not be run here (no nginx binary, no Docker): the check is a static test plus the e2e run against the Node server that mimics it. CI's `docker-compose.test.yml` E2E run goes through the real nginx with the new CSP guard, so it is the first real test. If it fails with a map or include error, fix `frontend/nginx.conf` and `docker/nginx.conf` together.
- Step 13: `frontend/e2e/fixtures.ts` was reformatted by Prettier (the format check forced it).
- Step 13: a deployment with the API on another origin (`REACT_APP_API_URL` set) needs `connect-src` relaxed; see deployment.md.
- Step 12: the guard now also refuses 100.64.0.0/10 (CGNAT, used by Tailscale and some clouds' metadata) and multicast when blocking is on. Users who enable it and sync from a Tailscale address must keep it off or use a name outside that range.
- Step 12: the banner is on the Users page only (admins), not on every page.
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

- Step 17: drop `contacts.photo_thumbnail` in a later migration once nobody needs to roll back (it frees the space and removes `LegacyThumbnail` and the fallback in `serveLegacyThumbnail`/`ContactToVCard`). Not done here because it would make the export irreversible; decide when to do it.

- Step 10: `FindOrProvisionUser` links a verified-email OIDC login to an existing account even if that account is already linked to a different subject (the link is overwritten). Left as is; consider refusing. Not tested/changed.
- Step 10: not covered in depth: the sync paths behind `SyncCardDAVConnection`/`SyncCalendarSubscription` (covered by the services tests) and `DiscoverCardDAVAddressBooks` (needs a CardDAV server).
- The down migration of 000008 leaves `contacts.etag`, `vcard_extra` and `vcard_uid` in place (it predates `DROP COLUMN`). Harmless, but a rollback below version 8 is not byte-identical to a fresh schema. Left as is; the migration test tolerates it.

## Blocked
