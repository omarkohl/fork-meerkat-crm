---
title: '0008. Five locales for every UI string'
parent: ADRs
nav_order: 8
---

# 0008. Five locales for every UI string

- Status: Accepted
- Date: 2026-10-17

## Context

Users read the app in several languages. A string in one language only is a bug.

## Decision

Every new UI string exists in all five locales: en, de, it, es, fr, in `frontend/src/i18n/locales/`.
Backend strings (for example email notifications) go in `backend/i18n/locales/` in the same five.

## Consequences

Each text change costs five translations. No fallback language gaps.
