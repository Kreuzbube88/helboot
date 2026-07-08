// Package backup implements full-state export and import (§31). The
// SQLite database contains everything HELBOOT owns — settings, users,
// hosts, profiles with all versions, installation history and ISO
// checksums — so a consistent database snapshot plus a manifest is a
// complete backup. ISO files themselves are excluded (size); their
// recorded checksums let the UI report missing images after a restore.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// pendingName is the staged restore file inside the data directory.
// The running process never swaps its own live database; the swap
// happens on the next startup (see ApplyPendingRestore).
const pendingName = "restore-pending.db"

// manifest describes a backup archive.
type manifest struct {
	Application string    `json:"application"`
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Manager performs exports and imports for one data directory.
type Manager struct {
	db      *sql.DB
	dataDir string
	version string
}

// NewManager creates a backup manager.
func NewManager(db *sql.DB, dataDir, version string) *Manager {
	return &Manager{db: db, dataDir: dataDir, version: version}
}

// Export streams a tar.gz archive with a consistent database snapshot.
func (m *Manager) Export(w io.Writer) error {
	snapshot := filepath.Join(m.dataDir, fmt.Sprintf(".export-%d.db", time.Now().UnixNano()))
	defer os.Remove(snapshot)
	// VACUUM INTO produces a consistent copy even while writers are
	// active — SQLite's online backup in one statement (ADR-0004).
	if _, err := m.db.Exec(`VACUUM INTO ?`, snapshot); err != nil {
		return fmt.Errorf("snapshot database: %w", err)
	}

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	meta, err := json.MarshalIndent(manifest{
		Application: "helboot",
		Version:     m.version,
		CreatedAt:   time.Now().UTC(),
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := writeTarFile(tw, "manifest.json", meta); err != nil {
		return err
	}
	if err := writeTarFileFromDisk(tw, "helboot.db", snapshot); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// Import stages a backup archive for restore. The database from the
// archive is validated and written next to the live one; the swap
// happens at the next startup so no open connections are pulled away.
func (m *Manager) Import(r io.Reader) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("not a gzip archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var sawManifest, sawDB bool
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		switch filepath.Clean(hdr.Name) {
		case "manifest.json":
			var meta manifest
			if err := json.NewDecoder(io.LimitReader(tr, 1<<20)).Decode(&meta); err != nil {
				return fmt.Errorf("invalid manifest: %w", err)
			}
			if meta.Application != "helboot" {
				return fmt.Errorf("archive is not a HELBOOT backup")
			}
			sawManifest = true
		case "helboot.db":
			if err := m.stageDatabase(tr); err != nil {
				return err
			}
			sawDB = true
		default:
			// Unknown entries are ignored for forward compatibility.
		}
	}
	if !sawManifest || !sawDB {
		return fmt.Errorf("archive is missing manifest.json or helboot.db")
	}
	return nil
}

// stageDatabase writes the incoming database to the pending path after
// validating it actually is a usable SQLite database.
func (m *Manager) stageDatabase(r io.Reader) error {
	tmp, err := os.CreateTemp(m.dataDir, ".import-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return fmt.Errorf("write database: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := validateSQLite(tmp.Name()); err != nil {
		return fmt.Errorf("archive database is not usable: %w", err)
	}
	return os.Rename(tmp.Name(), filepath.Join(m.dataDir, pendingName))
}

// validateSQLite opens the file and runs an integrity check plus a
// sanity check that it is a HELBOOT schema.
func validateSQLite(path string) error {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}
	var n int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`,
	).Scan(&n); err != nil || n != 1 {
		return fmt.Errorf("missing schema_migrations table")
	}
	return nil
}

// ApplyPendingRestore swaps a staged restore into place. Called at
// startup BEFORE the database is opened. The previous database is kept
// as a timestamped .bak file.
func ApplyPendingRestore(dataDir, dbPath string) (bool, error) {
	pending := filepath.Join(dataDir, pendingName)
	if _, err := os.Stat(pending); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if _, err := os.Stat(dbPath); err == nil {
		backupPath := fmt.Sprintf("%s.bak-%s", dbPath, time.Now().UTC().Format("20060102-150405"))
		if err := os.Rename(dbPath, backupPath); err != nil {
			return false, fmt.Errorf("preserve current database: %w", err)
		}
		// Stale WAL/SHM files must not be replayed into the restored DB.
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	}
	if err := os.Rename(pending, dbPath); err != nil {
		return false, fmt.Errorf("activate restored database: %w", err)
	}
	return true, nil
}

func writeTarFile(tw *tar.Writer, name string, content []byte) error {
	if err := tw.WriteHeader(&tar.Header{
		Name: name, Mode: 0o600, Size: int64(len(content)), ModTime: time.Now(),
	}); err != nil {
		return err
	}
	_, err := tw.Write(content)
	return err
}

func writeTarFileFromDisk(tw *tar.Writer, name, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{
		Name: name, Mode: 0o600, Size: info.Size(), ModTime: time.Now(),
	}); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}
