# Changelog

All notable changes to HELBOOT are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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
