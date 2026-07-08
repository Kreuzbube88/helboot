// Package db opens the SQLite database and applies embedded schema
// migrations (ADR-0004). SQLite is accessed through the pure-Go driver
// modernc.org/sqlite so the binary stays cgo-free.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Open opens (creating if necessary) the SQLite database at path and
// configures it for server use: WAL journal, foreign keys, busy timeout.
// SQLite allows only one writer, so the pool is capped at a single
// connection; at homelab scale this is not a bottleneck (ADR-0004).
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return sqlDB, nil
}

// Migrate applies all embedded migrations that have not run yet, in
// filename order. Applied migrations are tracked in schema_migrations;
// each migration runs inside its own transaction.
func Migrate(sqlDB *sql.DB) error {
	if _, err := sqlDB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := migrationNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		applied, err := isApplied(sqlDB, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := apply(sqlDB, name); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

func migrationNames() ([]string, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func isApplied(sqlDB *sql.DB, name string) (bool, error) {
	var n int
	err := sqlDB.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}
	return n > 0, nil
}

func apply(sqlDB *sql.DB, name string) error {
	content, err := migrationFS.ReadFile("migrations/" + name)
	if err != nil {
		return err
	}
	tx, err := sqlDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(string(content)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, name); err != nil {
		return err
	}
	return tx.Commit()
}
