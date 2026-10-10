# Run plan: second-sprint

Goal: close the open questions of the first sprint (PR #15), harden what was built, then work
through `IMPROVEMENTS.md` by value for a self-hosted personal CRM: security and data safety first,
then test and supply-chain hygiene, then search and reliability, then features, then the frontend
modernisation. Every step leaves `main` deployable. The stack is built on top of
`first-sprint/12-finish` (PR #15); CI of past PRs is not touched.

Read first: `.claude/AUTONOMOUS-RUNS.md`, `README-developer.md`, `IMPROVEMENTS.md`, and the
"Questions" and "Decisions" of PR #15 (`first-sprint/12-finish`).

PRs go to `omarkohl/fork-meerkat-crm` only, never to upstream. Reuse the implementer until its
context is above 200k at the end of a step, then start a new one. Reviewer is always fresh.

## Settled before the run (the user, 2026-10-10)

- Claude decides the open questions from the first sprint (below), from the view of UX, robust
  design and best practice for self-hosted apps. CI jobs that fail are fixed on top of the last PR
  (step 1); older PRs are left alone.
- Work does not stop while steps are open. When all steps are ticked and budget remains, the
  orchestrator appends new steps from `IMPROVEMENTS.md` (see "Replenish") before the finish step.

## Settled by Claude

- Soft-deleted users lose all access at once: API tokens, ICS feed, JWTs are refused (check the
  user row on every authenticated request, not only at login), tokens and feeds are deleted.
- Purge in CardDAV pull-only mode keeps the link as a tombstone (`purged_at` on the link): the sync
  skips tombstoned links and never re-imports. A purged contact stays gone.
- Webhooks: add `contact.restored`. No purge event (`contact.deleted` already fired).
- ICS feed: show "last fetched" (timestamp updated at most once per minute), like API tokens.
- Photo directories 0750, files 0640. `Close` errors on write paths are returned and tested.
- uint conversions of JWT claims and URL ids go through one range-checked helper.
- Startup tasks in `main.go` run under one context and a `WaitGroup`; shutdown waits for them
  (bounded) before the DB closes.
- `docs/api-reference.md` becomes complete; a Go test compares the registered routes with the
  routes documented, so it cannot drift again.
- Docker base images pinned to a version tag; GitHub Actions pinned by SHA; Dependabot keeps them
  current. Backups: docs say the data dir must be writable by the container user.
- SSRF: one `BLOCK_PRIVATE_URLS` switch overriding the three older flags (they keep working);
  default stays `false` because LAN CalDAV/Radicale/Monica hosts are a common legitimate setup;
  covers the CardDAV client too; startup warning and an admin banner when it is off and more than
  one user exists.
- New features are opt-in or additive; nothing changes for existing data without a user action.
  Every feature step ships with: migration (up and down), validation, user isolation test, i18n in
  five locales, API client, docs, e2e where UI behaviour changes.
- Where a step needs a design choice not listed here, take the simplest option that fits the code
  base and record it in PROGRESS.md `## Decisions`.

## Steps

- [x] 0. **Environment check**: `make precommit` green, whether `make test-e2e` runs.
- [x] 1. **CI fixes** (branch `second-sprint/01-ci-fixes`, done before the plan): bump `x/net` for
      govulncheck, golangci-lint v2.14.0, two new lint findings fixed.
- [x] 2. **Deleted users lose access**: JWT middleware checks the user is active on every request;
      `DeleteUser` removes API tokens and the ICS feed and invalidates sessions; same for admin
      disable if it exists. Simplify `user_id` claim handling to the one real type, range-checked
      helper for uint conversions (removes the G115 suppressions). Tests, isolation case.
- [x] 3. **Trash follow-ups**: link tombstone on purge (migration), pull-only sync skips it;
      `contact.restored` webhook (docs, UI event list, i18n); tests for both.
- [x] 4. **File and shutdown robustness**: photo dir 0750/files 0640 (existing dirs are left
      alone, documented); write-path `Close` errors checked; startup goroutines under a context and
      WaitGroup, shutdown waits bounded. Remove the matching gosec suppressions.
- [x] 5. **ICS feed "last fetched"**: column, update throttled to once a minute, shown on the
      Integrations card, i18n, tests.
- [x] 6. **Complete API reference**: document every route group missing (webhooks, calendars,
      CardDAV connections, Monica import, OIDC, enabled-contact-fields, trash, backups, ICS, fix
      `/api-tokens` path); Go test that every registered route appears in the doc.
- [x] 7. **Supply chain**: pin actions by SHA, align `docker/build-push-action`, pin Docker base
      tags, Trivy scan of the built image, SBOM and provenance on publish, build (no push) the image
      on PRs, build the docs on PRs touching `docs/**` (Jekyll via the workflow container).
- [x] 8. **Test infrastructure**: `go test -race` in CI, coverage for Go and Vitest uploaded as
      artifact and printed in the job summary, migration test (all up, all down, up again on seeded
      data), Radicale CardDAV integration tests as a service container.
- [x] 9. **Fuzz and parser tests**: fuzz targets for the vCard mapper, CSV import, Monica import
      parsing and ICS building; a short fuzz run (30 s each) in a nightly workflow; fix what they find.
- [x] 10. **Tests for untested logic**: `birthday_service`, `webhook_service` (signature, retries,
      SSRF), `credential_crypto`, `mailer`, and controllers for oidc, photo, calendar, carddav,
      graph, import, admin_user. Fix bugs found (one commit, list them).
- [x] 11. **i18n checker and local hooks**: script that fails when a key is missing in any locale
      (frontend and backend), also detects unused/duplicate keys as warnings; CI job; `lefthook`
      config (fmt, lint, i18n on staged files) documented as optional.
- [x] 12. **Unified SSRF switch**: `BLOCK_PRIVATE_URLS`, CardDAV coverage, warning and admin banner,
      docs, tests (including DNS rebinding: resolve once, dial the checked IP).
- [x] 13. **Security headers and CSP**: audit `security_headers.go`, `frame-ancestors`, CSP for
      the SPA (served by nginx too), report-only first then enforcing if the e2e suite passes;
      docs for reverse proxies.
- [ ] 14. **Sessions and audit log**: list/revoke sessions (token version per user, "sign out
      everywhere"), audit log table (login, failed login, export, token/ICS/backup actions, admin
      actions) with retention, admin page with filter, i18n, tests.
- [ ] 15. **TOTP two-factor**: enrol with QR and recovery codes, enforce at login, admin can reset,
      rate-limited, secrets encrypted with the existing credential crypto, OIDC users exempt.
      Passkeys are a later step.
- [ ] 16. **Search with SQLite FTS5**: contacts, notes, activities; triggers keep the index in
      sync; diacritics folding and prefix match; ranked results; migration builds the index; falls
      back to LIKE if FTS5 is unavailable; tests and performance check on 20k contacts.
- [ ] 17. **Thumbnails out of the database**: move `PhotoThumbnail` to files with ETag and cache
      headers, migration that exports existing blobs, drop the column in a later migration; list
      queries get smaller; backup includes the files.
- [ ] 18. **Prometheus metrics**: `/metrics` (off by default, optional bearer token or separate
      listener), request latency, job results from `job_execution`, webhook and email failures;
      docs with an example scrape config and Grafana-free alert hints.
- [ ] 19. **Outbox for webhooks and email**: table-backed queue with retries and backoff, survives
      restarts, delivery status visible on the webhook page, dead-letter after N tries.
- [ ] 20. **Keep-in-touch cadence**: per contact and per circle ("every N days"), overdue list on
      the dashboard from the last activity, snooze, reminder email digest option.
- [ ] 21. **Important dates**: anniversaries, name days, death dates beyond birthdays, with
      reminders, ICS export and import mapping; "on this day" dashboard widget.
- [ ] 22. **Gift ideas**: per contact list with status (idea, bought, given), surfaced in the
      birthday reminder email N days before.
- [ ] 23. **Markdown notes and @mentions**: safe renderer (sanitised), mentions create links
      between contacts, editor stays plain text with preview.
- [ ] 24. **Command palette and shortcuts**: `Ctrl+K` search and actions, `n`, `a`, `/`
      shortcuts, help dialog, no clash with inputs, accessible, i18n.
- [ ] 25. **Duplicate detection and merge**: similarity by name, email, phone; review list;
      merge keeps notes, activities, relationships; undo via trash semantics; tests on isolation.
- [ ] 26. **Saved filters and smart circles**: store filter definitions, show on the sidebar and
      dashboard, dynamic membership.
- [ ] 27. **Timeline filters and export**: filter by type and date, export one contact's history
      as Markdown and PDF-ready print stylesheet.
- [ ] 28. **Push notifications**: ntfy, Gotify and Apprise targets per user for reminders and
      digests, test button, uses the outbox (step 19), SSRF guarded.
- [ ] 29. **Frontend modernisation**: route-level code splitting; ESLint 9 flat config with
      `typescript-eslint`, react-hooks and jsx-a11y in CI; dependency bumps (Vitest, jsdom,
      plugin-react); accessibility checks with axe in Playwright.
- [ ] 30. **Vite migration**: replace CRA (`REACT_APP_*` -> `VITE_*`), keep the PWA, Dockerfile
      and nginx unchanged from the outside, e2e green, bundle size recorded in the PR.
- [ ] 31. **Data layer**: TanStack Query replacing the custom data hooks one resource at a time
      (contacts first), optimistic updates for the obvious cases, tests per migrated hook.
- [ ] 32. **Backend structure**: split the large files (`contact_sync_service.go`,
      `monica_import_session.go`, `import_service.go`) with no behaviour change, typed config
      parsing, repository layer for contact search/list queries.
- [ ] 33. **Docs and community**: `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, PR
      template, `CONTEXT.md` glossary, `docs/adr/` (record the decisions of both sprints),
      architecture diagram, Home Assistant / n8n webhook examples, remove or move legacy compose
      files, fix stale `README-developer.md` links.
- [ ] 34. **Docs pass**: README, `docs/`, `docs/api-reference.md`, settings and env tables for
      every step above; full `make precommit` and `make test-e2e`.
- [ ] 35. **Replenish**: if budget remains, append steps from the unused parts of `IMPROVEMENTS.md`
      (passkeys, map view, attachments, Google/Outlook import, email ingestion, optional LLM
      summaries, shared circles, Helm/Unraid templates, release automation, Weblate) and carry on
      with them. Only when nothing sensible is left go to the next step.
- [ ] 36. **Finish**: summary and the "For review" list in PROGRESS.md (copied into that PR's body),
      then delete `.claude/run/` in its own commit.
