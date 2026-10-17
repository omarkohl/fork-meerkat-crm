---
title: '0005. Every query is scoped by user_id'
parent: ADRs
nav_order: 5
---

# 0005. Every query is scoped by user_id

- Status: Accepted
- Date: 2026-10-17

## Context

Meerkat CRM is multi-user. A query without a user filter leaks data between users.

## Decision

Every table that holds user data has a `user_id` column, or is reached only through a parent that
has one. The exceptions: `users` (the owner itself), `activity_contacts` (join table, reached
through `activities`), `webhook_deliveries` (through `webhooks`) and `job_executions` (system data,
no user data). Every query is scoped to the authenticated user's `user_id`. A new resource gets a
case in `frontend/e2e/isolation.spec.ts` or in a Go test.

## Consequences

A forgotten filter is a security bug. Reviews check it; an invariant test that enumerates models is
planned.
