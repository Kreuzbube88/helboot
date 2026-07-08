# ADR-0002: Go for the backend

- **Status:** Accepted
- **Date:** 2026-07-08

## Context

The backend must implement low-level network protocols (DHCP, ProxyDHCP,
TFTP), serve large files efficiently (ISOs over HTTP range requests), run
as a single small Docker image on homelab hardware (often low-power), and
remain maintainable for years. The project specification prefers Go and
requires alternatives to be justified.

## Decision

The backend is written in **Go**.

- Static single binary → one-process container, trivial multi-arch builds
  (amd64/arm64), no runtime dependencies.
- First-class concurrency for running DHCP/TFTP/HTTP services in one
  process.
- Mature ecosystem for the exact protocols we need (e.g. iPXE ecosystem
  tooling, `insomniacslk/dhcp`, `pin/tftp`) and excellent standard-library
  HTTP.
- Strong static typing and tooling (`gofmt`, `go vet`, `govulncheck`)
  supports the project's quality rules.

Alternatives considered:

- **Python** (netboot.xyz-style tooling): simpler protocol prototyping,
  but heavier images, slower file serving, weaker typing for a long-lived
  codebase.
- **Rust:** excellent performance/safety, but a smaller contributor pool in
  the homelab space and slower iteration for this project's scope.
- **Node.js:** would unify language with the frontend, but is a poor fit
  for raw UDP/DHCP protocol work and large-file serving.

## Consequences

- Backend code lives in `backend/` as one Go module.
- CI enforces `gofmt`, `go vet`, tests with race detector, and
  `govulncheck`.
- Contributors need Go ≥ 1.24.
