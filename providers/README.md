# HELBOOT Providers

Each directory describes one operating system as a declarative
**provider manifest** (`provider.yaml`). No OS logic is hardcoded in
core; the UI renders its options purely from the `capabilities` declared
here. See [ADR-0005](../docs/adr/0005-provider-capability-architecture.md).

## Manifest schema

```yaml
name: <id>                 # must equal the directory name; [a-z0-9-]
display_name: "<label>"    # shown in the UI (product names, not localized)
family: <family>           # windows | debian | rhel | suse | appliance | ...
capabilities:              # booleans; the UI adapts to these
  iso: true                # installed from an original ISO
  unattended_install: true # fully automated installation supported
  pxe: true                # legacy/UEFI PXE network boot
  http_boot: true          # UEFI HTTP boot / iPXE http chaining
  usb_boot: true           # bootable iPXE USB image can target it
  secure_boot: false       # works with Secure Boot enabled
answer_file:
  format: <format>         # autounattend.xml | preseed | autoinstall.yaml |
                           # kickstart | autoyast | cloud-init | answer.toml | none
  template: templates/<f>  # relative to the provider directory
detection:                 # ISO analyzer matching rules
  volume_id_patterns: []   # globs against the ISO 9660 volume ID
  files: []                # paths that must exist inside the ISO
boot:                      # per boot method: pxe | http_boot | usb_boot
  pxe:
    kernel: <path>         # kernel/boot program (paths inside the ISO)
    initrd: [<path>, ...]
    cmdline: "<template>"
    requires: [<asset>]    # extra assets, e.g. winpe
notes: >                   # documented limitations (shown in the UI)
  ...
```

Adding an operating system = adding a directory with a valid manifest
(plus answer-file templates). The backend validates manifests at startup;
an invalid manifest disables only that provider.
