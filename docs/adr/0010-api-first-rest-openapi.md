# ADR-0010: API-first — versioned REST with OpenAPI as the contract

- **Status:** Accepted
- **Date:** 2026-07-08

## Context

The specification requires a REST API with an OpenAPI specification, and
that the frontend communicates exclusively through this API (§27). The
API is also the future surface for automation (scripts, Home Assistant,
CLI tools).

## Decision

- All functionality is exposed under **`/api/v1`** as resource-oriented
  REST endpoints with JSON bodies. There is no privileged internal
  channel — the shipped UI uses the same API any script can use.
- The **OpenAPI 3.1 document** (`backend/api/openapi.yaml`) is the
  contract: maintained with the code, served at `/api/v1/openapi.yaml`,
  and used to type the frontend client. CI fails if the spec is invalid.
- Errors use one envelope — `{"error": {"code", "message"}}` — where
  `code` is a stable identifier doubling as the i18n key (§23: error
  messages must be translatable).
- Breaking changes require a new version prefix (`/api/v2`); `v1` is
  additive-only after the first stable release.
- Boot-time endpoints consumed by firmware/iPXE (boot scripts, images,
  answer files) live under `/boot/` outside the JSON API: they are
  unauthenticated by nature (ADR-0006) and follow protocol constraints,
  not REST conventions.

## Consequences

- Automation and the UI have identical power; API gaps surface
  immediately during UI development.
- Spec maintenance is a review obligation on every API PR.
- The unauthenticated `/boot/` surface is minimal, read-only and
  MAC-scoped, and is documented in the security concept.
