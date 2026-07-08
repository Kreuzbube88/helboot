// Package iso manages the ISO library (§11): original images are never
// modified, only stored, hashed and analyzed. Detection matches the
// declarative rules from the provider manifests (ADR-0005).
package iso

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kdomanski/iso9660"

	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/provider"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// ErrInvalidFilename is returned for unsafe or unsupported filenames.
var ErrInvalidFilename = errors.New("iso: invalid filename")

// ErrExists is returned when a file with the same name already exists.
var ErrExists = errors.New("iso: file already exists")

var filenameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*\.(iso|img)$`)

// Manager owns the ISO directory and its database records.
type Manager struct {
	log      *slog.Logger
	dir      string
	store    *store.Store
	registry *provider.Registry
}

// NewManager creates the ISO manager for dir.
func NewManager(log *slog.Logger, dir string, st *store.Store, reg *provider.Registry) *Manager {
	return &Manager{log: log, dir: dir, store: st, registry: reg}
}

// Dir returns the ISO storage directory.
func (m *Manager) Dir() string { return m.dir }

// Import streams a new ISO into the library, hashing while writing,
// then analyzes it and records the result.
func (m *Manager) Import(filename string, r io.Reader) (*model.ISOImage, error) {
	// The raw name is validated as-is: the pattern permits no path
	// separators, so traversal attempts are rejected, not silently fixed.
	if !filenameRe.MatchString(filename) {
		return nil, ErrInvalidFilename
	}
	target := filepath.Join(m.dir, filename)
	if _, err := os.Stat(target); err == nil {
		return nil, ErrExists
	}
	if _, err := m.store.ISOByFilename(filename); err == nil {
		return nil, ErrExists
	}

	// Stream to a temp file in the same directory so the final rename is
	// atomic and a failed upload never leaves a half ISO behind.
	tmp, err := os.CreateTemp(m.dir, ".upload-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hasher), r)
	if err != nil {
		tmp.Close()
		return nil, fmt.Errorf("write upload: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return nil, fmt.Errorf("finalize upload: %w", err)
	}

	img := m.record(filename, size, hex.EncodeToString(hasher.Sum(nil)))
	return m.store.CreateISO(img)
}

// ScanDir indexes ISO files already present in the directory (e.g. an
// existing Unraid share mounted into the container) that have no
// database record yet. Returns the newly added images.
func (m *Manager) ScanDir() ([]model.ISOImage, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, fmt.Errorf("read iso directory: %w", err)
	}
	added := []model.ISOImage{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !filenameRe.MatchString(name) {
			continue
		}
		if _, err := m.store.ISOByFilename(name); err == nil {
			continue // already indexed
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		sum, err := hashFile(filepath.Join(m.dir, name))
		if err != nil {
			m.log.Warn("iso: cannot hash file, skipping", "file", name, "error", err)
			continue
		}
		img := m.record(name, info.Size(), sum)
		created, err := m.store.CreateISO(img)
		if err != nil {
			return nil, err
		}
		added = append(added, *created)
	}
	return added, nil
}

// Delete removes the database record and the file itself.
func (m *Manager) Delete(id int64) error {
	img, err := m.store.ISOByID(id)
	if err != nil {
		return err
	}
	if err := m.store.DeleteISO(id); err != nil {
		return err
	}
	path := filepath.Join(m.dir, filepath.Base(img.Filename))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		m.log.Warn("iso: record removed but file deletion failed", "file", img.Filename, "error", err)
	}
	return nil
}

// record builds the database row for a stored file, including the
// analysis result. Analysis failure never fails the import — the image
// is stored as "unsupported" and can be re-analyzed after a provider
// update.
func (m *Manager) record(filename string, size int64, sha string) model.ISOImage {
	img := model.ISOImage{
		Filename:  filename,
		SizeBytes: size,
		SHA256:    sha,
		Status:    "unsupported",
	}
	analysis, err := m.analyze(filepath.Join(m.dir, filename))
	if err != nil {
		m.log.Warn("iso: analysis failed", "file", filename, "error", err)
		return img
	}
	img.OSName = analysis.VolumeID
	img.Bootloader = analysis.Bootloader
	if analysis.Provider != nil {
		img.Provider = analysis.Provider.Name
		img.OSName = analysis.Provider.DisplayName
		img.InstallMethod = analysis.Provider.AnswerFile.Format
		img.Status = "ready"
	}
	return img
}

// analysis is the result of inspecting one image read-only.
type analysis struct {
	VolumeID   string
	Bootloader string
	Provider   *provider.Manifest
}

// analyze opens the image read-only and matches it against every
// provider's detection rules.
func (m *Manager) analyze(path string) (*analysis, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, err := iso9660.OpenImage(f)
	if err != nil {
		return nil, fmt.Errorf("not an ISO 9660 image: %w", err)
	}
	volumeID, err := img.Label()
	if err != nil {
		return nil, fmt.Errorf("read volume id: %w", err)
	}
	volumeID = strings.TrimSpace(volumeID)

	result := &analysis{VolumeID: volumeID}
	result.Bootloader = detectBootloader(img)
	result.Provider = m.match(img, volumeID)
	return result, nil
}

// match returns the first provider whose detection rules fit the image.
// Volume-ID patterns are checked first: they are cheap and also work for
// Windows ISOs, whose file tree lives in UDF/Joliet namespaces the plain
// ISO 9660 reader cannot fully resolve. File rules require every listed
// file to exist.
func (m *Manager) match(img *iso9660.Image, volumeID string) *provider.Manifest {
	for _, manifest := range m.registry.All() {
		for _, pattern := range manifest.Detection.VolumeIDPatterns {
			if ok, err := path.Match(pattern, volumeID); err == nil && ok {
				return manifest
			}
		}
	}
	for _, manifest := range m.registry.All() {
		files := manifest.Detection.Files
		if len(files) == 0 {
			continue
		}
		all := true
		for _, file := range files {
			if !fileExists(img, file) {
				all = false
				break
			}
		}
		if all {
			return manifest
		}
	}
	return nil
}

// detectBootloader classifies the boot layout from well-known paths.
func detectBootloader(img *iso9660.Image) string {
	efi := fileExists(img, "EFI") || fileExists(img, "efi")
	bios := fileExists(img, "isolinux") || fileExists(img, "boot/syslinux") || fileExists(img, "boot/grub")
	switch {
	case efi && bios:
		return "hybrid"
	case efi:
		return "uefi"
	case bios:
		return "bios"
	default:
		return ""
	}
}

// fileExists walks the ISO directory tree for a /-separated path,
// comparing names case-insensitively and ignoring ISO 9660 ";1" version
// suffixes.
func fileExists(img *iso9660.Image, p string) bool {
	current, err := img.RootDir()
	if err != nil {
		return false
	}
	for _, segment := range strings.Split(strings.Trim(p, "/"), "/") {
		if !current.IsDir() {
			return false
		}
		children, err := current.GetChildren()
		if err != nil {
			return false
		}
		var next *iso9660.File
		for _, child := range children {
			name := strings.TrimSuffix(child.Name(), ";1")
			if strings.EqualFold(name, segment) {
				next = child
				break
			}
		}
		if next == nil {
			return false
		}
		current = next
	}
	return true
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
