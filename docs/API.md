# HELBOOT API

The REST API is the only interface between the frontend and the backend,
and the intended surface for automation. The authoritative contract is
the OpenAPI 3.1 document at
[`backend/api/openapi.yaml`](../backend/api/openapi.yaml), served by a
running instance at `/api/v1/openapi.yaml`.

## Conventions

- Base path: `/api/v1`, JSON request/response bodies.
- **Authentication:** session cookie (`helboot_session`), obtained via
  `POST /api/v1/auth/login`. Mutating requests (POST/PUT/PATCH/DELETE)
  must send the session's CSRF token in the `X-CSRF-Token` header
  (returned by login and `GET /api/v1/auth/me`).
- **Roles:** `admin` > `operator` > `viewer`. Each endpoint documents its
  minimum role.
- **Errors:** single envelope, `code` is a stable identifier that doubles
  as the i18n key:

```json
{ "error": { "code": "auth.invalid_credentials", "message": "Invalid username or password" } }
```

## Endpoint groups (v1)

| Group | Examples | Purpose |
| ----- | -------- | ------- |
| Health | `GET /health` | Liveness/readiness (unauthenticated) |
| System | `GET /system/info` | Version, mode, uptime |
| Setup | `GET /setup/status`, `POST /setup` | First-run wizard (only until completed) |
| Auth | `POST /auth/login`, `POST /auth/logout`, `GET /auth/me` | Sessions |
| Providers | `GET /providers`, `GET /providers/{name}` | Capability manifests for the UI |
| ISOs | `GET/POST /isos`, `GET /isos/{id}` | ISO library and analysis results |
| Profiles | CRUD `/profiles`, `POST /profiles/{id}/versions`, `/clone`, `/export` | Installation profiles with versioning |
| Hosts | CRUD `/hosts` | MAC-registered machines, discovery inbox |
| Installations | `GET/POST /installations` | Installation queue and history |
| Network | `GET/PUT /network/config` | Mode A/B configuration |
| Backup | `GET /backup/export`, `POST /backup/import` | Full state export/import |
| Logs | `GET /logs` | Structured log access for the UI |

## Boot surface (not part of the JSON API)

Endpoints consumed by firmware and iPXE live under `/boot/` and are
unauthenticated by protocol necessity (see
[ADR-0010](adr/0010-api-first-rest-openapi.md) and the security concept
in [ARCHITECTURE.md](ARCHITECTURE.md)): iPXE scripts per MAC, kernels,
initrds, ISO content, generated answer files scoped to an active
installation.
