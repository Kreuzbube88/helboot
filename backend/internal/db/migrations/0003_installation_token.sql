-- Per-installation secret token. Boot-time endpoints (answer files,
-- status reports) are unauthenticated by protocol necessity; the token
-- scopes them to one specific installation.

ALTER TABLE installations ADD COLUMN token TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_installations_token ON installations (token);
