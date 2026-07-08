# ADR-0009: One Docker image, one container, one supervising process

- **Status:** Accepted
- **Date:** 2026-07-08

## Context

The specification requires exactly one Docker image and one container for
the user (§18), while HELBOOT internally needs several services: HTTP
(API + UI + boot files), TFTP, DHCP/ProxyDHCP.

## Decision

- One multi-stage Dockerfile produces a single image: stage 1 builds the
  frontend (Node), stage 2 builds the Go binary embedding the frontend's
  static assets (`go:embed`), the final stage is a minimal base image with
  just the binary, iPXE boot assets, and provider data.
- The **Go binary is the supervisor**: DHCP, TFTP and HTTP run as
  goroutine-based services managed by an internal service manager with
  health states, restart-on-failure and clean shutdown. No s6/supervisord
  layer — one PID 1, one log stream, simpler images.
- All state lives in a single `/data` volume (database, ISOs, generated
  assets, logs, secrets).

Alternatives considered:

- **docker-compose with separate containers** (dnsmasq + app): violates
  the one-container requirement and complicates Unraid templates.
- **s6-overlay + dnsmasq:** battle-tested DHCP/TFTP, but external process
  configuration (config file generation, log multiplexing) is more
  fragile than owning the protocols in-process, and Go libraries for
  DHCP/TFTP are mature.

## Consequences

- Full control over PXE behavior (per-MAC boot scripts) without config
  file generation for a third-party daemon.
- We own protocol correctness — covered by integration tests against
  recorded DHCP/TFTP exchanges.
- Image stays small (static binary + assets) and multi-arch friendly.
