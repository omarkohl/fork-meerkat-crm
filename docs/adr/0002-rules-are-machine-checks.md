---
title: '0002. Rules are machine checks with fix messages'
parent: ADRs
nav_order: 2
---

# 0002. Rules are machine checks with fix messages

- Status: Accepted
- Date: 2026-10-17

## Context

AI agents write plausible code fast. Whether it is correct depends on what the machine checks. Agents follow error messages reliably and prose rules unreliably.

## Decision

Move every rule out of heads and prose into a check that the machine runs. The failure message of the check explains the fix, and links to the ADR where it helps. The checks are: strict types, strict linting with every rule an error, automatic formatting, architecture rules, tests. Humans, agents and CI run the same `make` targets, and `make precommit` is the one gate. A new check starts with a baseline of named exemptions that each have a reason; later steps remove them. Show that a new rule fails on a violation before you rely on it. Warnings do not exist: a rule is an error or it is off. Prose rules stay only where no check is possible.

## Consequences

Less review attention per change. Each new rule costs the work to write the check and its message. Exemptions must be tracked and removed.
