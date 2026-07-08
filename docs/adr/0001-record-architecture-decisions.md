# ADR-0001: Record architecture decisions

- **Status:** Accepted
- **Date:** 2026-07-08

## Context

HELBOOT is intended to be a long-lived, professionally maintained
open-source project. Technical decisions need to be traceable years later,
by maintainers who were not present when they were made.

## Decision

Every significant technical decision is recorded as an Architecture
Decision Record (ADR) in `docs/adr/`, numbered sequentially, using this
format: Context, Decision, Consequences. ADRs are immutable once accepted;
a superseding decision gets a new ADR that references the old one.

## Consequences

- Contributors proposing significant changes include an ADR in their PR.
- The reasoning behind the stack and architecture stays discoverable.
