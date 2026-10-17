---
title: '0006. Never edit an existing migration'
parent: ADRs
nav_order: 6
---

# 0006. Never edit an existing migration

- Status: Accepted
- Date: 2026-10-17

## Context

Migrations run automatically on startup on installs of every version. Changing an applied migration makes databases differ.

## Decision

A schema change is a new numbered pair in `backend/database/migrations` (`up` and a working `down`), made with `make migrate-create NAME=xxx`. An existing migration is never edited. Tests round-trip all migrations and migrate databases seeded at older versions.

## Consequences

The history is append-only. A mistake gets a new migration that fixes it.
