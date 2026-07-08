package backup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/kreuzbube88/helboot/backend/internal/db"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

func TestExportImportRoundtrip(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "helboot.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.Migrate(sqlDB); err != nil {
		t.Fatal(err)
	}
	st := store.New(sqlDB)
	if err := st.SetSetting("test.marker", "survives-backup"); err != nil {
		t.Fatal(err)
	}

	// Export.
	m := NewManager(sqlDB, dataDir, "test")
	var archive bytes.Buffer
	if err := m.Export(&archive); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if archive.Len() == 0 {
		t.Fatal("empty archive")
	}

	// Import stages the restore.
	if err := m.Import(bytes.NewReader(archive.Bytes())); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, pendingName)); err != nil {
		t.Fatalf("pending restore not staged: %v", err)
	}

	// Startup applies the pending restore and preserves the old DB.
	sqlDB.Close()
	applied, err := ApplyPendingRestore(dataDir, dbPath)
	if err != nil || !applied {
		t.Fatalf("ApplyPendingRestore: applied=%v err=%v", applied, err)
	}
	restored, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	v, err := store.New(restored).GetSetting("test.marker")
	if err != nil || v != "survives-backup" {
		t.Errorf("restored setting = %q, %v", v, err)
	}

	baks, _ := filepath.Glob(dbPath + ".bak-*")
	if len(baks) != 1 {
		t.Errorf("previous database not preserved: %v", baks)
	}
}

func TestImportRejectsGarbage(t *testing.T) {
	dataDir := t.TempDir()
	m := NewManager(nil, dataDir, "test")
	if err := m.Import(bytes.NewReader([]byte("not an archive"))); err == nil {
		t.Error("garbage accepted")
	}
}

func TestApplyPendingRestoreNoop(t *testing.T) {
	dataDir := t.TempDir()
	applied, err := ApplyPendingRestore(dataDir, filepath.Join(dataDir, "helboot.db"))
	if err != nil || applied {
		t.Errorf("expected noop, got applied=%v err=%v", applied, err)
	}
}
