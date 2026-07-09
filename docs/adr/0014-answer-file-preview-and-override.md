# ADR-0014: Answer-file templates per provider, preview and manual override

- **Status:** Accepted
- **Date:** 2026-07-09

## Context

Answer files (autounattend.xml, preseed, kickstart, autoinstall,
AutoYaST, answer.toml, dietpi.txt) are generated from Go
`text/template` files that live **in each provider's directory**
(`providers/<name>/templates/`), never in core — new OS versions are
added as data (ADR-0005). Template values come from the profile's
config document (whose shape ADR-0012 declares) plus boot-time
parameters (URLs, MAC, hostname) injected by the boot chain; boot
parameters win on key conflicts so a profile cannot break the chain.

What was missing per specification: the generated file must be
**inspectable before use** and **manually overridable** — for expert
tweaks the schema does not cover, and for debugging.

## Decision

1. **Preview endpoint.** `GET /profiles/{id}/versions/{version}/answer`
   returns the answer file exactly as the boot chain would render it,
   with clearly recognizable placeholder boot parameters
   (`http://<helboot>/boot/...`, MAC `52:54:00:00:00:01`) so the user
   sees real structure, not secrets of a live installation.
2. **Override per profile version.** The user can store an edited
   answer file next to a version
   (`PUT /profiles/{id}/versions/{version}/answer-override`); an empty
   body clears it. While an override exists, the boot chain renders the
   *override* instead of the provider template — through the same
   template engine, so `{{ .ISOURL }}`-style boot parameters keep
   working inside overrides. The preview shows the override when one is
   set and flags it as such.
3. Overrides live on the version row (immutable config + mutable
   override side channel). Regeneration is "clear the override". They
   affect only future boots; HELBOOT does not retroactively store the
   byte-exact file of past installations.

Alternatives considered: storing overrides per installation (too late —
the user wants to review *before* queueing) and rendering previews with
live host data (leaks per-installation tokens into a non-boot context).

## Consequences

- The chain profile → template → generated file is transparent: what
  you preview is what the installer fetches.
- Expert users can express anything the installer supports, at the cost
  that an override no longer follows later config or template changes —
  the UI labels overridden versions explicitly.
- Overrides are templates themselves; a syntax error surfaces at boot
  time in the logs. The preview endpoint renders with the same engine,
  so checking the preview catches such errors beforehand.
