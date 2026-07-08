// Package provider implements HELBOOT's data-driven operating system
// provider system (ADR-0005). Each OS is described by a declarative YAML
// manifest under providers/<name>/provider.yaml; no OS logic is
// hardcoded in core.
package provider

import (
	"fmt"
	"regexp"
)

// Manifest is the parsed provider.yaml describing one operating system.
type Manifest struct {
	// Name is the unique provider identifier (lowercase, digits, dashes).
	Name string `yaml:"name" json:"name"`
	// DisplayName is what the UI shows; localization of OS product names
	// is not needed, they are proper nouns.
	DisplayName string `yaml:"display_name" json:"displayName"`
	// Family groups providers that share installer mechanics
	// (windows, debian, rhel, suse, appliance, ...). Family-level Go
	// hooks key off this value.
	Family string `yaml:"family" json:"family"`
	// Capabilities declares what this provider supports; the UI renders
	// its options purely from these flags (§10).
	Capabilities map[string]bool `yaml:"capabilities" json:"capabilities"`
	// AnswerFile describes the unattended-install answer file.
	AnswerFile AnswerFile `yaml:"answer_file" json:"answerFile"`
	// Detection contains the rules the ISO analyzer matches against.
	Detection Detection `yaml:"detection" json:"detection"`
	// Boot configures each supported boot method.
	Boot map[string]BootConfig `yaml:"boot" json:"boot"`
	// Notes documents known limitations of this provider (§8: where full
	// automation is not possible, prepare and document).
	Notes string `yaml:"notes" json:"notes,omitempty"`
}

// Well-known capability keys. Manifests may declare additional ones;
// unknown capabilities are preserved and exposed to the UI.
const (
	CapISO               = "iso"
	CapUnattendedInstall = "unattended_install"
	CapPXE               = "pxe"
	CapHTTPBoot          = "http_boot"
	CapUSBBoot           = "usb_boot"
	CapSecureBoot        = "secure_boot"
)

// AnswerFile describes the answer-file format of a provider.
type AnswerFile struct {
	// Format, e.g. "autounattend.xml", "autoinstall.yaml", "preseed",
	// "kickstart", "autoyast", "cloud-init", "answer.toml".
	Format string `yaml:"format" json:"format"`
	// Template is the path of the template file, relative to the
	// provider's directory.
	Template string `yaml:"template" json:"template,omitempty"`
}

// Detection holds ISO-recognition rules for the analyzer.
type Detection struct {
	// VolumeIDPatterns are glob patterns matched against the ISO 9660
	// volume identifier.
	VolumeIDPatterns []string `yaml:"volume_id_patterns" json:"volumeIdPatterns,omitempty"`
	// Files are paths that must exist inside the ISO.
	Files []string `yaml:"files" json:"files,omitempty"`
}

// BootConfig configures one boot method for a provider.
type BootConfig struct {
	// Kernel is the boot program or kernel to chainload (e.g. "wimboot",
	// "casper/vmlinuz").
	Kernel string `yaml:"kernel" json:"kernel,omitempty"`
	// Initrd lists initrd images, in load order.
	Initrd []string `yaml:"initrd" json:"initrd,omitempty"`
	// Cmdline is the kernel command line template.
	Cmdline string `yaml:"cmdline" json:"cmdline,omitempty"`
	// Requires names extra assets this method depends on (e.g. "winpe").
	Requires []string `yaml:"requires" json:"requires,omitempty"`
}

// Has reports whether the manifest declares the capability as true.
func (m *Manifest) Has(capability string) bool {
	return m.Capabilities[capability]
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Validate checks the structural invariants every manifest must satisfy.
func (m *Manifest) Validate() error {
	if !nameRe.MatchString(m.Name) {
		return fmt.Errorf("provider name %q must be lowercase letters, digits and dashes", m.Name)
	}
	if m.DisplayName == "" {
		return fmt.Errorf("provider %s: display_name is required", m.Name)
	}
	if m.Family == "" {
		return fmt.Errorf("provider %s: family is required", m.Name)
	}
	if len(m.Capabilities) == 0 {
		return fmt.Errorf("provider %s: at least one capability is required", m.Name)
	}
	for method := range m.Boot {
		switch method {
		case "pxe", "http_boot", "usb_boot":
		default:
			return fmt.Errorf("provider %s: unknown boot method %q", m.Name, method)
		}
	}
	return nil
}
