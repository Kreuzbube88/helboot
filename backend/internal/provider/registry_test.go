package provider

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func writeManifest(t *testing.T, dir, name, content string) {
	t.Helper()
	pdir := filepath.Join(dir, name)
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "provider.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const validManifest = `
name: debian
display_name: "Debian"
family: debian
capabilities:
  iso: true
  unattended_install: true
  pxe: true
answer_file:
  format: preseed
  template: templates/preseed.cfg.tmpl
detection:
  volume_id_patterns: ["Debian *"]
boot:
  pxe:
    kernel: install.amd/vmlinuz
    initrd: [install.amd/initrd.gz]
`

func TestLoadDirValid(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "debian", validManifest)

	reg, err := LoadDir(dir, discardLogger())
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	m := reg.Get("debian")
	if m == nil {
		t.Fatal("debian provider not loaded")
	}
	if !m.Has(CapPXE) {
		t.Error("pxe capability not detected")
	}
	if m.Has(CapSecureBoot) {
		t.Error("undeclared capability reported as true")
	}
	if m.AnswerFile.Format != "preseed" {
		t.Errorf("answer file format = %q", m.AnswerFile.Format)
	}
}

func TestLoadDirSkipsInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "debian", validManifest)
	// Missing display_name and family → must be skipped, not fatal.
	writeManifest(t, dir, "broken", "name: broken\ncapabilities: {iso: true}\n")
	// Name/directory mismatch → must be skipped.
	writeManifest(t, dir, "mismatch", validManifest)

	reg, err := LoadDir(dir, discardLogger())
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if got := len(reg.All()); got != 1 {
		t.Errorf("loaded %d providers, want 1 (only the valid one)", got)
	}
}

func TestLoadDirMissingDirectory(t *testing.T) {
	reg, err := LoadDir(filepath.Join(t.TempDir(), "nope"), discardLogger())
	if err != nil {
		t.Fatalf("missing directory must not be fatal: %v", err)
	}
	if len(reg.All()) != 0 {
		t.Error("expected empty registry")
	}
}

func TestValidateRejectsBadNames(t *testing.T) {
	m := &Manifest{Name: "Bad Name!", DisplayName: "x", Family: "x",
		Capabilities: map[string]bool{"iso": true}}
	if err := m.Validate(); err == nil {
		t.Error("expected validation error for bad name")
	}
}

func TestValidateRejectsUnknownBootMethod(t *testing.T) {
	m := &Manifest{Name: "ok", DisplayName: "x", Family: "x",
		Capabilities: map[string]bool{"iso": true},
		Boot:         map[string]BootConfig{"carrier-pigeon": {}}}
	if err := m.Validate(); err == nil {
		t.Error("expected validation error for unknown boot method")
	}
}
