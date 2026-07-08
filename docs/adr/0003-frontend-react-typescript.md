# ADR-0003: React + TypeScript for the frontend

- **Status:** Accepted
- **Date:** 2026-07-08

## Context

HELBOOT needs a web UI with a first-run wizard, capability-driven dynamic
forms (providers declare what options exist), full internationalization,
and long-term maintainability. The specification prefers React +
TypeScript.

## Decision

The frontend is a **React + TypeScript** single-page application built
with **Vite**, using **react-i18next** for internationalization and
**react-router** for navigation. It communicates exclusively through the
REST API (ADR-0010) and is served as static assets by the Go binary —
keeping the one-container model.

Alternatives considered:

- **Vue/Svelte:** perfectly viable, but React has the largest contributor
  pool and the specification's preference stands without a strong reason
  to deviate.
- **Server-side rendering (HTMX/templ):** simpler deployment, but
  capability-driven dynamic forms and the wizard benefit from a typed
  client-side model, and the strict API-only rule keeps the API honest.

## Consequences

- All UI strings live in locale JSON files (`en`, `de` at minimum); ESLint
  configuration flags hardcoded JSX text.
- The OpenAPI spec is the typing contract for API calls.
- `npm run build` output is embedded/served by the backend container.
