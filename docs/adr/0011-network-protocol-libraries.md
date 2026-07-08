# ADR-0011: Network protocol libraries and iPXE distribution

- **Status:** Accepted
- **Date:** 2026-07-08

## Context

ADR-0009 decided that HELBOOT owns the boot protocols in-process instead
of wrapping dnsmasq. That requires DHCP and TFTP implementations, and
clients need iPXE binaries to bootstrap from.

## Decision

- **DHCP/ProxyDHCP:** `github.com/insomniacslk/dhcp` — the de-facto
  standard Go DHCPv4 library (used by CoreDHCP and others), BSD-licensed,
  pure Go. HELBOOT implements the handlers itself: a ProxyDHCP responder
  (ports 67 + 4011) for Mode A and an authoritative server with a
  persisted lease pool for Mode B.
- **TFTP:** `github.com/pin/tftp/v3` — mature, MIT-licensed, supports
  the block-size negotiation PXE firmware needs. Read-only handler with
  path-traversal protection.
- **iPXE binaries** (`undionly.kpxe`, `ipxe.efi`, …) are **not committed
  to the repository** (they are GPLv2 build artifacts, ~1 MB each, and
  should be updatable independently). They live in the data volume under
  `assets/tftp/`. The Docker image downloads them at build time
  (`deploy/scripts/fetch-ipxe.sh`); bare-metal users run the same script.
  A missing binary produces a clear log hint, never a crash.

## Consequences

- Full control over per-MAC boot behavior without generating config
  files for an external daemon.
- The DHCP state machine is our correctness burden — covered by unit
  tests on the reply builders and lease pool.
- Image builds need network access to fetch iPXE; offline builds can
  pre-populate `assets/tftp` instead.
