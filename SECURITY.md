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

**Important operational note:** PXE/TFTP/DHCP are inherently unauthenticated
protocols. Anyone on the boot network segment can boot the images HELBOOT
serves. Run HELBOOT on network segments you control and treat generated
answer files (which may contain credentials for provisioned systems) as
sensitive. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full
threat model.
