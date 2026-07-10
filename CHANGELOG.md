# Changelog

All notable changes to HELBOOT are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.1] - 2026-07-10

### Fixed

- Windows network installs failing at the WinPE stage with
  `.../boot/assets/wimboot ... Not found`: the `wimboot` binary was never
  fetched into the image, and even once fetched would have been seeded
  into the wrong directory (`assets/tftp/` instead of the assets root
  that `/boot/assets/` serves from).
- The Windows provider manifests (Windows 10, 11, Server 2022, Server
  2025) were missing the `initrd` entries wimboot needs (`BCD`,
  `boot.sdi`, `boot.wim`), so WinPE had nothing to boot even once
  wimboot itself was reachable.
- Windows 11 ISOs being misdetected as Windows 10: both share the
  `CCCOMA_X64FRE*` volume label on Microsoft's media, and ISO detection
  picked the first alphabetically registered provider on a tie. Detection
  now breaks such ties using provider-specific marker files.

## [1.0.0] - 2026-07-09

### Added

- Boot network services running supervised inside the single binary:
  TFTP, ProxyDHCP (Mode A, coexists with existing routers) and an
  authoritative DHCP server (Mode B) with persisted leases;
  architecture-aware iPXE bootstrapping and a network configuration API.
- Host discovery: unknown machines that PXE-boot appear automatically
  as "discovered" hosts.
- ISO management: streaming upload, SHA-256 hashing, automatic OS
  detection against provider rules, directory scan for existing shares,
  and HTTP serving of ISO contents (kernels/initrds) without modifying
  the original images.
- Installation queue: pinned profile versions, full iPXE boot chain,
  rendered answer files and cloud-init seeds, token-scoped installer
  status reporting, per-host history.
- Backup export/import (staged restore on restart) and an API log
  viewer backed by a structured log ring buffer.
- Frontend pages for ISOs (upload/scan), installations, logs and
  settings (network mode, DHCP range, backup) in English and German.
- User management: admin creates/edits/deletes accounts with the
  administrator/operator/viewer roles, admin password resets, and
  password self-service; the last administrator is protected and
  password changes revoke all sessions.
- Profile clone, export and import as portable JSON documents.
- Downloadable USB/CD boot media (iPXE images) for machines without
  PXE-capable firmware.
- Audit log entries for privileged actions and failed logins.
- Unraid template icon.
- Project architecture, component model and Architecture Decision Records.
- Go backend scaffolding: configuration, structured logging, SQLite with
  embedded migrations, session-based authentication with Argon2id password
  hashing, role model (admin / operator / viewer).
- REST API v1 skeleton: health, system info, setup wizard, authentication,
  providers, hosts and profiles endpoints; OpenAPI 3.1 specification.
- Provider/capability system with declarative YAML manifests for all v1
  target operating systems.
- React + TypeScript frontend scaffolding with i18n (English, German),
  login, first-run wizard and core page skeletons.
- Single Docker image (multi-stage build) and Unraid Community
  Applications template.
- CI pipeline: build, tests, lint, security scanning, dependency updates.
- Open source project files: contribution guide, code of conduct,
  security policy, support guide.

[Unreleased]: https://github.com/Kreuzbube88/helboot/compare/v1.0.1...HEAD
[1.0.1]: https://github.com/Kreuzbube88/helboot/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/Kreuzbube88/helboot/releases/tag/v1.0.0
