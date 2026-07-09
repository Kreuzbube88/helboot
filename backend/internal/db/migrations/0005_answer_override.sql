-- Manual answer-file override per profile version (ADR-0014). While
-- non-empty it replaces the provider template for this version; clearing
-- it means "regenerate from the template". The immutable config column
-- is untouched — the override is a mutable side channel.

ALTER TABLE profile_versions ADD COLUMN answer_override TEXT NOT NULL DEFAULT '';
