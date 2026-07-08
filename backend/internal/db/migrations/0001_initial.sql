-- HELBOOT initial schema. See docs/ARCHITECTURE.md §4 for the data model.

CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL CHECK (role IN ('admin', 'operator', 'viewer')),
    locale        TEXT    NOT NULL DEFAULT 'en',
    created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    csrf_token TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (datetime('now')),
    expires_at TEXT    NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions (expires_at);

-- Application settings, including first-run wizard state and network mode.
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE iso_images (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    filename       TEXT    NOT NULL UNIQUE,
    provider       TEXT    NOT NULL DEFAULT '',
    os_name        TEXT    NOT NULL DEFAULT '',
    version        TEXT    NOT NULL DEFAULT '',
    arch           TEXT    NOT NULL DEFAULT '',
    bootloader     TEXT    NOT NULL DEFAULT '',
    install_method TEXT    NOT NULL DEFAULT '',
    size_bytes     INTEGER NOT NULL DEFAULT 0,
    sha256         TEXT    NOT NULL DEFAULT '',
    status         TEXT    NOT NULL DEFAULT 'uploaded'
        CHECK (status IN ('uploaded', 'analyzing', 'ready', 'unsupported')),
    created_at     TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE profiles (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL UNIQUE,
    provider        TEXT    NOT NULL,
    iso_id          INTEGER REFERENCES iso_images (id) ON DELETE SET NULL,
    current_version INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- Immutable configuration snapshots: versioning, cloning and export are
-- built on these rows (docs/ARCHITECTURE.md §4).
CREATE TABLE profile_versions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id INTEGER NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    version    INTEGER NOT NULL,
    config     TEXT    NOT NULL DEFAULT '{}',
    created_at TEXT    NOT NULL DEFAULT (datetime('now')),
    UNIQUE (profile_id, version)
);

CREATE TABLE hosts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    mac        TEXT    NOT NULL UNIQUE,
    hostname   TEXT    NOT NULL DEFAULT '',
    vendor     TEXT    NOT NULL DEFAULT '',
    model      TEXT    NOT NULL DEFAULT '',
    serial     TEXT    NOT NULL DEFAULT '',
    asset_id   TEXT    NOT NULL DEFAULT '',
    tags       TEXT    NOT NULL DEFAULT '[]',
    firmware   TEXT    NOT NULL DEFAULT '' CHECK (firmware IN ('', 'bios', 'uefi')),
    arch       TEXT    NOT NULL DEFAULT '',
    profile_id INTEGER REFERENCES profiles (id) ON DELETE SET NULL,
    status     TEXT    NOT NULL DEFAULT 'discovered'
        CHECK (status IN ('discovered', 'ready', 'installing', 'error')),
    created_at TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- Installations reference a specific profile version so history stays
-- accurate when profiles evolve.
CREATE TABLE installations (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    host_id            INTEGER NOT NULL REFERENCES hosts (id) ON DELETE CASCADE,
    profile_version_id INTEGER NOT NULL REFERENCES profile_versions (id),
    status             TEXT    NOT NULL DEFAULT 'waiting'
        CHECK (status IN ('discovered', 'waiting', 'installing', 'success', 'error')),
    started_at         TEXT,
    finished_at        TEXT,
    log                TEXT    NOT NULL DEFAULT '',
    created_at         TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_installations_host ON installations (host_id);

CREATE TABLE audit_log (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id   INTEGER REFERENCES users (id) ON DELETE SET NULL,
    action    TEXT    NOT NULL,
    entity    TEXT    NOT NULL DEFAULT '',
    entity_id TEXT    NOT NULL DEFAULT '',
    at        TEXT    NOT NULL DEFAULT (datetime('now'))
);
