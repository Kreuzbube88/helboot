# ADR-0013: Explicit profile versioning and host version pinning

- **Status:** Accepted
- **Date:** 2026-07-09

## Context

Profiles already store their configuration in immutable
`profile_versions` snapshots, and installations pin the version they
were queued with. Two gaps remained against the specification:

1. *Every* config edit silently created a new version — versions
   accumulated noise and the user had no control over what constitutes
   a "version".
2. Hosts referenced only the profile, implicitly meaning "current
   version at queue time". The specification requires that a host
   references a *fixed* profile version, visible and changeable by the
   user — never an automatic "latest".

## Decision

1. **New versions are created only on explicit request.** Updating a
   profile's config edits the current head version *in place* by
   default. The API's `PUT /profiles/{id}` accepts
   `saveAsNewVersion: true` to snapshot the config as version N+1
   instead; the UI offers this as a checkbox when saving.
2. **History stays immutable.** In-place editing is refused (HTTP 409,
   `profile.version_in_use`) when the head version is referenced by any
   installation — at that point the user must save as a new version.
   Finished installations therefore always point at exactly the config
   they installed, even if the profile evolves later.
3. **Hosts pin a version number.** `hosts.profile_version` stores the
   pinned version of the assigned profile. Assigning a profile pins its
   then-current version by default; the user can re-pin any existing
   version in the host form. Queueing an installation uses the host's
   pinned version (an explicit profile override on the queue call uses
   that profile's current version).
4. Cloning and import create a fresh profile at version 1 from the
   source's current config — unchanged from before.

## Consequences

- Users decide when a change is worth a version; the version list stays
  meaningful ("v3 = added docker package") instead of one entry per
  keystroke.
- A version referenced by an installation can never change afterwards —
  traceability holds by construction, enforced in the store layer, not
  by convention.
- Hosts surviving profile edits keep installing exactly what the user
  pinned; upgrading a host to a newer profile version is a visible,
  deliberate act.
- Slight API asymmetry: version numbers (1, 2, …) are the user-facing
  handle, version row IDs stay internal.
