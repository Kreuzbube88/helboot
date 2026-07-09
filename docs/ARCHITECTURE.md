# HELBOOT Architecture

HELBOOT is a **provisioning engine** for homelabs. PXE is only one boot
method among several; the architecture is designed so new boot methods,
operating systems and integrations can be added without touching the core.

This document describes the system architecture. Individual decisions and
their trade-offs are recorded as [Architecture Decision Records](adr/).

## 1. High-level view

```
                        ┌──────────────────────────────────────────────┐
                        │            HELBOOT (one container)           │
                        │                                              │
 ┌──────────┐  HTTPS    │  ┌─────────────┐      ┌────────────────────┐ │
 │ Browser  ├───────────┼─▶│  Web UI     │      │   REST API (v1)    │ │
 └──────────┘           │  │ React + TS  ├─────▶│   Go, OpenAPI      │ │
                        │  └─────────────┘      └─────────┬──────────┘ │
                        │                                 │            │
                        │                        ┌────────▼──────────┐ │
                        │                        │       Core        │ │
                        │                        │  profiles, hosts, │ │
                        │                        │  queue, ISO scan, │ │
                        │                        │  answer files     │ │
                        │                        └──┬───────┬──────┬─┘ │
                        │                           │       │      │   │
                        │  ┌────────────────┐  ┌────▼───┐ ┌─▼────┐ │   │
                        │  │ Provider       │  │Storage │ │SQLite│ │   │
                        │  │ Registry       │◀─┤ (ISOs, │ │ + mi-│ │   │
                        │  │ (YAML          │  │ assets)│ │ gra- │ │   │
                        │  │  manifests)    │  └────────┘ │ tions│ │   │
                        │  └────────────────┘             └──────┘ │   │
                        │                                          │   │
                        │  ┌───────────────────────────────────────▼─┐ │
 ┌──────────┐ DHCP/PXE  │  │            Network Services              │ │
 │ Target   │◀──────────┼──┤  DHCP │ ProxyDHCP │ TFTP │ HTTP Boot     │ │
 │ Machines │  TFTP/HTTP│  └──────────────────────────────────────────┘ │
 └──────────┘           └──────────────────────────────────────────────┘
```

Everything ships as **one Docker image / one container** (ADR-0009).
Internally, network services run as goroutines/subprocesses supervised by
the main Go binary — the user never manages more than one container.

## 2. Component model

| Component | Directory | Responsibility |
| --------- | --------- | -------------- |
| **core** | `backend/internal/core` | Domain logic: profiles, hosts, installation queue, ISO analysis, answer-file generation. Independent of HTTP and network protocols. |
| **api** | `backend/internal/api` | REST API v1, session handling, middleware (auth, CSRF, rate limiting, logging). The frontend talks *only* to this API (ADR-0010). |
| **provider** | `backend/internal/provider` + `providers/` | Provider registry; loads declarative YAML manifests describing each OS's capabilities, boot methods and answer-file templates (ADR-0005). |
| **network** | `backend/internal/network` | DHCP, ProxyDHCP, TFTP and HTTP-boot services; boot session orchestration (ADR-0006). |
| **storage** | `backend/internal/storage` | ISO library, extracted boot assets, generated per-host files; path layout of the `/data` volume. |
| **db** | `backend/internal/db` | SQLite access (WAL, single writer — ADR-0015), embedded migrations, backup/restore (ADR-0004). |
| **auth** | `backend/internal/auth` | Local users (Argon2id), server-side sessions, roles; OIDC-ready abstraction (ADR-0007). |
| **frontend** | `frontend/` | React + TypeScript SPA, fully internationalized (ADR-0003). |
| **plugins** | `backend/internal/plugin` | Extension-point registry for future plugin support (ADR-0008). |

### Dependency rule

`api → core → (provider, storage, db)`. Nothing imports `api`. `core`
never imports network protocol code; the network layer consumes core
services through interfaces. This keeps domain logic testable without
sockets or root privileges.

## 3. Provider & capability system

Operating systems are **never hardcoded**. Each OS is a *provider*
declared by a YAML manifest under `providers/<name>/provider.yaml`:

```yaml
name: windows11
display_name: "Windows 11"
family: windows
capabilities:
  iso: true
  unattended_install: true
  pxe: true
  http_boot: true
  usb_boot: true
  secure_boot: true
answer_file:
  format: autounattend.xml
  template: templates/autounattend.xml.tmpl
settings_schema:            # generates the profile form (ADR-0012)
  - key: Language
    type: string
    label: "Language"
    group: localization
    default: "en-US"
    required: true
  - key: ProductKey
    type: string
    label: "Product key"
    group: install
detection:
  volume_id_patterns: ["CCCOMA_X64FRE*", "CPBA_X64FRE*"]
  files: ["sources/install.wim", "sources/install.esd"]
boot:
  pxe:
    kernel: wimboot
    requires: [winpe]
```

