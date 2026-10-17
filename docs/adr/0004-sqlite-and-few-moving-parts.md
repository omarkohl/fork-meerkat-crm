---
title: '0004. SQLite with WAL and few moving parts'
parent: ADRs
nav_order: 4
---

# 0004. SQLite with WAL and few moving parts

- Status: Accepted
- Date: 2026-10-17

## Context

Every extra service is one that an agent can misconfigure and a test must start. Meerkat CRM is a self-hosted app for small installs.

## Decision

The database is embedded SQLite, opened by the Go backend with `journal_mode(WAL)`, `busy_timeout(5000)` and `synchronous(NORMAL)` (see `backend/database/migrate.go`). The deployment is one all-in-one image: nginx serves the SPA and proxies `/api/` to the backend. No separate database, queue or cache service.

## Consequences

Simple backup and tests on a real database. One writer at a time, so no horizontal scaling of the backend. Adding a service needs a new ADR.
