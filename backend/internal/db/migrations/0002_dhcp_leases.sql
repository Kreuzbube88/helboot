-- DHCP leases for Mode B (HELBOOT is the DHCP server, ADR-0006).
-- Persisted so leases survive restarts.

CREATE TABLE dhcp_leases (
    mac        TEXT PRIMARY KEY,
    ip         TEXT NOT NULL UNIQUE,
    hostname   TEXT NOT NULL DEFAULT '',
    expires_at TEXT NOT NULL
);
CREATE INDEX idx_dhcp_leases_expires ON dhcp_leases (expires_at);