The registry validates manifests at startup and exposes them through the
API. **The UI renders options purely from capabilities** — a provider
without `usb_boot` simply never shows a USB option. Adding an OS means
adding a manifest (plus templates), not changing core code.

The **`settings_schema`** (ADR-0012) declares each configurable field
(language, keyboard, users, network, partitioning, packages, scripts)
with type, default, required flag and field dependencies. The frontend
generates the profile form from this schema — there are no hardcoded
per-OS forms — and the API validates profile config documents against
it. Field keys double as the template variables of the provider's
answer-file template, so schema, stored config and generated file stay
in lockstep.

Providers whose full automation is not yet technically possible (e.g.
ESXi Secure Boot scenarios) declare reduced capabilities and document the
limitation in their manifest `notes` field; the UI surfaces this.

## 4. Data model

```
User          (id, username, password_hash, role, locale, created_at, …)
Session       (id, user_id, expires_at, csrf_token, …)
Setting       (key, value)                    – app config incl. wizard state
ISOImage      (id, filename, os_name, version, arch, bootloader,
               install_method, size, sha256, status, provider, …)
Profile       (id, name, provider, iso_id, current_version, created_at, …)
ProfileVersion(id, profile_id, version, config_json, answer_override,
               created_at)
Host          (id, mac, hostname, vendor, model, serial, asset_id,
               firmware [bios|uefi], arch, tags, profile_id,
               profile_version, status, …)
Installation  (id, host_id, profile_version_id, status
               [discovered|waiting|installing|success|error],
               started_at, finished_at, log, …)
AuditLog      (id, user_id, action, entity, entity_id, at)
```

Key relationships:

- A **Profile** belongs to a provider and (optionally) an ISO; its
  settings are stored per **ProfileVersion**. New versions are created
  only when the user explicitly saves one — ordinary edits change the
  head version in place, unless an installation references it
  (ADR-0013). A version may carry a manual answer-file override
  (ADR-0014).
- A **Host** is identified by MAC address and points at the profile to
  install, **pinned to a specific version** — never an implicit
  "latest". The pin is visible and changeable in the UI.
- An **Installation** links a host to a *specific profile version*, so
  history stays accurate even if the profile changes later. Versions
  referenced by installations are immutable, enforced in the store.

Profile configuration is stored as a JSON document (validated against a
schema) rather than dozens of columns, because its shape differs per
provider (Windows vs. kickstart vs. cloud-init).

## 5. API design

- REST, JSON, versioned base path `/api/v1` (ADR-0010).
- The OpenAPI 3.1 specification lives at `backend/api/openapi.yaml`, is
  served at `/api/v1/openapi.yaml`, and is the contract the frontend is
  generated/typed against.
- Auth: session cookie (HttpOnly, SameSite=Lax, Secure when TLS);
  mutating requests require the `X-CSRF-Token` header.
- Errors use a single envelope: `{"error": {"code": "…", "message": "…"}}`
  where `code` is a stable, i18n-translatable identifier — the UI maps
  codes to localized messages, the message is an English fallback.

Endpoint groups (v1): `health`, `system`, `setup` (first-run wizard),
`auth`, `users`, `providers`, `isos`, `profiles`, `hosts`,
`installations`, `network`, `backup`, `logs`. See [API.md](API.md).

## 6. Network architecture

Two operating modes, selected in the first-run wizard and changeable
later in the settings (ADR-0006, ADR-0016):

- **Mode A — ProxyDHCP** (default): an existing DHCP server (FRITZ!Box,
  router) keeps handing out addresses. HELBOOT answers only the PXE part
  of the DHCP conversation (ProxyDHCP, port 4011 + broadcast), plus TFTP
  and HTTP boot. Zero changes to the existing network.
- **Mode B — full DHCP**: HELBOOT runs its own DHCP server with a
  configurable pool, plus PXE/TFTP/HTTP.

Boot flow (both modes):

1. NIC broadcasts DHCP/PXE discover → HELBOOT offers boot server info.
2. Firmware fetches `ipxe.efi` / `undionly.kpxe` via TFTP (architecture
   detected from DHCP option 93).
3. iPXE chains to HELBOOT's HTTP boot endpoint, sending its MAC.
4. HELBOOT looks up the host: unknown MACs are recorded as *discovered*
   (device discovery); known hosts with a queued installation receive a
   per-host boot script pointing at kernel/initrd/WinPE and the generated
   answer file.
5. Installers stream packages/ISO contents over HTTP from HELBOOT.

**Mode changes and rogue DHCP** (ADR-0016): the mode can be switched at
any time (admin, restart required) without invalidating hosts, profiles
or installations — none of them store mode-specific data. In both modes
a passive observer extracts the server identifier from client
`DHCPREQUEST` broadcasts; unexpected DHCP servers (a second one in
Mode A, any foreign one in Mode B) surface as warnings via
`GET /api/v1/network/status` and a dashboard banner.

**USB boot** does not write USB sticks directly: HELBOOT generates a
small ISO/IMG containing iPXE + bootloader configured to contact the
HELBOOT server; the user writes it with their favorite imaging tool.
After boot, the flow continues at step 3 above.

