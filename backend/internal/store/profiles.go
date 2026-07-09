package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/model"
)

// CreateProfile inserts a profile together with its first version
// snapshot in one transaction.
func (s *Store) CreateProfile(name, provider string, isoID *int64, config string) (*model.Profile, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO profiles (name, provider, iso_id, current_version) VALUES (?, ?, ?, 1)`,
		name, provider, isoID,
	)
	if err != nil {
		return nil, fmt.Errorf("create profile: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(
		`INSERT INTO profile_versions (profile_id, version, config) VALUES (?, 1, ?)`,
		id, config,
	); err != nil {
		return nil, fmt.Errorf("create profile version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.ProfileByID(id)
}

// ListProfiles returns all profiles ordered by name.
func (s *Store) ListProfiles() ([]model.Profile, error) {
	rows, err := s.db.Query(profileSelect + ` ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer rows.Close()

	profiles := []model.Profile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, *p)
	}
	return profiles, rows.Err()
}

// ProfileByID returns one profile or ErrNotFound.
func (s *Store) ProfileByID(id int64) (*model.Profile, error) {
	rows, err := s.db.Query(profileSelect+` WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrNotFound
	}
	return scanProfile(rows)
}

// UpdateProfile updates profile metadata and applies a config change
// according to the explicit-versioning rules (ADR-0013): by default the
// head version is edited in place; with newVersion the config becomes
// an immutable version N+1. In-place edits are refused with
// ErrVersionInUse once any installation references the head version.
func (s *Store) UpdateProfile(id int64, name string, isoID *int64, config *string, newVersion bool) (*model.Profile, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE profiles SET name = ?, iso_id = ?, updated_at = ? WHERE id = ?`,
		name, isoID, formatTime(time.Now()), id,
	)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}

	if config != nil {
		if newVersion {
			err = appendProfileVersion(tx, id, *config)
		} else {
			err = editHeadVersion(tx, id, *config)
		}
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.ProfileByID(id)
}

// appendProfileVersion snapshots config as the next version and makes
// it the profile's current one.
func appendProfileVersion(tx *sql.Tx, profileID int64, config string) error {
	var next int
	if err := tx.QueryRow(
		`SELECT COALESCE(MAX(version), 0) + 1 FROM profile_versions WHERE profile_id = ?`, profileID,
	).Scan(&next); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO profile_versions (profile_id, version, config) VALUES (?, ?, ?)`,
		profileID, next, config,
	); err != nil {
		return fmt.Errorf("append profile version: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE profiles SET current_version = ? WHERE id = ?`, next, profileID,
	); err != nil {
		return err
	}
	return nil
}

// editHeadVersion overwrites the current version's config in place —
// only while no installation references it (history stays immutable).
func editHeadVersion(tx *sql.Tx, profileID int64, config string) error {
	var headID int64
	if err := tx.QueryRow(
		`SELECT pv.id FROM profile_versions pv
		 JOIN profiles p ON p.id = pv.profile_id AND p.current_version = pv.version
		 WHERE pv.profile_id = ?`, profileID,
	).Scan(&headID); err != nil {
		return fmt.Errorf("head version: %w", err)
	}
	var refs int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM installations WHERE profile_version_id = ?`, headID,
	).Scan(&refs); err != nil {
		return err
	}
	if refs > 0 {
		return ErrVersionInUse
	}
	if _, err := tx.Exec(
		`UPDATE profile_versions SET config = ? WHERE id = ?`, config, headID,
	); err != nil {
		return fmt.Errorf("edit profile version: %w", err)
	}
	return nil
}

// DeleteProfile removes a profile and all its versions (cascade).
func (s *Store) DeleteProfile(id int64) error {
	res, err := s.db.Exec(`DELETE FROM profiles WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ProfileVersions returns all version snapshots of a profile, newest last.
func (s *Store) ProfileVersions(profileID int64) ([]model.ProfileVersion, error) {
	rows, err := s.db.Query(
		`SELECT id, profile_id, version, config, answer_override, created_at FROM profile_versions
		 WHERE profile_id = ? ORDER BY version`, profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("list profile versions: %w", err)
	}
	defer rows.Close()

	versions := []model.ProfileVersion{}
	for rows.Next() {
		var v model.ProfileVersion
		var createdAt string
		if err := rows.Scan(&v.ID, &v.ProfileID, &v.Version, &v.Config, &v.AnswerOverride, &createdAt); err != nil {
			return nil, err
		}
		v.CreatedAt = parseTime(createdAt)
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// SetAnswerOverride stores (or, with empty content, clears) the manual
// answer-file override of a profile version (ADR-0014).
func (s *Store) SetAnswerOverride(profileID int64, version int, content string) error {
	res, err := s.db.Exec(
		`UPDATE profile_versions SET answer_override = ? WHERE profile_id = ? AND version = ?`,
		content, profileID, version,
	)
	if err != nil {
		return fmt.Errorf("set answer override: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

const profileSelect = `SELECT id, name, provider, iso_id, current_version, created_at, updated_at FROM profiles`

func scanProfile(rows *sql.Rows) (*model.Profile, error) {
	var p model.Profile
	var isoID sql.NullInt64
	var createdAt, updatedAt string
	err := rows.Scan(&p.ID, &p.Name, &p.Provider, &isoID, &p.CurrentVersion, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan profile: %w", err)
	}
	if isoID.Valid {
		p.ISOID = &isoID.Int64
	}
	p.CreatedAt = parseTime(createdAt)
	p.UpdatedAt = parseTime(updatedAt)
	return &p, nil
}
