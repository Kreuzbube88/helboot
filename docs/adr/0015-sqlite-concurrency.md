# ADR-0015: SQLite concurrency — WAL and a single writer

- **Status:** Accepted
- **Date:** 2026-07-09

## Context

SQLite (ADR-0004) allows many concurrent readers but only one writer.
HELBOOT has several potential writers: API handlers, the boot chain
(installation state transitions), device discovery, the DHCP lease pool
and the backup exporter. Uncoordinated write access risks
`SQLITE_BUSY` errors and, with multiple connections, writer starvation.

## Decision

1. **WAL mode: yes.** The database is opened with
   `journal_mode=WAL`. Readers never block the writer and vice versa,
   and crash recovery is robust. The `-wal`/`-shm` sidecar files live
   in the data volume next to the database.
2. **Single-writer pattern, enforced by construction.** The connection
   pool is capped at **one connection** (`SetMaxOpenConns(1)`), and all
   components — API, boot chain, discovery, lease persistence — go
   through the one shared `store.Store` on top of it. No internal
   process opens its own database handle; anything that cannot call the
   store directly must go through a service interface that does (e.g.
   the lease pool persists through a store-backed adapter). External
   processes never touch the database file; the only write path into
   HELBOOT state is the REST API and the internal services behind it.
3. **Timeout instead of hand-rolled retry.**
   `busy_timeout=5000` makes SQLite itself retry a locked database for
   up to 5 s before surfacing `SQLITE_BUSY`. Combined with the
   single-connection pool (Go serializes all access, so intra-process
   lock conflicts cannot occur) the only realistic `SQLITE_BUSY` source
   is the online-backup reader, which WAL tolerates. Application-level
   retry loops are deliberately *not* added: they would hide design
   errors that reintroduce a second writer.
4. **Transactions stay short.** Multi-statement invariants (profile +
   first version, version append + head bump) run in one transaction;
   nothing holds a transaction across network or file I/O.

Trade-off accepted: one connection serializes *reads* too. At homelab
scale (a handful of concurrent users, tens of hosts) this is measured
in microseconds and irrelevant; if it ever becomes a bottleneck, the
next step is a read pool (WAL permits n readers + 1 writer), which is
a config change, not a redesign — but it needs a new ADR because it
reintroduces `SQLITE_BUSY` handling.

## Consequences

- Lock conflicts are structurally impossible inside the process; the
  busy timeout covers the rest.
- Every new subsystem must accept the `store.Store` (or an interface
  backed by it) instead of opening the database — reviewers reject
  direct `sql.Open` calls outside `internal/db`.
- Backup/restore keeps using SQLite's online backup API against the
  same connection discipline (ADR-0004).
