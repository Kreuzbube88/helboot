package iso

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kdomanski/iso9660"

	"github.com/kreuzbube88/helboot/backend/internal/db"
	"github.com/kreuzbube88/helboot/backend/internal/provider"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// makeISO builds a real (tiny) ISO 9660 image for the analyzer to chew on.
func makeISO(t *testing.T, volumeID string, files map[string]string) []byte {
	t.Helper()
	w, err := iso9660.NewWriter()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Cleanup()
	for path, content := range files {
		if err := w.AddFile(strings.NewReader(content), path); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := w.WriteTo(&buf, volumeID); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testManager(t *testing.T) (*Manager, *store.Store) {
	t.Helper()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.Migrate(sqlDB); err != nil {
		t.Fatal(err)
	}
	st := store.New(sqlDB)

	// Providers: one matching by volume ID, one by marker files.
	providersDir := t.TempDir()
	writeProvider(t, providersDir, "debian", `
name: debian
display_name: "Debian"
family: debian
capabilities: {iso: true, pxe: true}
answer_file: {format: preseed}
detection:
  volume_id_patterns: ["Debian *"]
`)
	writeProvider(t, providersDir, "ubuntu", `
name: ubuntu
display_name: "Ubuntu"
family: debian
capabilities: {iso: true, pxe: true}
answer_file: {format: autoinstall.yaml}
detection:
  files: ["casper/vmlinuz", ".disk/info"]
`)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry, err := provider.LoadDir(providersDir, log)
	if err != nil {
		t.Fatal(err)
	}
	return NewManager(log, t.TempDir(), st, registry), st
}

func writeProvider(t *testing.T, dir, name, manifest string) {
	t.Helper()
	pdir := filepath.Join(dir, name)
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "provider.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImportDetectsByVolumeID(t *testing.T) {
	m, _ := testManager(t)
	data := makeISO(t, "Debian 13.1.0 amd64 n", map[string]string{"README": "hi"})

	img, err := m.Import("debian-13.iso", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if img.Provider != "debian" {
		t.Errorf("provider = %q, want debian", img.Provider)
	}
	if img.Status != "ready" {
		t.Errorf("status = %q, want ready", img.Status)
	}
	if img.InstallMethod != "preseed" {
		t.Errorf("install method = %q, want preseed", img.InstallMethod)
	}
	if img.SHA256 == "" || img.SizeBytes == 0 {
		t.Error("hash/size not recorded")
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "debian-13.iso")); err != nil {
		t.Errorf("file not stored: %v", err)
	}
}

func TestImportDetectsByMarkerFiles(t *testing.T) {
	m, _ := testManager(t)
	data := makeISO(t, "CUSTOM UBUNTU REMIX", map[string]string{
		"casper/vmlinuz": "kernel",
		".disk/info":     "Ubuntu-Server 24.04",
	})
	img, err := m.Import("ubuntu.iso", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if img.Provider != "ubuntu" {
		t.Errorf("provider = %q, want ubuntu", img.Provider)
	}
}

func TestImportUnknownISOIsKeptAsUnsupported(t *testing.T) {
	m, _ := testManager(t)
	data := makeISO(t, "MYSTERY OS", map[string]string{"stuff": "x"})
	img, err := m.Import("mystery.iso", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if img.Status != "unsupported" || img.Provider != "" {
		t.Errorf("unknown ISO should be unsupported, got status=%q provider=%q", img.Status, img.Provider)
	}
	// The volume ID is preserved so the user can identify the file.
	if img.OSName != "MYSTERY OS" {
		t.Errorf("osName = %q, want volume id", img.OSName)
	}
}

func TestImportRejectsBadFilenames(t *testing.T) {
	m, _ := testManager(t)
	for _, name := range []string{"../evil.iso", "no-extension", "bad*.iso", ".hidden.iso"} {
		if _, err := m.Import(name, strings.NewReader("x")); err == nil {
			t.Errorf("filename %q was accepted", name)
		}
	}
}

func TestImportRejectsDuplicates(t *testing.T) {
	m, _ := testManager(t)
	data := makeISO(t, "Debian 13", nil)
	if _, err := m.Import("dup.iso", bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Import("dup.iso", bytes.NewReader(data)); err != ErrExists {
		t.Errorf("duplicate import: err = %v, want ErrExists", err)
	}
}

func TestScanDirIndexesExistingFiles(t *testing.T) {
	m, _ := testManager(t)
	data := makeISO(t, "Debian 13 scan", nil)
	if err := os.WriteFile(filepath.Join(m.Dir(), "preexisting.iso"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.Dir(), "notes.txt"), []byte("skip me"), 0o644); err != nil {
		t.Fatal(err)
	}

	added, err := m.ScanDir()
	if err != nil {
		t.Fatalf("ScanDir: %v", err)
	}
	if len(added) != 1 || added[0].Filename != "preexisting.iso" {
		t.Fatalf("added = %+v, want just preexisting.iso", added)
	}
	if added[0].Provider != "debian" {
		t.Errorf("scan did not analyze: provider = %q", added[0].Provider)
	}

	// Second scan adds nothing.
	again, err := m.ScanDir()
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("rescan added %d images, want 0", len(again))
	}
}

func TestDeleteRemovesRecordAndFile(t *testing.T) {
	m, st := testManager(t)
	data := makeISO(t, "Debian 13 del", nil)
	img, err := m.Import("delete-me.iso", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Delete(img.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := st.ISOByID(img.ID); err != store.ErrNotFound {
		t.Error("record still present")
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "delete-me.iso")); !os.IsNotExist(err) {
		t.Error("file still present")
	}
}
