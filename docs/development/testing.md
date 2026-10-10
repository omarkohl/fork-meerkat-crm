---
title: Testing
parent: Development
nav_order: 4
---

# Testing

## Backend Tests

### Running Tests

```sh
cd backend
go test ./...
```

CI runs `go test -race -coverprofile=coverage.out ./...` (the race detector needs cgo and a C compiler; the SQLite driver itself is pure Go). Run the same locally to reproduce a race, and `go tool cover -func=coverage.out | tail -1` for the total.

### Coverage

The Unit Tests workflow prints the total coverage of the Go and Vitest suites in the job summary and uploads the full reports (`go-coverage`, `frontend-coverage`) as workflow artifacts for 14 days. No third-party service is involved. Locally: `cd frontend && yarn test:coverage` writes `frontend/coverage/`.

### Migrations

`TestMigrationsRoundTripStepByStepOnSeededDatabase` applies all migrations, fills the main tables, then rolls back one migration at a time (no table may lose rows, the schema of every version is recorded), checks the database is empty, and applies the migrations one at a time again, comparing each version's schema with the recorded one. `TestMigrationsUpKeepsRowsSeededAtOlderVersions` seeds databases at older versions and checks that the rows survive the migrations up to the newest. Every new migration needs a working `down`, or these tests fail. Add a statement for a new table to `seedStatements`. The one known exception is the down migration of 000008, which leaves three `contacts` columns behind (SQLite limitation); the test ignores them below version 8.

### CardDAV integration (Radicale)

The contact sync tests (and the other tests of `./services/` that use the same harness) also run against a real Radicale server when `MEERKAT_CARDDAV_IT=1` is set; CI does this in the `CardDAV sync (Radicale)` job. Locally: `docker compose -f docker-compose.carddav-test.yml up -d --wait radicale`, then `cd backend && MEERKAT_CARDDAV_IT=1 go test ./services/ -v`. The Radicale image tag is pinned in the compose file and in `unit-tests.yml`; change both together.

### Test Database

Tests use an in-memory SQLite database that is auto-migrated before each test. No external dependencies required.

### Writing Tests

Use the test helpers in the `_test` files alongside each package. Set up a test DB with `database.NewTestDB()` (or equivalent), create a Gin test context, and call controller functions directly. Assert on the `httptest.ResponseRecorder`.

## Frontend Tests

There are no isolated frontend tests with yarn test, since this requires code/logic duplication for mocking. Instead Playwright is used to run integrated E2E tests.

### Setup

```sh
cd frontend
yarn playwright install  # install browsers (first time only)
```

The E2E tests expect the full application running on `http://localhost:7300`. `global-setup.ts` seeds a test user before the suite runs.

### Running E2E Tests

```sh
yarn test:e2e           # headless
yarn test:e2e:headed    # with browser visible
yarn test:e2e:ui        # Playwright UI mode
yarn test:e2e:debug     # debug mode
```

### Writing E2E Tests

Tests live in `frontend/e2e/`. Use the shared fixtures from `fixtures.ts` for login/logout helpers and the `TEST_USER` credentials from `global-setup.ts`. Group tests with `test.describe` and keep each test independent.
