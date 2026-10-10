# Progress

## Next step

3.

## Stack

- 1: second-sprint/01-ci-fixes (base: first-sprint/12-finish, PR #15)

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

## Decisions

- There is no admin "disable user" feature, so only deletion is covered. One extra `COUNT` query per
  authenticated request (primary key lookup; fine for SQLite).
- Claim `user_id` is accepted only as a whole number float64 in 1..MaxUint32 (what JSON decoding of
  our own tokens yields); other types are refused as invalid tokens.

## For review

- Step 2: every authenticated request now does a user lookup; confirm that cost is acceptable.

## Questions

## Blocked
