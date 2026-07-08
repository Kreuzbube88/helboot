# ADR-0004: SQLite as the database

- **Status:** Accepted
- **Date:** 2026-07-08

## Context

HELBOOT stores users, sessions, hosts, profiles, installation history and
settings. The deployment target is a single Docker container on homelab
hardware; the specification mandates SQLite with migrations, backup and
restore.

## Decision

**SQLite** is the only supported database, accessed through the pure-Go
driver `modernc.org/sqlite` (no cgo → static binary, painless
cross-compilation for amd64/arm64).

- Schema changes are **embedded, numbered SQL migrations** applied
  automatically at startup and tracked in a `schema_migrations` table.
- WAL mode with a single writer connection; the API's data volume is the
  only state.
- Backup uses SQLite's online-backup mechanism into the export archive;
  restore replaces the database atomically.

Alternatives considered:

- **PostgreSQL:** overkill for a single-node homelab appliance and breaks
  the one-container promise.
- **mattn/go-sqlite3 (cgo):** faster, but complicates cross-compilation
  and static builds; performance is not a bottleneck at homelab scale.
- **BoltDB/Badger:** no SQL, harder ad-hoc querying, weaker migration
  story.

## Consequences

- No connection-string configuration burden for users.
- Concurrency is bounded by SQLite's single-writer model — acceptable at
  homelab scale, mitigated with WAL and short transactions.
- A future multi-node story would require a new ADR.
