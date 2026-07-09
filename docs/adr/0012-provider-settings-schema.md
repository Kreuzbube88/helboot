# ADR-0012: Declarative settings schema per provider

- **Status:** Accepted
- **Date:** 2026-07-09

## Context

Profiles store their configuration as a provider-shaped JSON document
(ADR-0005, ARCHITECTURE §4), but nothing declares *which* fields a
provider understands. The frontend therefore had no configuration form
at all, and the API accepted any JSON without validation. The
specification requires that each provider describes its settings
(language, keyboard, users, network, partitioning, packages, scripts)
with types, defaults, required flags and field dependencies, and that
the UI generates the profile form from that description — no hardcoded
per-OS forms.

Full JSON Schema was considered: it is expressive, but rendering an
arbitrary JSON Schema as a usable form is a large project, most of its
power (recursion, `oneOf`, `$ref`) is unnecessary here, and validating
it in Go would add a heavyweight dependency.

## Decision

Each provider manifest gains a **`settings_schema`**: a flat, ordered
list of field descriptors — a deliberate, form-oriented subset of what
JSON Schema can express:

```yaml
settings_schema:
  - key: Locale            # template variable name ({{ .Locale }})
    type: string           # string|text|password|bool|int|select|list
    label: "Locale"        # English fallback label
    group: localization    # localization|users|network|storage|packages|scripts|install
    default: "en_US"
    required: true
    help: "…"              # optional hint, shown under the field
    options: [...]         # select only
    min: 1                 # int only
    max: 99                # int only
    depends_on:            # field is shown/required only when another
      field: OtherKey      #   field has the given value
      value: true
```

- **Types:** `string`, `text` (multiline), `password` (masked input),
  `bool`, `int`, `select` (one of `options`), `list` (list of strings —
  packages, late commands).
- **Field keys are the template variables.** The schema is the single
  source of truth connecting the form, the stored config document and
  the answer-file template (ADR-0014).
- The registry **validates the schema itself** at manifest load time
  (unknown types, `select` without options, dangling `depends_on`
  disable the provider, nothing else).
- The API **validates profile config documents against the schema** on
  create/update: type checks, required fields (respecting
  `depends_on`), `select` membership, `int` ranges. Keys *not* declared
  in the schema are preserved untouched — older backups and hand-written
  configs stay importable; strictness applies only to declared fields.
- The frontend renders the profile form **generically** from
  `settings_schema`, grouped by `group`, with no per-provider code.
  Labels resolve through i18n first (`profileFields.<key>`) and fall
  back to the manifest `label`, so well-known fields are localized while
  new providers can introduce fields without touching the core.
- `password` fields are masked in the form but stored as given; whether
  a provider expects a plain password or a crypt hash is stated in the
  field's `help` (e.g. Linux providers want `mkpasswd -m sha-512`
  output). Server-side hashing helpers are future work.

## Consequences

- Adding an OS version or a new setting stays a data change in
  `providers/` — the UI picks it up automatically.
- Existing profiles become editable in the UI: the form is rendered
  from the schema and pre-filled from the stored config document.
- The subset is less expressive than JSON Schema (no nesting, no
  cross-field validation beyond `depends_on`); if a provider ever needs
  more, the schema format gets extended by a new ADR rather than
  worked around in core code.
- Config documents remain JSON, so export/import, cloning and
  versioning (ADR-0013) are unaffected.
