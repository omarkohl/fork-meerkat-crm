# Meerkat CRM – High-Value Improvements

Ordered by value within each section. Effort: **S** (< 1 day), **M** (days), **L** (week+).

## 1. Tooling & Linting

| # | Item | Effort | Why |
|---|------|--------|-----|
| 1.1 | Add `golangci-lint` (errcheck, staticcheck, gosec, revive, bodyclose, sqlclosecheck) + CI job | S | No Go linter today. `gosec` matters for an app that stores personal data and fetches remote DAV URLs. |
| 1.2 | Migrate frontend from CRA (`react-scripts` 5, unmaintained) to **Vite** | M | Vitest + `@vitejs/plugin-react` already present. Faster dev server, modern deps, removes many transitive CVEs. Rename `REACT_APP_*` → `VITE_*`. |
| 1.3 | Replace CRA eslint config with flat ESLint 9 + `typescript-eslint` + `eslint-plugin-react-hooks` + `jsx-a11y` | S | Current lint only runs inside `react-scripts start`; not in CI. |
| 1.4 | Add Prettier (or Biome for lint+format in one tool) and `gofmt`/`goimports` check in CI | S | No formatter enforced. |
| 1.5 | Run `tsc --noEmit` in CI | S | Type errors are not gated today. |
| 1.6 | Root task runner (`justfile` or root `Makefile`): `lint`, `fmt`, `test`, `e2e`, `dev` | S | One entry point for backend + frontend. Today: backend Makefile only, frontend scripts only. |
| 1.7 | Pre-commit hooks (`lefthook` / `pre-commit`) running fmt + lint on staged files | S | Catches issues before CI. |
| 1.8 | i18n key checker: fail CI if a key exists in `en.json` but not in `de/it/es/fr` (frontend + backend) | S | 5 locales × 2 apps drift easily. |

## 2. Testing

| # | Item | Effort | Why |
|---|------|--------|-----|
| 2.1 | Coverage reports (`go test -coverprofile`, `vitest --coverage`) uploaded in CI; show trend | S | Know where gaps are. |
| 2.2 | Controller tests for untested files: `oidc`, `photo`, `calendar`, `carddav`, `graph`, `import`, `admin_user` controllers | M | Auth, upload, and admin paths are the highest-risk and have no tests. |
| 2.3 | Tests for `birthday_service`, `webhook_service` (signature, retries), `credential_crypto`, `mailer` | S | Pure logic; cheap to cover. |
| 2.4 | Frontend component tests (RTL) for dialogs: `AddContactDialog`, `ReminderDialog`, `ImportContactsDialog` | M | Only 4 frontend unit test files exist. |
| 2.5 | Run `go test -race` in CI | S | Scheduler, sync services, and import sessions are concurrent. |
| 2.6 | Fuzz tests (`go test -fuzz`) for vCard mapper, CSV import, Monica import parsing | S | Untrusted input parsers. |
| 2.7 | Migration test: apply all `up`, then all `down`, then `up` again on a seeded DB | S | 23 migrations; down scripts are likely never exercised. |
| 2.8 | Run Radicale CardDAV integration tests in CI (service container) | S | Today opt-in via `MEERKAT_CARDDAV_IT=1` only. Token-sync path is otherwise untested in CI. |
| 2.9 | Playwright: add a11y checks (`@axe-core/playwright`) and visual snapshots for dark/light mode | S | Low cost, catches regressions in themes. |
| 2.10 | Contract tests: generate OpenAPI (see 4.1) and validate responses in Go tests | M | Frontend `types/api.ts` and backend structs drift silently. |

## 3. CI/CD & Supply Chain

| # | Item | Effort | Why |
|---|------|--------|-----|
| 3.1 | Dependabot: add `npm` (frontend), `github-actions`, `docker`, `bundler` (docs) ecosystems; group minor/patch | S | Only `gomod` is covered now. |
| 3.2 | Pin actions by SHA; align `docker/build-push-action` versions (v5 vs v6 used) | S | Supply-chain hygiene. |
| 3.3 | `govulncheck` + `yarn npm audit` / OSV-Scanner job | S | Known-CVE gating. |
| 3.4 | Trivy/Grype scan of the published image; SBOM + provenance (`provenance: true`, `sbom: true` in build-push) | S | Self-hosters pull `:latest` blindly. |
| 3.5 | Build PR images on PRs (no push) so Dockerfile breakage shows before release | S | Today images build only on tags (E2E builds the combined image only). |
| 3.6 | Release automation (`release-please` or `goreleaser`) with conventional commits → changelog | M | Manual tagging today. |
| 3.7 | Docs workflow: build (not deploy) on PRs that touch `docs/**` | S | Broken Jekyll builds are found only after merge. |

