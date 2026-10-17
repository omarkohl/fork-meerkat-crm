# Progress

## Next step

10.

## Stack

<!-- step: branch, PR number (base) -->

## Log

- Step 0: `make precommit` green. `make test-e2e` runs (54 passed, no workaround needed).
- Step 1: `docs/adr/` with README index and ADRs 0001-0008 (record decisions, machine checks, testing strategy, SQLite/WAL, user_id isolation, migrations, AppError, five locales); linked from `architecture.md`.
- Step 2: committed `AGENTS.md` (index, hard limits, failure modes, bad/good examples, jj and push rules, issue footers); `CLAUDE.md` is only `@AGENTS.md` and no longer ignored. `make precommit` green.
- Step 3: targets declare `install` as prerequisite; added `test-coverage`, `arch` (placeholder), `seed` (`scripts/seed.mjs`, needs `make dev` running; tested against a temp backend); `test-e2e` is in `precommit`; Checks in AUTONOMOUS-RUNS.md and run-one-test table in testing.md updated. `make precommit` green incl. E2E (54 passed).
- Step 4: Makefile split into `install-go`/`install-frontend`, `lint-go`/`lint-frontend`, `test-go`/`test-frontend`, `test-coverage-go`/`-frontend`, `test-carddav`, `test-e2e-docker`, `vuln-go`, `audit-frontend`, `fuzz`. Lint, unit, e2e, security and fuzz workflows call them (docs, docker-build, docker-publish unchanged: Jekyll and image build, not make). actionlint v1.7.7 clean; Go version in fuzz.yml and the CardDAV job of unit-tests.yml raised 1.26 -> 1.27 (needed: `backend/go.mod` requires 1.27.0); CI itself not run locally. `make precommit` green.
- Step 5: the full reformat (`04bed7d6` "style: format all files with gofmt and Prettier") and `format-check` on all files (no `BASE` logic) were already in the history below this run (`4eb6e3b1`); `make format` changes nothing. So no new reformat commit; this commit only adds `.git-blame-ignore-revs` for `04bed7d6` and a note in contributing.md. `make precommit` green.
- Step 6: enabled `noUncheckedIndexedAccess`, `noImplicitOverride`, `noPropertyAccessFromIndexSignature` in `frontend/tsconfig.json`; fixed 86 errors (bracket access for index signatures incl. `process.env['REACT_APP_...']`, `override` on class members, destructuring with defaults for regex/split parts, `.slice(0, 10)` instead of `.split('T')[0]` in four dialogs, `?.` in tests). No non-null assertions. The three flags are also in `tsconfig.node.json` (E2E and Playwright config) and `make typecheck` now checks that project too. `make precommit` green incl. E2E.
- Step 7: ESLint 9 flat config `frontend/eslint.config.mjs` (`typescript-eslint` strictTypeChecked, `jsx-a11y` recommended, `eslint-comments` recommended + `require-description`/`no-unlimited-disable`, `react-hooks` rules-of-hooks/exhaustive-deps, `reportUnusedDisableDirectives: error`; all errors); `--fix` applied (mostly braces for `no-confusing-void-expression`); 4 unused catch/param variables, 2 undescribed disables fixed by hand; stale file-wide disable in `service-worker.ts` removed (rule was not active). 529 findings remain, bulk-suppressed (counts per file and rule) in `frontend/eslint-suppressions.json`, reasons per rule class in `docs/development/frontend.md`. Only `src/**/*.{ts,tsx}` is linted (e2e comes with step 16). Checked: a new floating promise in `src/ContactsPage.tsx` fails; fixing one makes `make lint-frontend` fail until `--prune-suppressions` (exit 2). Each rule shown to fail on a violation file (rules-of-hooks, exhaustive-deps, no-floating-promises, no-non-null-assertion, eslint-comments, jsx-a11y/alt-text, restrict-template-expressions). Build and tests green.
- Step 8: Vitest `coverage.thresholds` (statements 28, branches 80, functions 33, lines 28; current 28.09/80.51/33.76/28.09) and `scripts/check-go-coverage.mjs` (test first, red then green) with `GO_COVERAGE_MIN=62` (current 62.9) called by `make test-coverage-go`. `precommit` runs `test-coverage` instead of `test`; CI already calls the coverage targets, so thresholds apply there. Shown to fail with a too-high threshold for both. Script tests moved from `i18n-check` to `make test-scripts` (in `test`, `precommit` and the lint workflow's locales job); the script checks its main-module test with `pathToFileURL` and rejects a blank threshold (exit 2). `make precommit` green.
- Step 9: ADR 0009; `backend/clock` (Clock, Real, Fake, `FAKE_NOW` clock; tests first); `middleware.ClockMiddleware` + `RequestNow`; scheduler tasks take `clk.Now()` once per job; all 84 `time.Now()` outside main/middleware/clock/metrics replaced (services take `now time.Time`, import session managers hold a clock set by `controllers.SetImportClock`, GORM `NowFunc`, JWT time func); `forbidigo` bans `time.Now` with the ADR link (shown to fail on a violation file; two reasoned `nolint`: SMTP deadline, health duration); `FAKE_NOW` config + validation + warning; frontend `src/utils/now.ts` and an ESLint ban on `new Date()`/`Date.now()` elsewhere in `src` (shown to fail). Docs: backend.md, getting-started.md. One commit. Review fixes: job-like work (outbox, webhooks, reminders, calendar and contact sync) takes the `clock.Clock` and reads it where it stamps something (after slow I/O, lock release at the end); TOTP always uses the real time; purge jobs are skipped while `FAKE_NOW` is set; `forbidigo` runs with `analyze-types` (an aliased `time` import is caught; without it the alias passed); Monica import reads the clock once per import; `Date()` without `new` is banned too; tests for the manager clocks and for retry scheduling after a slow send; `FAKE_NOW` in deployment.md and `.env.example` (test only). `make precommit` green.

## Decisions

- Clock injection: request-scoped services take a trailing `now time.Time`; long-running jobs take the `clock.Clock`; the two import session managers hold a `clock.Clock` (set once by `controllers.SetImportClock`) because their methods are many and long-lived. The gocron scheduler still uses the wall clock for when jobs start (documented in ADR 0009). The browser does not follow `FAKE_NOW`.

- Baseline via ESLint bulk suppressions, not per-file `off` entries: a file's existing violations are counted, so new ones in the same file fail. `eslint-suppressions.json` is in `.prettierignore` (the tool writes it).

- ESLint 9 vs `react-scripts` 5 (bundles ESLint 8): yarn nests ESLint 8 under react-scripts, but its `eslint-webpack-plugin` would pick the hoisted ESLint 9, so `start` and `build` set `DISABLE_ESLINT_PLUGIN=true` and the `eslintConfig` block in package.json is removed. Lint runs only through `make lint-frontend`. The build itself is unchanged.
- `react-hooks` v7 `recommended` adds React Compiler rules; only `rules-of-hooks` and `exhaustive-deps` (what `react-app` had) are enabled.
- `no-confusing-void-expression` auto-fix changed ~250 arrow handlers to block bodies (behaviour unchanged).

- CI sets `YARN: yarn` (runner yarn) because the Makefile default `corepack yarn` needs a `packageManager` field.
- golangci-lint job uses the action with `install-only: true`, then `make lint-go` (adds `go vet`); loses the action's lint cache.

- `test-coverage` is not in `precommit` yet (step 8 adds thresholds first).

## For review

- Step 7: `frontend/eslint-suppressions.json` hides 529 findings (mostly `no-confusing-void-expression` fixed; remaining: restrict-template-expressions 91, no-misused-promises 90, no-floating-promises 65, no-unsafe-* ~140, no-unnecessary-condition 61, no-deprecated 34). Later steps should shrink it. `no-autofocus` is exempted as a design choice to confirm.

- Step 4: CI cannot run locally. Check the first PR run: `install-only` of golangci-lint-action v8, `YARN: yarn`, `make fuzz` env vars.

## Questions

- One full `make precommit` run during step 8 had 2 E2E failures (52 passed); a rerun and a second full run passed 54/54. Names not captured. Possible flake; watch for it.

- `new Date().toISOString().slice(0, 10)` (and the older `.split('T')[0]`) gives the UTC date, so default dates in AddNote/AddActivity and the reminder `min` can show the wrong day near midnight. Pre-existing; not fixed.

- `frontend/package.json` has no `packageManager` field, so `corepack yarn` (Makefile default) needs CI to set `YARN=yarn`. Adding the field would remove that workaround; not done (out of scope).

- `apperrors.ErrNotFound(resource)` appends " not found", so `backend/services/import_session.go:96,100,107` and `monica_import_session.go:132,135,149` produce "Import session expired or not found not found". Not fixed (out of scope).

## Blocked
