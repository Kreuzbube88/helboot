# HELBOOT on Unraid

HELBOOT provides a Docker template compatible with Unraid Community
Applications: [`deploy/unraid/helboot.xml`](../deploy/unraid/helboot.xml).

## Recommended setup

1. Install via Community Applications (or add the template XML manually
   under *Docker → Add Container → Template repositories*).
2. **Network type: Host** (recommended). PXE needs DHCP broadcasts; host
   networking is the most reliable option and required if HELBOOT should
   act as the DHCP server (Mode B).
3. Map the data share, e.g. `/mnt/user/appdata/helboot` → `/data`.
4. Optionally map an existing ISO share, e.g. `/mnt/user/isos` →
   `/data/isos`.
5. Start the container and open `http://<unraid-ip>:8080` for the
   first-run wizard.

## br0 / macvlan notes

Unraid's `br0` custom network gives the container its own IP on your LAN
and generally delivers broadcasts, so ProxyDHCP (Mode A) usually works.
Caveats:

- With macvlan-based `br0`, the **Unraid host itself cannot reach the
  container** — open the UI from another device, or enable Unraid's
  "Host access to custom networks" setting.
- On some setups macvlan has caused kernel instability on Unraid; ipvlan
  (Unraid ≥ 6.10 option) avoids that but has its own broadcast quirks.
- If PXE clients don't see boot options on `br0`, switch the container to
  **Host** networking — that is the supported baseline.

## Port conflicts

With host networking, ensure UDP 67/69/4011 and TCP 8080 are free on the
Unraid host. Common conflicts: a Pi-hole or dnsmasq container also using
host networking, or Unraid's own services if you changed defaults. The
HTTP port is configurable via `HELBOOT_HTTP_ADDR`.

## Template variables

The template exposes:

- **Data** (`/data`) — appdata path (required)
- **ISOs** (`/data/isos`) — optional dedicated ISO share
- **WebUI port** — informational; with host networking the port is set
  via `HELBOOT_HTTP_ADDR`
- **Log level** — `HELBOOT_LOG_LEVEL`
