# ADR-0005: Data-driven provider & capability architecture

- **Status:** Accepted
- **Date:** 2026-07-08

## Context

HELBOOT must support many operating systems (Windows, Debian, Ubuntu,
Fedora, openSUSE, DietPi, Proxmox VE, TrueNAS SCALE, Home Assistant OS,
ESXi) with very different boot methods, installers and answer-file
formats. Hardcoding OS logic in core would make every addition a core
change and violate the specification's "no hardcoding" rule.

## Decision

Every operating system is a **provider**: a directory under `providers/`
containing a declarative **YAML manifest** (`provider.yaml`) plus
answer-file templates. The manifest declares:

- identity (`name`, `display_name`, `family`)
- **capabilities** (`iso`, `unattended_install`, `pxe`, `http_boot`,
  `usb_boot`, `secure_boot`, …) — booleans the UI uses to decide which
  options to render
- ISO **detection** rules (volume-ID patterns, marker files)
- **boot** configuration per method (kernel/initrd paths, iPXE fragments)
- **answer file** format and template reference
- free-text `notes` documenting limitations

The Go core contains a single generic registry that loads, validates and
serves manifests. Behavior that genuinely cannot be expressed as data
(e.g. WIM parsing for Windows) lives behind a small Go interface keyed by
`family`, not by individual OS.

## Consequences

- Adding an OS is (mostly) adding files, not code — reviewable by
  non-Go-developers.
- The UI never contains per-OS conditionals; it renders from capabilities.
- Manifests are validated at startup; a broken manifest disables only that
  provider, never the application.
- Family-level Go hooks are the escape hatch, and each new hook needs
  justification in review.

## Amendment (2026-07-09): DietPi and Home Assistant OS removed

DietPi and Home Assistant OS were shipped as providers with
`unattended_install`/`image` capabilities but no boot method at all
(`pxe`/`http_boot`/`usb_boot` all `false`): both ship as pre-built disk
images with no network-install path, so HELBOOT had nothing to actually
deliver over the network — queuing an installation left the host stuck
in `waiting` indefinitely. This is a different situation from TrueNAS
SCALE (network-boots a real, if interactive, installer) or ESXi Secure
Boot (a declared, narrower capability gap): DietPi/HAOS had no working
path at all, not a reduced one.

Both providers are removed until HELBOOT gains a genuine network
image-write capability (netboot a minimal environment that writes a raw
image to the target disk) — a substantial addition (destructive
disk-write safety, a new boot-method type) that needs its own ADR and
explicit product decision, not a passive "documented limitation" on an
otherwise inert provider.
