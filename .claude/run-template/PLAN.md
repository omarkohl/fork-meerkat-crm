# Run plan: <name>

Goal: <one paragraph; link the issue or requirements doc>. Every step leaves `main`
deployable.

Read first: `.claude/AUTONOMOUS-RUNS.md`, `docs/development/*.md`, <docs and files>.

## Settled before the run (the user, <date>)

- <decision>

## Settled by Claude (the user may override before launch)

- <decision>

## Steps

- [ ] 0. **Environment check** (no own commit; record in PROGRESS.md with step 1):
      `make precommit` green, whether `make test-e2e` runs. A failure that blocks
      every step goes under `## Blocked`.
- [ ] 1. **<title>**: <what, end-to-end; which tests>.
- [ ] N. **Docs**: README, `docs/`, `docs/api-reference.md` as needed. Full
      `make precommit` and `make test-e2e`.
- [ ] N+1. **Finish**: summary and the "For review" list in PROGRESS.md (copied into
      that PR's body), then delete `.claude/run/` in its own commit.
