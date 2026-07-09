-- Hosts pin a specific profile version instead of implicitly following
-- "latest" (ADR-0013). 0 means "no version pinned" (host has no
-- profile). Existing assignments are backfilled with the profile's
-- current version so behavior stays what the user saw.

ALTER TABLE hosts ADD COLUMN profile_version INTEGER NOT NULL DEFAULT 0;

UPDATE hosts
SET profile_version = (SELECT current_version FROM profiles WHERE profiles.id = hosts.profile_id)
WHERE profile_id IS NOT NULL;
