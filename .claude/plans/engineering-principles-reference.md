# Engineering principles: reference for the run

Source material for `.claude/plans/engineering-principles.md`, adapted to Meerkat CRM
(Go + Gin + GORM + SQLite backend, React + TypeScript frontend with Vitest,
Playwright E2E, GitHub Actions, Dependabot). The run moves each section into ADRs,
`AGENTS.md` and `docs/development/`; the last step of the run deletes this file.

## Overview

AI agents write plausible code fast. Whether it is _correct_ is decided by what the
machine checks. So:

> **Move every rule out of people's heads and prose documents into a check the
> machine runs, and make the check's failure message explain the fix.**

The more the build enforces, the less review attention each change needs.

| #   | Principle                          | In one sentence                                                                        |
| --- | ---------------------------------- | -------------------------------------------------------------------------------------- |
| 1   | One command, everywhere            | Humans, agents and CI run the same `make` targets; one `precommit` gate.               |
| 2   | Make wrong code fail to build      | Strict types, strict linting, auto-formatting, conventions as custom lint rules.       |
| 3   | Hexagonal architecture, tested     | Pure domain at the centre, adapters outside; the build fails on a wrong-way import.    |
| 4   | Test behaviour, not implementation | Integration tests on real SQLite carry the load; few user-like E2E tests; TDD.         |
| 5   | Design for determinism             | Inject time and other ambient state; few moving parts.                                 |
| 6   | Update dependencies continuously   | Daily, grouped, with a cooldown; the test suite makes it safe.                         |
| 7   | Small coherent commits             | One commit = one shippable change, message says why, issue linked.                     |
| 8   | Documentation lives with the code  | Docs and decision records in the repo, changed in the same commit.                     |
| 9   | Write instructions agents follow   | Hard limits, real bad/good examples, named failure modes.                              |

## 1. One command, everywhere

**Why:** If CI, humans and agents run different commands, "it passed for me" means
nothing, and agents guess at invocations.

- The root `Makefile` holds every routine task: `install`, `dev`, `build`, `format`,
  `format-check`, `lint`, `arch`, `typecheck`, `test`, `test-coverage`, `test-e2e`.
- One aggregate target runs everything a commit must pass:

  ```make
  precommit: format-check lint arch typecheck i18n-check test-coverage test-e2e
  ```

- CI calls the same targets, one job per check, so failures are easy to read. CI
  installs from the lockfile only (`yarn install --frozen-lockfile`, `go mod download`).
- Targets declare their prerequisites (`test: install`), so a fresh clone can run any
  target directly.
- A fresh clone reaches a running app with seed data in one or two commands
  (`make dev`, `make seed`).
- Document how to run **one** test by file, line and name (`go test -run`, `vitest
  <file> -t`, `playwright test <file>:<line>` / `--grep`). Agents iterate far faster
  on a single test than on the suite.
- Several checkouts work side by side (ports, data dir via `dev.env`): agents often
  work in parallel worktrees.

## 2. Make wrong code fail to build

**Why:** Agents follow error messages reliably and prose rules unreliably.

### Types, linting, formatting

- **Strict typing**: TypeScript `strict`, `noUncheckedIndexedAccess`; Go's compiler
  plus golangci-lint. Types document signatures; no doc comments that repeat them.
- **Type-aware linting** with a strict preset (`typescript-eslint`
  `strict-type-checked`; golangci-lint). Every rule is an error; warnings get
  ignored by everyone.
- **Automatic formatting** (gofmt, Prettier) checked in CI (`format-check`) on all
  files. Style never appears in a review.

### Project conventions as rules

Any convention that matters gets a check. A linter sees one file at a time: use it
for banned calls, banned syntax and required shapes (ESLint `no-restricted-syntax`,
golangci-lint `forbidigo`). Rules about the import graph are in §3.

The message is written for the person, or agent, who broke the rule: what to do
instead, and which ADR explains why:

```yaml
# backend/.golangci.yml
forbidigo:
  forbid:
    - pattern: ^time\.Now$
      msg: "time.Now reads an ambient clock. Take `now` from the request context and pass it down (ADR 00NN)."
```

```js
// frontend ESLint
{
  selector: "NewExpression[callee.name='Date'][arguments.length=0]",
  message: "`new Date()` reads an ambient clock. Use the `now` helper (ADR 00NN).",
}
```

Rules for rules:

- **Exempt a line, not a file**, and require a reason (`//nolint:gosec // reason`,
  `// eslint-disable-next-line <rule> -- <reason>`). A file-wide exclusion silently
  stops checking everything else in it.
- **Check that a rule's file pattern matches something.** A pattern that matches
  nothing looks exactly like one that works.
- **Generated artifacts are committed and checked**: CI regenerates them (OpenAPI
  spec, frontend API types) and fails on a diff.
- **Documentation links are validated** in CI.

## 3. Hexagonal architecture, tested

**Why:** Clear boundaries tell humans and agents where code belongs, keep business
rules testable without Gin, and let one rule serve every entry point (REST API,
CardDAV, scheduler jobs, imports). Boundaries checked by the build do not erode.

### The shape

Dependencies point **inward only**:

```
adapters (huma/Gin HTTP handlers, CardDAV, scheduler jobs, GORM persistence, mail, webhooks)
   └─> application (use cases; depend only on ports)
          └─> domain (entities, policies; pure, no I/O)
```

- **Domain**: entities, rules and policies. Imports only itself and the standard
  library. No Gin, no GORM, no I/O, no clock.