Docker: **host networking is required for Mode B and recommended for
Mode A** (broadcast traffic). macvlan/Unraid `br0` are supported where
the platform delivers broadcasts reliably; limitations are documented in
[DOCKER.md](DOCKER.md) and [UNRAID.md](UNRAID.md).

## 7. ISO handling

Original ISOs are **never modified**:

1. User uploads (or drops into the ISO directory) an original ISO.
2. The analyzer inspects it read-only: volume ID, well-known files,
   bootloader layout → matches provider `detection` rules → OS, version,
   architecture, boot method.
3. Boot-relevant files (kernel, initrd, boot.wim, …) are extracted or
   served directly from the ISO via HTTP range requests.
4. All customization happens through *external* configuration: answer
   files, kernel command lines, iPXE scripts — generated per
   host/profile at boot time.

## 8. Answer files

Generated from provider templates + profile data:

- Windows → `autounattend.xml`
- Ubuntu → `autoinstall.yaml` (subiquity) / cloud-init
- Debian → preseed
- Fedora → kickstart
- openSUSE → AutoYaST
- Proxmox VE → answer TOML (auto-install)
- Others → per provider

Templates are Go `text/template` files living **inside each provider's
directory**, never in core; values come from the profile config (shaped
by the provider's `settings_schema`, ADR-0012) plus boot-time
parameters (URLs, MAC, hostname) that always win on conflict.

The generated file is **inspectable before use** via a per-version
preview endpoint, and **manually overridable**: an override stored on
the profile version replaces the provider template until it is cleared
(ADR-0014). Overrides run through the same template engine, so boot
parameters keep working inside them.

## 9. Security concept

Threat model: HELBOOT runs on a trusted LAN, but defense in depth applies.

- **Passwords:** Argon2id with per-user salt; parameters follow current
  OWASP guidance.
- **Sessions:** server-side (SQLite), random 256-bit IDs, HttpOnly +
  SameSite cookies, idle and absolute expiry.
- **CSRF:** per-session token required on all mutating requests.
- **Input validation:** every API payload validated (types, ranges, MAC
  formats, path traversal guards on all file access).
- **Rate limiting:** token bucket on `/auth/*`; failed logins are audited.
- **Roles:** `admin` (everything), `operator` (manage hosts, profiles,
  installs), `viewer` (read-only). Enforced in API middleware; the full
  resource × role × action matrix lives in
  [SECURITY.md](../SECURITY.md#permission-matrix).
- **Secrets:** never in the repository; generated at first run and stored
  in the data volume. Answer files can embed credentials for target
  systems → they are only served to the matching MAC during an active
  installation window and are excluded from unauthenticated listing.
- **Unauthenticated by design:** DHCP/TFTP/PXE cannot be authenticated;
  this residual risk is documented in [SECURITY.md](../SECURITY.md).

## 10. Plugin concept

Version 1 ships the *extension points*, not a full plugin runtime
(ADR-0008, including its amendment on contract and load locations):

- **Providers** are already data-driven plugins (YAML manifest +
  settings schema + templates). They load from the shipped
  `providers/` directory and additionally from `/data/providers/` on
  the data volume (later wins on name collision), so users can add or
  patch providers without rebuilding the image.
- Core defines registry interfaces for the code-level plugin kinds in
  `backend/internal/plugin`: `BootMethod`, `IdentityProvider` (OIDC is
  the first planned addition), `AnswerFileRenderer`; post-v1:
  notification sinks and software/driver repositories.
- Plugins do **not** ship UI code: capabilities and the settings schema
  are the declarative UI extension surface.
- Code plugins are Go packages registered at compile time in
  `cmd/helboot`; out-of-process plugins are a later ADR once the
  interfaces have stabilized.

## 11. Internationalization

- No visible string is hardcoded — UI text lives in
  `frontend/src/i18n/locales/{en,de}/*.json` (react-i18next).
- API error **codes** are the translation keys for server-side failures.
- Locale is a user preference; the wizard's first step selects it.

## 12. Observability

- Structured logging (Go `slog`, JSON in production): levels debug, info,
  warning, error.
- Logs are persisted (ring buffer + files in `/data/logs`) and exposed
  through the API for the UI log viewer.
- Installation lifecycles are additionally recorded per host
  (installation history).

## 13. Backup & restore

`/api/v1/backup` exports a single archive: SQLite database (consistent
snapshot), profiles, settings and configuration. Import restores it
completely. ISOs are *not* included (size); the archive records their
checksums so missing ISOs are reported after restore.

## 14. Repository layout

```
backend/          Go module (core, api, provider, network, storage, db, auth)
frontend/         React + TypeScript SPA
providers/        Provider manifests + answer-file templates (data, not code)
deploy/docker/    Dockerfile, entrypoint
deploy/unraid/    Unraid Community Applications template
docs/             Architecture, guides, ADRs
docs/adr/         Architecture Decision Records
```
