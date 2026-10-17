---
title: '0001. Record decisions as ADRs'
parent: ADRs
nav_order: 1
---

# 0001. Record decisions as ADRs

- Status: Accepted
- Date: 2026-10-17

## Context

An agent only knows what is in the repository. Decisions kept in chat, pull requests or heads get
re-opened or broken by accident.

## Decision

Every significant decision gets an ADR in `docs/adr/NNNN-slug.md` with status, date, context,
decision and consequences. ADRs are numbered and never deleted: a changed decision gets a new ADR,
and the old one is marked `Superseded by NNNN`. The ADR changes in the same commit as the code it
explains. Lint messages and agent rules link to the ADR. `README.md` in this folder is the index.

## Consequences

Agents and people read the ADR instead of re-opening the decision. Each new ADR needs a line in the
index.
