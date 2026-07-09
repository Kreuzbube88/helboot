# ADR-0016: Network-mode changes and rogue-DHCP detection

- **Status:** Accepted
- **Date:** 2026-07-09

## Context

ADR-0006 defines two operating modes: Mode A (ProxyDHCP next to an
existing DHCP server) and Mode B (HELBOOT is the DHCP server). Two
operational questions were unanswered:

1. What happens when *unexpected* DHCP servers appear on the segment —
   a rogue server in Mode A, or any foreign server in Mode B? PXE
   clients take the first usable offer; a rogue server silently breaks
   or hijacks netboot, which is very hard to diagnose from the outside.
2. Can the network mode be changed after the first-run wizard, and
   what does a change invalidate?

## Decision

### Rogue-DHCP detection (passive)

HELBOOT already receives every broadcast DHCP message on UDP 67 in both
modes. A shared **DHCP observer** extracts the *server identifier*
(option 54) from client `DHCPREQUEST` broadcasts — the client names the
server whose offer it accepted — and records each distinct foreign
server IP with a last-seen timestamp. No probing, no raw sockets, no
extra listeners.

Warnings derived from the observation window (last 30 minutes):

- **Mode A:** exactly one foreign DHCP server (the existing router) is
  the expected state. **Two or more** distinct foreign server IDs raise
  the `network.multiple_dhcp_servers` warning — a likely rogue server
  or misconfigured second router.
- **Mode B:** HELBOOT is authoritative, so **any** foreign server ID
  raises `network.rogue_dhcp_server`.

Sightings are exposed via `GET /api/v1/network/status` (server IPs,
last seen, active warnings) and logged as warnings; the UI shows a
banner on the dashboard. Detection is best-effort by nature: a passive
observer only sees broadcast traffic on its own segment and cannot see
which *boot options* another server hands out. This limitation is
documented rather than papered over with fragile sniffing.

### Changing the mode after the wizard

The mode remains changeable at any time under *Settings → Network*
(admin only, `PUT /api/v1/network/config`, takes effect after restart).
The change is **non-destructive by design**:

- Hosts are keyed by MAC address, profiles by provider — neither stores
  anything mode-specific. Installations reference profile versions.
  None of them are touched by a mode change.
- Mode B leases persist in the database but are simply unused while
  Mode A is active; switching back revives unexpired ones.
- Generated USB/CD boot media chain to HELBOOT's HTTP endpoint by
  server IP, which is independent of the DHCP mode. (Changing the
  *server IP* does invalidate previously downloaded boot media — the
  UI states this next to the field.)

The switch Mode A → Mode B additionally requires the DHCP range to be
configured; the API rejects the change otherwise, so a restart can
never come up in a half-configured authoritative mode.

## Consequences

- Rogue-DHCP problems become visible in the UI instead of manifesting
  as "PXE sometimes doesn't work"; the warning names the offending IPs.
- False positives are possible where two legitimate DHCP servers serve
  one segment (split scopes) — the warning is informational, never
  blocks operation, and clears itself once sightings age out.
- The mode is a runtime setting, not an identity: users can start
  cautious (Mode A) and later move to Mode B without re-registering
  hosts or rebuilding profiles.
