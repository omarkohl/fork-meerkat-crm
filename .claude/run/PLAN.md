# Run plan: first-sprint

Goal: ship the "Suggested First Sprint" from `IMPROVEMENTS.md` as small, stacked PRs
against the fork (`omarkohl/fork-meerkat-crm`), never upstream. Every step leaves
`main` deployable.

Read first: `.claude/AUTONOMOUS-RUNS.md`, `README-developer.md`, `IMPROVEMENTS.md`
(the item numbers below refer to it), `.github/workflows/`, `Makefile`.

## Settled before the run (the user, 2026-10-10)

- Scope is the first-sprint list in `IMPROVEMENTS.md`, in the order below.
- PRs only against the fork `omarkohl/fork-meerkat-crm`.
- Reuse one implementer; start a new one if its context is above 200k after a task.

## Settled by Claude (the user may override before launch)

- The root `Makefile`, `tsc` and ESLint in `make precommit` already exist, so items
  1.5/1.6 only need to be gated in CI, not built.
- Prettier is introduced check-only on changed files; no repo-wide reformat commit.
- New linters start with a baseline: fix real findings, exclude noisy rules with a
  comment, rather than a huge diff.
- Dependency swaps keep behaviour identical; no feature changes.
- The "Trash" feature uses the existing GORM soft delete; auto-purge after 30 days.
- ICS feed uses a per-user random token URL, stored hashed or as an API-token style
  secret following the existing API token pattern.
- Backups: scheduled `VACUUM INTO` plus photos tarball, retention N from config,
  download via admin endpoint and UI.

## Steps

- [x] 0. **Environment check** (no own commit; record in PROGRESS.md with step 1):
      `make precommit` green, whether `make test-e2e` runs. A failure that blocks
      every step goes under `## Blocked`.
- [x] 1. **SQLite pragmas (5.2)**: WAL, `busy_timeout`, `foreign_keys=ON`,
      `synchronous=NORMAL`. Test that they are active; make sure enabling foreign
      keys breaks no existing test or migration.
- [x] 2. **Go lint in CI (1.1)**: `.golangci.yml` (errcheck, staticcheck, gosec,
      revive, bodyclose, sqlclosecheck), CI job, fix or justify findings.
- [x] 3. **Frontend lint/format/type gates in CI (1.3-1.5)**: ESLint, `tsc --noEmit`
      and Prettier check (changed files) as CI jobs; add `gofmt` check.
- [x] 4. **Dependabot and govulncheck (3.1, 3.3)**: npm, github-actions, docker,
      bundler ecosystems with grouped minor/patch; `govulncheck` and npm audit job.
- [x] 5. **Replace `nfnt/resize` (6.4)** with `golang.org/x/image/draw`; photo tests
      prove output sizes unchanged.
- [x] 6. **`golang-jwt` v4 to v5 (6.2)**; auth tests cover the migration.
- [x] 7. **`gocron` v1 to v2 (6.3)**; scheduler tests still pass.
- [x] 8. **ICS feed export (feature)**: token-protected subscribable URL for
      birthdays and reminders; migration, controller, API client, settings UI,
      i18n (5 locales), isolation test, docs/api-reference.md.
- [ ] 9. **Trash page (feature)**: list soft-deleted contacts, restore, purge,
      auto-purge after 30 days via the scheduler; API, UI, i18n, isolation test.
- [ ] 10. **Built-in backups (5.3)**: scheduled `VACUUM INTO` plus photos tarball,
      retention, admin download endpoint and UI, i18n, tests, docs.
- [ ] 11. **Docs**: README, `docs/`, `docs/api-reference.md`, deployment notes for
      backups. Full `make precommit` and `make test-e2e`.
- [ ] 12. **Finish**: summary and the "For review" list in PROGRESS.md (copied into
      that PR's body), then delete `.claude/run/` in its own commit.
