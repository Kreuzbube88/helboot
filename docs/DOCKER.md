# Docker Deployment

HELBOOT ships as **one image** and runs as **one container**. All state
lives in a single volume mounted at `/data`.

## Quick start

```bash
docker run -d \
  --name helboot \
  --network host \
  -v /srv/helboot:/data \
  ghcr.io/kreuzbube88/helboot:latest
```

Open `http://<host-ip>:8080` — the first-run wizard guides you through
language, admin account, network mode and storage locations.

## Networking

PXE relies on DHCP broadcast traffic, which does **not** traverse
Docker's default bridge network.

| Mode | Works | Notes |
| ---- | ----- | ----- |
| `--network host` | ✅ recommended | Required for HELBOOT-DHCP (Mode B), recommended for ProxyDHCP (Mode A). |
| `macvlan` | ⚠️ usually | The container gets its own MAC/IP on the LAN and receives broadcasts. Caveat: the Docker *host* cannot reach the container directly (macvlan isolation) — manage the UI from another machine or add a macvlan shim interface. |
| Unraid `br0` | ⚠️ usually | Unraid's br0 custom network is macvlan/ipvlan-based; same caveats. See [UNRAID.md](UNRAID.md). |
| default bridge | ❌ | Broadcasts don't arrive; PXE will not work. UI-only evaluation is possible with `-p 8080:8080`. |

## Ports

| Port | Protocol | Service | Needed |
| ---- | -------- | ------- | ------ |
| 8080 | TCP | Web UI + API + HTTP boot | always (configurable) |
| 69 | UDP | TFTP | PXE boot |
| 67 | UDP | DHCP / ProxyDHCP | Mode B / Mode A |
| 4011 | UDP | ProxyDHCP (PXE service) | Mode A |

With host networking make sure no other service occupies these ports
(e.g. a dnsmasq or pihole instance on the same host).

## Volume layout (`/data`)

```
/data/
  helboot.db        SQLite database
  isos/             uploaded original ISOs (never modified)
  assets/           extracted/generated boot assets
  logs/             structured logs
  secrets/          generated at first run (session keys)
```

## Environment variables

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `HELBOOT_DATA_DIR` | `/data` | State directory |
| `HELBOOT_HTTP_ADDR` | `:8080` | HTTP listen address |
| `HELBOOT_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `HELBOOT_LOG_FORMAT` | `json` | `json` / `text` |

## Building locally

```bash
docker build -f deploy/docker/Dockerfile -t helboot:dev .
```

The multi-stage build compiles the frontend, embeds it into the Go
binary, and produces a minimal final image (see
[ADR-0009](adr/0009-single-docker-image.md)).
