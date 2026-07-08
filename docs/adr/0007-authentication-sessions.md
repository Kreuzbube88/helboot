# ADR-0007: Local authentication with server-side sessions, OIDC-ready

- **Status:** Accepted
- **Date:** 2026-07-08

## Context

The specification requires local users, additionally OIDC, and an
architecture open for further identity providers. Roles are fixed:
Administrator, Operator, Viewer. The UI is a browser SPA served from the
same origin as the API.

## Decision

- **Local accounts** are the baseline: usernames + **Argon2id** password
  hashes (OWASP parameters), created in the first-run wizard.
- **Server-side sessions** stored in SQLite: random 256-bit tokens in an
  HttpOnly, SameSite=Lax cookie (Secure when TLS is active), with idle
  and absolute expiry. Mutating requests additionally require the
  session's CSRF token in `X-CSRF-Token`.
- Authentication is abstracted behind an `IdentityProvider` interface;
  the local provider is the first implementation, **OIDC** the second
  (post-scaffold). Role mapping from OIDC claims is configuration.
- Authorization is a middleware concern: each API route declares its
  minimum role (`viewer` < `operator` < `admin`).

Alternatives considered:

- **JWTs:** stateless, but revocation and logout become messy; a
  single-node app gains nothing from statelessness.
- **Basic auth:** no sessions, no CSRF story, poor UX.

## Consequences

- Logout and session revocation are trivial (delete row).
- Session storage adds one hot table to SQLite — negligible at this scale.
- OIDC integration will be a new identity provider implementation plus an
  ADR for its configuration surface.
