# Security Policy

## Supported versions

HELBOOT is in early development; only the latest release receives security
fixes.

## Reporting a vulnerability

Please report vulnerabilities **privately** via
[GitHub Security Advisories](https://github.com/Kreuzbube88/helboot/security/advisories/new).
Do not open public issues for security problems.

Please include:

- A description of the issue and its impact
- Steps to reproduce
- Affected version/commit

You will receive an acknowledgement, and we aim to provide a fix or
mitigation as quickly as possible for the severity involved.

## Security design notes

HELBOOT is designed for **trusted homelab networks**, but still implements
defense in depth:

- Passwords are hashed with Argon2id; sessions are server-side with
  secure, HttpOnly cookies.
- CSRF protection on all state-changing API requests.
- Input validation on all API endpoints.
- Rate limiting on authentication endpoints.
- No secrets are ever committed to the repository; runtime secrets live in
  the data volume or environment variables.

## Permission matrix

Three strictly ordered roles: `viewer` < `operator` < `admin`. Every
API route declares its minimum role; the middleware enforces it
(`backend/internal/api/server.go` is the authoritative source — this
table documents the intended policy the routes implement).

| Resource | Action | Viewer | Operator | Admin |
| -------- | ------ | :----: | :------: | :---: |
| Profiles | view, list versions, preview answer file, export | ✔ | ✔ | ✔ |
| Profiles | create, edit, save new version, clone, import, delete, set answer-file override | – | ✔ | ✔ |
| Hosts | view | ✔ | ✔ | ✔ |
| Hosts | create, edit (incl. pinned profile version), delete | – | ✔ | ✔ |
| ISOs | view | ✔ | ✔ | ✔ |
| ISOs | upload, rescan, delete | – | ✔ | ✔ |
| Installations | view (incl. logs) | ✔ | ✔ | ✔ |
| Installations | queue, cancel | – | ✔ | ✔ |
| Users | manage accounts, roles, passwords | – | – | ✔ |
| Users | change **own** password, view own session | ✔ | ✔ | ✔ |
| Settings | view network config, system info, providers, app logs, download boot media | ✔ | ✔ | ✔ |
| Settings | change network config (mode, server IP, DHCP range) | – | – | ✔ |
| Backup | export, import | – | – | ✔ |

Rationale: viewers can observe everything except secrets (answer-file
*content* endpoints stay operator-write, viewer-read; backups contain
the whole database and are admin-only in both directions). Operators
run day-to-day provisioning. Only admins touch identities, network
behavior and whole-instance state.

Privileged actions and failed logins are recorded in the audit log.

**Important operational note:** PXE/TFTP/DHCP are inherently unauthenticated
protocols. Anyone on the boot network segment can boot the images HELBOOT
serves. Run HELBOOT on network segments you control and treat generated
answer files (which may contain credentials for provisioned systems) as
sensitive. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full
threat model.
