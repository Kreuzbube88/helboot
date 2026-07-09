// Package store implements HELBOOT's persistence layer on top of the
// SQLite database. It is the only package that writes SQL; core and API
// code go through these repositories.
package store

import (
	"database/sql"
	"errors"
	"time"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("store: not found")

// ErrVersionInUse is returned when an in-place profile edit would
// change a version that an installation references (ADR-0013).
var ErrVersionInUse = errors.New("store: profile version is referenced by an installation")

// timeFormat matches SQLite's datetime('now') output (UTC).
const timeFormat = "2006-01-02 15:04:05"

// Store wraps the SQLite handle with typed repository methods.
type Store struct {
	db *sql.DB
}

// New creates a Store on an already-opened and migrated database.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func parseTime(s string) time.Time {
	t, err := time.Parse(timeFormat, s)
	if err != nil {
		// Also accept RFC3339, the format we write ourselves.
		if t2, err2 := time.Parse(time.RFC3339, s); err2 == nil {
			return t2.UTC()
		}
		return time.Time{}
	}
	return t.UTC()
}

func formatTime(t time.Time) string {
	return t.UTC().Format(timeFormat)
}
