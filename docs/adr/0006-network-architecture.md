# ADR-0006: Network architecture — ProxyDHCP first, optional full DHCP

- **Status:** Accepted
- **Date:** 2026-07-08

## Context

Homelab networks almost always already have a DHCP server (router,
FRITZ!Box) that users cannot or do not want to modify. PXE, however,
requires DHCP options pointing at a boot server. HELBOOT must work in
both worlds (specification §17).

## Decision

HELBOOT implements two mutually exclusive modes, chosen in the first-run
wizard and changeable in settings:

- **Mode A — ProxyDHCP (default):** HELBOOT never assigns IP addresses.
  It listens for PXE DHCP discovery broadcasts and answers *in addition
  to* the existing DHCP server, supplying only boot options (PXE service
  on UDP 67/4011). This coexists safely with any router.
- **Mode B — authoritative DHCP:** HELBOOT runs a full DHCP server with a
  configurable range, for networks without DHCP or labs on an isolated
  segment.

In both modes HELBOOT provides TFTP (initial firmware fetch: iPXE
binaries chosen by DHCP architecture option 93) and HTTP (iPXE scripts,
kernels, ISO content, answer files). All post-iPXE traffic uses HTTP —
TFTP is only the minimal firmware bootstrap.

The container therefore **requires host networking** for Mode B and
recommends it for Mode A, because broadcast/raw UDP does not traverse
Docker's default bridge NAT. macvlan / Unraid `br0` work where the
platform delivers broadcasts to the container and are documented with
their limitations rather than promised unconditionally.

## Consequences

- Works out of the box next to a FRITZ!Box — the primary user story.
- Two DHCP code paths must be tested; the shared PXE-option encoder is a
  single component used by both.
- Port conflicts (67/69/80) with other host services are a documented
  operational concern; ports are configurable.
