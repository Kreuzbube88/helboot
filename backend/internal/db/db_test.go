package db

import (
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()

	if err := Migrate(sqlDB); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// All core tables must exist after migration.
	for _, table := range []string{
		"users", "sessions", "settings", "iso_images",
		"profiles", "profile_versions", "hosts", "installations", "audit_log",
	} {
		var n int
		err := sqlDB.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table,
		).Scan(&n)
		if err != nil || n != 1 {
			t.Errorf("table %q missing after migration (err=%v)", table, err)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()

	if err := Migrate(sqlDB); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := Migrate(sqlDB); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}