## 4. Architecture & Code Structure

| # | Item | Effort | Why |
|---|------|--------|-----|
| 4.1 | **OpenAPI spec** (code-first with `swaggo/swag` or `huma`, or spec-first with `oapi-codegen`) + generated TS client (`openapi-typescript`) | L | ~100 routes, hand-written `docs/api-reference.md` and hand-written `frontend/src/api/*.ts`. One source of truth removes a whole bug class and gives API users Swagger UI. |
| 4.2 | Repository layer between controllers and GORM | M | Controllers contain query logic (e.g. search `LIKE` clauses). Easier testing, easier move to FTS (5.1). |
| 4.3 | Split large files: `contact_sync_service.go` (1.3k LOC), `monica_import_session.go` (1.2k), `import_service.go` (1k), `ContactDetailPage.tsx` (800) | M | Readability and review cost. |
| 4.4 | Frontend data layer: **TanStack Query** instead of custom `{data, loading, error, refetch}` hooks | M | Caching, dedup, optimistic updates, background refetch for free. Removes most of `hooks/`. |
| 4.5 | Form library (`react-hook-form` + `zod`) for large dialogs | M | Shared validation; schemas can be generated from OpenAPI. |
| 4.6 | Route-level code splitting (`React.lazy`) – especially `NetworkPage` (force-graph) and import dialogs | S | Smaller initial bundle. |
| 4.7 | Typed config via `caarlos0/env` or `kelseyhightower/envconfig` | S | Replaces manual env parsing in `config.go`. |
| 4.8 | Fix stale docs: `README-developer.md` links `backend/middleware/middleware.go` (does not exist → `errors/middleware.go`); mixes `npm run` and `yarn` | S | Onboarding friction. |
| 4.9 | Add `CONTEXT.md` (domain glossary: contact, circle, activity, note, reminder, connection) and `docs/adr/` | S | Helps contributors and AI agents. |
| 4.10 | Remove legacy compose files (`docker-compose.legacy.yml`, `docker-compose.build.legacy.yml`) or move to `deploy/legacy/` | S | Root clutter. |

## 5. Performance & Reliability

| # | Item | Effort | Why |
|---|------|--------|-----|
| 5.1 | **SQLite FTS5** for global search over contacts, notes, activities | M | Current search is `LOWER(x) LIKE '%q%'` across many columns → full scans, no ranking, no diacritics folding. FTS5 gives ranked, fast, typo-tolerant(ish) search. |
| 5.2 | SQLite pragmas: `journal_mode=WAL`, `busy_timeout`, `foreign_keys=ON`, `synchronous=NORMAL` | S | `gorm.Open(sqlite.Open(dbPath))` uses defaults. WAL avoids `database is locked` with sync jobs + web traffic. FKs would replace manual cascade cleanup. |
| 5.3 | **Built-in backups**: scheduled `VACUUM INTO` + photos tarball, retention N, download from admin UI | M | Personal data; most self-hosters have no backup. Big trust win. |
| 5.4 | Move photo thumbnails out of DB (`PhotoThumbnail` base64 column) to files; serve with ETag/cache headers | S | Smaller rows, faster list queries. |
| 5.5 | Prometheus `/metrics` (request latency, sync job results, email failures) | S | `job_execution` model exists – expose it. |
| 5.6 | Persistent job queue / retries for webhooks and email (e.g. table-backed outbox) | M | Failed deliveries are lost on restart. |

## 6. Dependencies

| # | Item | Effort | Why |
|---|------|--------|-----|
| 6.1 | `react-scripts` → Vite (see 1.2); drop `cra-template-pwa-typescript` from runtime deps | M | CRA is deprecated. |
| 6.2 | `golang-jwt/jwt/v4` → `v5` | S | v5 is current; better validation API. |
| 6.3 | `go-co-op/gocron` v1 → `v2` | S | v1 is in maintenance; v2 has context support and better error handling. |
| 6.4 | `nfnt/resize` (archived since 2018) → `golang.org/x/image/draw` (already a dep) | S | Remove dead dependency. |
| 6.5 | `@vitejs/plugin-react` ^4 → current; `jsdom` ^26 → current; Vitest 3 → 4 | S | Stay in step. |
| 6.6 | Evaluate `glebarez/sqlite` (pure Go, wraps modernc) vs `ncruces/go-sqlite3` (wazero) | S | Both pure-Go; `ncruces` tracks SQLite upstream faster and supports FTS5 well. `wazero` is already an indirect dep. |

## 7. Security

