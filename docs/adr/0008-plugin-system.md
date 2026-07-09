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

## Amendment (2026-07-09): contract, load locations, UI question

The original decision left three questions open; they are answered
here.

### What may a plugin extend?

| Extension surface | Plugin kind | v1 mechanism |
| ----------------- | ----------- | ------------ |
| New operating system / OS version | **Provider** (data plugin) | YAML manifest + templates + settings schema (ADR-0005, ADR-0012); no code |
| New boot method (VM, Redfish, IPMI, …) | `plugin.BootMethod` | Go interface, compile-time registration |
| New auth provider (OIDC, LDAP, …) | `plugin.IdentityProvider` | Go interface, compile-time registration |
| New answer-file format | `plugin.AnswerFileRenderer` | Go interface, compile-time registration |
| Post-v1: notifications, software repositories | future interfaces | new ADR when they land |

Anything not in this table — middleware, storage engines, arbitrary
API routes — is deliberately **not** a plugin surface.

### UI extension points: no

Plugins do **not** ship UI code. The frontend renders everything from
data the backend serves: capabilities decide which options exist
(ADR-0005), the settings schema generates the profile form (ADR-0012).
That declarative surface *is* the UI extension mechanism. Injectable
frontend code (module federation, iframes) would break the i18n rule,
the security model and offline packaging for marginal gain; if a
future plugin genuinely needs custom UI, that requires its own ADR.

### Contract and load locations

- The minimal contract is the interface set in
  `backend/internal/plugin` — construction from a config value, a
  stable `Name()`, and the interface methods. Registration happens in
  `cmd/helboot` wiring at compile time; there is no init()-magic or
  global registry to keep implementations testable.
- **Providers** (the only runtime-loadable plugin kind) are loaded from
  two locations, later wins on name collision:
  1. the shipped `providers/` directory in the image,
  2. `/data/providers/` on the data volume — users drop in new or
     patched providers without rebuilding the image.
  A manifest that fails validation disables only that provider.
- Go-interface plugins are compiled in; their *selection* (e.g. which
  identity provider is active) is configuration, not code.
