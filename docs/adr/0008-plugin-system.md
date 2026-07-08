# ADR-0008: Plugin system — extension points first, runtime later

- **Status:** Accepted
- **Date:** 2026-07-08

## Context

The specification demands a plugin concept and an architecture that stays
extensible for years (boot methods, VM provisioning, Redfish/IPMI,
software/driver management). Shipping a full dynamic plugin runtime in
v1 would freeze immature interfaces.

## Decision

Version 1 ships **well-defined extension points**, not a dynamic plugin
runtime:

1. **Providers** (ADR-0005) are the first plugin type — pure data
   (YAML + templates), loadable from the data volume without recompiling.
2. Core defines Go registry interfaces for future plugin kinds:
   - `BootMethod` (PXE, iPXE, HTTP boot, USB image — later: VM
     provisioning, Redfish, IPMI)
   - `IdentityProvider` (local, later OIDC — ADR-0007)
   - `AnswerFileRenderer` (per answer-file format)
   - post-v1: notification sinks, software/driver repositories (§32)
3. Implementations register at compile time. Out-of-process or
   dynamically loaded plugins (e.g. via gRPC like HashiCorp's model) are
   deferred to a future ADR once interfaces have proven stable.

## Consequences

- Extensibility is real from day one (new provider = new files), while
  interface churn stays cheap (compile-time registration).
- Third-party binary plugins are not possible yet; this is an explicit,
  documented limitation.
- Interfaces live in dedicated packages so a future runtime can adopt
  them without breaking implementations.
