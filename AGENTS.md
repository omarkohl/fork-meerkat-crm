# Agent rules

Index of the rules agents break most. Details are in the linked files. For autonomous
multi-step runs read [.claude/AUTONOMOUS-RUNS.md](.claude/AUTONOMOUS-RUNS.md) first.

**These rules override consistency with the surrounding code.** If old code breaks a
rule, do not copy it.

## Docs

- [Architecture](docs/development/architecture.md), [backend](docs/development/backend.md),
  [frontend](docs/development/frontend.md), [testing](docs/development/testing.md),
  [contributing and release](docs/development/contributing.md)
- [Decisions (ADRs)](docs/adr/README.md): read them before you re-open a decision.
  A significant new decision gets an ADR in the same commit.
- Docs change in the same commit as the code they describe.

## Hard limits

These are limits, not targets.

- Comments: at most one per ~50 new lines of code, two lines each. Adding no
  comment is a normal, correct outcome. Delete test: cover the comment; if the
  code says the same, delete it.
- Do the step asked, all of it, nothing beyond it. Unrelated findings go to
  `PROGRESS.md` under "Questions" (in a run) or into your report.

## Failure modes (forbidden)

- Comments that narrate the code, and doc comments that repeat the types.
- Changelog entries that tell a story.
- E2E tests that build URLs with internal IDs or replay raw API payloads.
- Tests that assert internal calls instead of user-visible results.
- Editing an existing migration.
- A query without the `user_id` scope.
- A new UI string that is not in all five locales (en, de, it, es, fr).

## Bad and good

- Bad: ``await page.goto(`/contacts/${contact.ID}`)`` in
  `frontend/e2e/reminders.spec.ts`. It skips the user's path and needs an internal ID.
  Good: open the contact from the contact list by its visible name.
- Bad: `// ErrUnauthorized returns an unauthorized error` in
  `backend/errors/errors.go`. The comment repeats the name. Good: no comment.
- Bad: `c.JSON(http.StatusForbidden, gin.H{"error": "..."})` in
  `backend/middleware/admin.go`. Good: return
  `apperrors.ErrForbidden("API tokens cannot access admin endpoints")`
  ([ADR 0007](docs/adr/0007-apperror.md)).

## Tests

TDD is mandatory: write the failing test, run it and read the failure, implement the
minimum, run it green, refactor without touching the test. Exceptions: pure styling
and fully covered refactors. See [ADR 0003](docs/adr/0003-testing-strategy.md) and
[testing](docs/development/testing.md).

Before a commit run `make precommit`. Run `make test-e2e` when UI or API behaviour
changes.

## Version control

- Conventional commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`),
  subject at most 72 characters, body says why.
- One commit is one coherent change from the user's view (migration, model,
  controller, API client, UI, i18n and tests together). Each commit passes
  `make precommit`.
- Issue footer: `fixes #N` when the commit resolves the issue, `issue #N` when it is
  part of it.
- Review your own diff before you ask for review.
- Never push without explicit permission, each time.
- With jj, always pass the message and never open an editor: `jj commit -m "msg"`,
  `jj squash` (add `-m "msg"` to avoid the editor when both messages exist),
  `jj describe -m "msg"`. For diffs use `jj diff --git`. Never use `jj split`,
  `jj resolve` or other interactive commands without arguments.