- **Application / use cases**: one function per user intention ("add a note to a
  contact"). Each declares its dependencies as **ports** (interfaces), takes the
  actor (user ID) and `now` as inputs, and runs its writes in one transaction (unit
  of work).
- **Adapters**: implement ports (GORM repositories, mailer) or call use cases (HTTP
  handlers, CardDAV, jobs). All entry points call the **same** use cases, so a
  business rule, including `user_id` scoping, is written once.
- **Composition root**: `main.go` builds the use cases from concrete adapters. Tests
  build them with a real SQLite test database.
- **Modules** (contacts, notes, reminders, …) each expose **one public entry
  package**; Go `internal/` directories make the compiler enforce this.
- **Types follow the same direction**: an entity type is domain vocabulary and lives
  in the domain. A GORM model or `*gorm.DB` stays inside the persistence adapter.
- **Contracts at the edge**: the OpenAPI spec is generated by huma from the typed
  request/response definitions and committed; frontend types are generated from it
  (see the generated-artifact check in §2).

### Automated architecture tests

`depguard` (Go, in golangci-lint) and `dependency-cruiser` (frontend) run in `make
arch`, `precommit` and CI. Typical rules:

| Rule                             | Forbids                                                                 |
| -------------------------------- | ----------------------------------------------------------------------- |
| no-circular                      | any import cycle (Go: compiler; frontend: dependency-cruiser)           |
| domain-stays-pure                | domain importing anything outside itself and the standard library       |
| use-cases-depend-on-ports        | application code importing Gin, huma, GORM or other adapter code        |
| modules-through-entry-package    | importing a module's internals from outside it                          |
| repositories-stay-in-persistence | anything outside the persistence adapter importing GORM or repositories |
| pages-use-hooks                  | frontend pages calling `src/api` directly instead of through hooks      |
| no-unresolvable                  | broken imports                                                          |

Each rule names what is forbidden and why, in its own message:

```yaml
# backend/.golangci.yml
depguard:
  rules:
    domain-stays-pure:
      files: ["**/domain/**"]
      allow: [$gostd, meerkat/domain]
      deny:
        - pkg: gorm.io/gorm
          desc: "The domain is pure: move persistence to an adapter and pass the result in (ADR 00NN)."
```

How to add a rule:

1. Write it and **check that it fails** on a file that violates it. A mis-scoped rule
   reports success and enforces nothing.
2. Fix the existing violations, or list them as named exemptions with a reason.
   Prefer fixing: an exemption granted at introduction tends to stay.
3. Put the reason in the rule's message, not only in a doc. The person who hits the
   rule reads the error, not the doc.

Further checks worth having:

- **Type-only imports count** (frontend `import type`). An erased import still points
  the wrong way.
- **Invariant tests** for rules that imports cannot express, e.g. a test that
  enumerates every mutating route of the Gin engine and fails if one is not
  authenticated and scoped to the user (see §4).
- A generated dependency graph often settles a boundary argument faster than a
  discussion.

## 4. Test behaviour, not implementation

**Why:** A test should fail when a user's outcome changes, and ideally only then.
Tests that mirror implementation break on every refactor and catch few real bugs.

### Tiers

| Tier            | Share    | Scope                                                                                                                                       |
| --------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| **Integration** | the bulk | Business logic through its real entry points (HTTP handlers, use cases) against a **real SQLite database**. Mock only what you cannot run locally (Resend, remote OIDC). |
| **E2E**         | few      | Important user workflows in Playwright. Quality over quantity.                                                                              |
| **Unit**        | rare     | Complex isolated functions only (vCard/CSV parsers, ICS folding). Never duplicate integration coverage.                                     |

Integration tests:

- Keep the real database cheap: migrate a template database once, copy it per test.
- Verify results through a read surface, in this order: the matching read endpoint,
  then repository reads, then raw rows as a last resort.
- Scheduler jobs do not run implicitly in tests; tests trigger them, then assert.

E2E tests:

- **Act like a user**: click visible elements, follow links, type into labelled
  fields. Use role, label and visible text locators, not CSS or test IDs.
- **Never construct URLs with internal IDs** (`/contacts/${id}`) and never replay raw
  API payloads.
- Tests that change global settings run separately (serial, after the parallel bulk).

### Quality guardrails

- A test that breaks on an internal rename with no user-visible change is a bad test:
  rewrite or delete it.
- No assertions on call counts of internal helpers; no reaching into unexported state.
- Factories build minimal valid entities; tests override only the fields they care
  about. If a test sets 12 fields, fix the factory.
- No shared state between tests.

### Making coverage visible and durable

- **Coverage thresholds** in CI (Go and Vitest) stop slow erosion. Raise them when
  coverage rises.
- **Use-case catalogue** (`docs/use-cases.yml`): user goals ("User adds a note to a
  contact"), grouped by feature, each with a permanent ID (`005-US1`), actor and
  priority. Retire an ID with `status: deprecated`; never renumber. Tests declare the
  IDs they cover (`test("adds a note @005-US1")`). The build fails on an unknown ID
  and on an E2E test without one. A report shows which goals no test covers.
- **Invariant tests that enumerate**: e.g. every model with `user_id` is covered by an
  isolation test. New code cannot forget the rule.

### Test what you ship

- Run the E2E suite **against the all-in-one Docker image** (CI does this). It catches
  path, packaging, upload and time-zone bugs (the container runs in UTC).
- **Upgrade tests**: at each release, store a seeded database from that version. A
  test migrates every stored version forward and exercises basic reads and writes.
  Users never start from an empty database.
- **No silent skips in CI**: tests that need optional services (Radicale) skip locally
  when unconfigured, but CI always configures them and fails if they are missing.

### TDD: mandatory for agents

1. Find or add the use case; write the failing test tagged with its ID.
2. Run it and **confirm the failure output**: this proves the test tests something.
3. Implement the minimum to pass.
4. Run again; confirm green.
5. Refactor without touching the test.

Narrow exceptions: pure styling changes, and refactors already fully covered. For
those, write no test at all (no tests that assert layout or spacing).

## 5. Design for determinism

**Why:** Hidden inputs make tests flaky and behaviour impossible to preview.

- **No ambient state in business logic.** Time, randomness and environment enter at
  the boundary (request middleware, scheduler job start) and are passed down. A lint
  rule bans the direct calls (`time.Now`, `math/rand`, `os.Getenv`) outside the
  boundary.
- An injected clock enables a **fake clock**: walk the system through time-dependent
  states (birthdays, reminders, trash purge) without editing data, in dev, in tests
  and on a demo instance. Gate it with an explicit flag, not the build mode.
- **Format dates in the user's time zone**, never the server's.
- **Keep few moving parts**: embedded SQLite, one all-in-one image. Every service you
  remove is one that an agent cannot misconfigure and a test does not need to start.

## 6. Update dependencies continuously

**Why:** Many small upgrades are cheap; one large upgrade after a year is a project.
Agents also work best with current, documented library versions.

- Dependabot opens PRs **daily**, **one grouped PR per ecosystem**.
- A **cooldown** (7 days after release) limits exposure to compromised or broken
  releases.
- Every ignored or pinned version carries a comment with the issue that blocks it.
- The test suite, especially E2E, is what makes merging these PRs safe.

```yaml
updates:
  - package-ecosystem: "gomod"
    directory: "/backend"
    schedule: { interval: "daily" }
    cooldown: { default-days: 7 }
    groups: { all-dependencies: { patterns: ["*"] } }
    ignore:
      # <module> X not yet compatible with Y v3 (issue #NN)
      - dependency-name: "Y"
        versions: [">= 3"]
```

## 7. Small coherent commits

**Why:** Coherent commits are reviewable, revertable and bisectable, for humans and
review agents.

- **One commit = one semantically coherent change**, end-to-end from the user's view:
  migration, model, controller, API client, UI, i18n and tests of one feature go
  together. Each commit passes `make precommit` and could ship alone.
- **Conventional-commit** prefixes (`feat:`, `fix:`, `refactor:`, `docs:` …); subject
  ≤ 72 characters; the body explains **why**, not what.
- **Issue footer**: `fixes #123` when the commit resolves the issue, `issue #123` when
  it is part of it.
- **Self-review the diff before requesting review.**
- **Never push without explicit permission**, each time. This applies to agents too.
- Document the version-control gotchas agents hit (jj: always pass `-m`, never open
  an editor; `--git` for diffs) with the exact non-interactive command.

## 8. Documentation lives with the code

**Why:** An agent only knows what is in the repository.

- **Developer docs (`docs/development/`), user docs (`docs/`) and ADRs (`docs/adr/`)
  live in the repo** and change **in the same commit** as the code they describe.
- **ADRs** for every significant decision: status, date, context, decision,
  consequences. Numbered, never deleted (superseded instead). Agents read them
  instead of re-opening decisions; lint messages link to them.
- **`AGENTS.md` is an index**, not an encyclopedia: the rules agents most often
  break, in short form, plus links to the detailed chapters.
- **Record gotchas where they bite.** When someone trips over the same thing twice,
  add one paragraph with the fix to the relevant chapter.
- **Changelog for users** (`CHANGELOG.md`): one bullet per change, issue reference,
  hard length limit. Implementation details go in the commit message. Internal
  changes get an entry only if they are a release highlight.

## 9. Write instructions agents follow

**Why:** Agents overshoot vague guidance and copy surrounding code, including its
worst parts.

- **Hard limits, not adjectives.** "At most one comment per ~50 lines of new code, two
  lines each" works; "keep comments sparse" does not. Add "these are limits, not
  targets".
- **Default to zero where applicable.** "Adding no comment is a normal, correct
  outcome."
- **Name the failure modes you observe** and forbid them: comments that narrate code,
  doc comments that restate types, changelog entries that tell a story, E2E tests
  that construct URLs.
- **Show bad and good examples taken from this codebase**, with a one-line reason
  each.
- **Give a test the agent can apply**, e.g. "the delete test: cover the comment; if
  the code says the same, delete it".
- **State that the rules override consistency with surrounding code**; otherwise
  legacy patterns spread.
- When a rule can become a lint check, make it one (§2, §3) and shorten the prose.