| # | Item | Effort | Why |
|---|------|--------|-----|
| 7.1 | **2FA**: TOTP + **passkeys** (WebAuthn, `go-webauthn/webauthn`) | M | CRM of personal data; password-only for local accounts. |
| 7.2 | Encryption at rest option for notes (per-user key derived from password) | L | Differentiator for privacy-focused self-hosters. |
| 7.3 | Unify SSRF guards: one `BLOCK_PRIVATE_URLS` switch (today separate `CALDAV_`/`MONICA_`/`WEBHOOK_` flags, all default `false`), cover CardDAV client too, consider default `true` | S | Three flags are easy to miss; default-off exposes multi-user instances. |
| 7.4 | Session management UI: list active sessions / API tokens, revoke all | S | Already has API tokens; extend. |
| 7.5 | Audit log (logins, exports, token creation, admin actions) | M | Accountability on multi-user instances. |
| 7.6 | CSP header with nonces; check `security_headers.go` covers `frame-ancestors` | S | Defense in depth vs XSS in notes. |

## 8. Features

### Relationship intelligence
- **Keep-in-touch cadence per contact/circle** ("every 30 days") with an auto-computed *"overdue"* list on the dashboard, based on last activity. Turns the CRM from passive storage into an active nudge. **M**
- **Relationship health score**: frequency + recency of activities; sparkline on contact card. **M**
- **"On this day"** dashboard widget: activities and notes from 1/5/10 years ago. **S**
- **Gift ideas & wishlist** per contact, surfaced N days before birthday reminder. **S**
- **Important dates** beyond birthdays (anniversaries, name days, death dates) with reminders. **S**

### Capture & input
- **Quick-capture command palette** (`Ctrl+K`): search contacts, "log call with Anna", jump to pages. **M**
- **Markdown in notes** (safe renderer like `react-markdown` + `rehype-sanitize`) and `@mentions` of contacts that create links. **M**
- **PWA share target**: share text/URL/photo from phone into a note for a contact. Service worker already exists. **S**
- **Email/Telegram/Matrix bot ingestion**: forward a message → becomes a note on the matching contact. **L**
- **Attachments** on notes/activities (files, not only profile photos). **M**

### Notifications & integrations
- **ntfy / Gotify / Apprise / Web Push** notifications in addition to email. Apprise covers ~100 services in one integration. **S–M**
- **ICS feed export** (subscribable, token-protected URL) for birthdays and reminders → shows in any calendar app. Inbound calendar sync exists; outbound does not. **S**
- **Google Contacts / Outlook import** via CSV presets or OAuth. **M**
- **Home Assistant / n8n** friendly: document webhooks + API tokens with examples. **S**

### Organization & UX
- **Trash / undo**: `gorm.Model` already soft-deletes; add a Trash page with restore and auto-purge after 30 days. **S**
- **Saved filters / smart circles** (e.g. "Berlin + no contact in 90 days"). **M**
- **Map view** of contacts by address (Leaflet + OSM, optional geocoding). **M**
- **Duplicate detection & merge** (name/email/phone similarity) — high value after CSV/VCF/Monica imports. **M**
- **Timeline filters and export** (PDF/Markdown of a contact's history). **S**
- **Keyboard shortcuts** throughout (`n` new note, `a` activity, `/` search). **S**

### Optional AI (opt-in, self-host friendly)
- Pluggable LLM provider (Claude API or local Ollama) for: summarizing a contact's history before a meeting, extracting dates/facts from free-text notes into structured fields, suggesting follow-ups. Off by default; never sends data without explicit config. **M**

### Multi-user
- **Shared circles** between users of one instance (e.g. household shares "Family"). **L**

## 9. Documentation & Community

- Architecture diagram (request flow nginx → Gin → GORM/SQLite, scheduler, DAV sync) in `docs/development/architecture.md`. **S**
- `CONTRIBUTING.md` at root (GitHub surfaces it), `SECURITY.md` with disclosure process, `CODE_OF_CONDUCT.md`. **S**
- PR template with checklist (tests, i18n keys for all 5 locales, migration up/down). **S**
- Helm chart / Unraid / TrueNAS / Umbrel app templates for discoverability. **M**
- Translation platform (Weblate free for OSS) to grow beyond 5 languages. **S**

## Suggested First Sprint (quick wins)

1. SQLite pragmas (5.2)
2. golangci-lint + ESLint + Prettier + `tsc` in CI (1.1, 1.3–1.5)
3. Dependabot ecosystems + govulncheck (3.1, 3.3)
4. Replace `nfnt/resize`, bump jwt v5, gocron v2 (6.2–6.4)
5. ICS feed export + Trash page (feature quick wins)
6. Built-in backups (5.3)
