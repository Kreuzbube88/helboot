# Development Guide

## Prerequisites

- Go ≥ 1.24
- Node.js ≥ 22 (with npm)
- Docker (only needed for image builds)

## Repository layout

```
backend/          Go module — API, core, providers, network, storage, db
frontend/         React + TypeScript SPA (Vite)
providers/        Provider manifests and answer-file templates
deploy/           Dockerfile and Unraid template
docs/             Documentation and ADRs
```

## Backend

```bash
cd backend
go build ./...            # build everything
go test ./...             # run tests
go test -race -cover ./...# what CI runs
go run ./cmd/helboot      # start the server (data dir: ./data by default)
```

Configuration is environment-based, prefix `HELBOOT_`:

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `HELBOOT_DATA_DIR` | `./data` | State directory (database, ISOs, logs, secrets) |
| `HELBOOT_HTTP_ADDR` | `:8080` | HTTP listen address for UI + API |
| `HELBOOT_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `HELBOOT_LOG_FORMAT` | `text` | `text` or `json` |

The database schema is created automatically at startup via embedded
migrations (`backend/internal/db/migrations/`). To add a migration, add
the next numbered `NNNN_name.sql` file — never edit an existing one.

## Frontend

```bash
cd frontend
npm install
npm run dev        # Vite dev server, proxies /api to localhost:8080
npm run build      # typecheck + production build
npm run lint
npm test
```

**i18n rule:** never hardcode visible text. Add keys to
`src/i18n/locales/en/translation.json` **and**
`src/i18n/locales/de/translation.json`, then use `t('your.key')`.

## Providers

Provider manifests live in `providers/<name>/provider.yaml`. The backend
loads and validates them at startup; an invalid manifest disables that
provider and logs an error. See
[ADR-0005](adr/0005-provider-capability-architecture.md) and existing
manifests for the schema.

## Docker

```bash
docker build -f deploy/docker/Dockerfile -t helboot:dev .
docker run --rm --network host -v $(pwd)/data:/data helboot:dev
```

## Quality gates (CI mirrors these)

- `gofmt -l .` must be empty; `go vet ./...` clean
- `go test -race ./...` green
- `npm run lint`, `npm run build`, `npm test` green
- Docker image builds

## Architecture rules

- Frontend talks to the backend **only** via `/api/v1` (ADR-0010).
- No OS-specific logic in core — extend providers (ADR-0005).
- Significant decisions need an ADR (ADR-0001).
