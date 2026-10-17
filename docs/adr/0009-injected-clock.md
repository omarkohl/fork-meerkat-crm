---
title: '0009. Injected clock'
parent: ADRs
nav_order: 9
---

# 0009. Injected clock

- Status: Accepted
- Date: 2026-10-17

## Context

Hidden inputs make tests flaky and behaviour impossible to preview. The backend called `time.Now()` in 84 places, so a birthday, a reminder or the trash purge could only be tested by editing data, and a demo could not be walked through time.

## Decision

Business code never reads the wall clock. It receives `now` as a value.

- **Go.** The package `backend/clock` is the only code that reads the wall clock (`Real`, `Fake` for tests, and the clock built from `FAKE_NOW`). Middleware takes the time once per request (`middleware.ClockMiddleware`); handlers read it with `middleware.RequestNow(c)`. The scheduler passes the clock to each job. Request-scoped work takes `now time.Time` as a parameter. Work that does slow I/O or handles many items (outbox deliveries, calendar and contact sync, reminders) takes the `clock.Clock` instead and reads it where it stamps something: a time taken at the start of a long batch is stale after the first slow call (a claim lock that looks old, a retry scheduled from the wrong moment). "Once" applies to a decision (which day is it), not to per-item side effects. Long-lived objects that cannot take it per call (the import session managers) hold a `clock.Clock` set at startup. GORM's `NowFunc` uses the same clock, so `created_at` agrees with everything else. JWT expiry is judged by the request clock.
- **Lint.** `forbidigo` (with `analyze-types`, so an aliased import is caught too) bans `time.Now` everywhere except `main.go`, `clock/`, `middleware/`, `metrics/` and tests. The message links this ADR. A line that must read the real clock (an I/O deadline, a duration measurement) carries `//nolint:forbidigo // reason`.
- **Fake clock.** `FAKE_NOW=2026-12-24T12:00:00Z` starts the clock at that instant and lets it run. It is an explicit setting, not tied to the build mode, and the server logs a warning when it is set. It is for demos and manual tests, never for a real instance. While it is set the purge jobs (trash, outbox, sessions, audit log) are skipped, because a clock set into the future would delete data for good. Two-factor codes are always checked against the real time (authenticator apps know no other).
- **Frontend.** `src/utils/now.ts` is the one place that calls `new Date()`; ESLint bans `new Date()` without arguments and `Date.now()` elsewhere in `src`. The browser does not follow `FAKE_NOW`: dates it computes (default values of date fields, "overdue") use the browser clock.

## Consequences

Time-dependent behaviour is tested by passing a time. Functions that used to read the clock gained a `now` parameter. The scheduler itself (gocron) still schedules with the wall clock, so a fake clock moves what jobs do, not when they start. Under `FAKE_NOW`, timestamps the server stores use the fake time while remote systems (a CardDAV server's `REV`, a webhook receiver) use the real one; contact sync's newest-wins comparison therefore mixes the two. That is acceptable for a demo clock and is why it must not be used on a real instance. `time.Since` and `time.Until` are not banned: they measure durations.
