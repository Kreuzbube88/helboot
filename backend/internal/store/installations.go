package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/model"
)

// CreateInstallation queues an installation for a host.
func (s *Store) CreateInstallation(hostID, profileVersionID int64, token string) (*model.Installation, error) {
	res, err := s.db.Exec(
		`INSERT INTO installations (host_id, profile_version_id, status, token)
		 VALUES (?, ?, 'waiting', ?)`,
		hostID, profileVersionID, token,
	)
	if err != nil {
		return nil, fmt.Errorf("create installation: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.InstallationByID(id)
}

// ListInstallations returns all installations, newest first.
func (s *Store) ListInstallations() ([]model.Installation, error) {
	rows, err := s.db.Query(installationSelect + ` ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list installations: %w", err)
	}
	defer rows.Close()

	installs := []model.Installation{}
	for rows.Next() {
		inst, err := scanInstallation(rows)
		if err != nil {
			return nil, err
		}
		installs = append(installs, *inst)
	}
	return installs, rows.Err()
}

// InstallationByID returns one installation or ErrNotFound.
func (s *Store) InstallationByID(id int64) (*model.Installation, error) {
	return s.oneInstallation(installationSelect+` WHERE id = ?`, id)
}

// InstallationByToken returns the installation carrying the boot token,
// or ErrNotFound.
func (s *Store) InstallationByToken(token string) (*model.Installation, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	return s.oneInstallation(installationSelect+` WHERE token = ?`, token)
}

// ActiveInstallationForHost returns the host's waiting or running
// installation, or ErrNotFound. At most one installation per host is
// active at any time (enforced by the API).
func (s *Store) ActiveInstallationForHost(hostID int64) (*model.Installation, error) {
	return s.oneInstallation(
		installationSelect+` WHERE host_id = ? AND status IN ('waiting', 'installing') ORDER BY id DESC`,
		hostID,
	)
}

// MarkInstallationStarted transitions waiting → installing.
func (s *Store) MarkInstallationStarted(id int64, now time.Time) error {
	_, err := s.db.Exec(
		`UPDATE installations SET status = 'installing', started_at = ? WHERE id = ? AND status = 'waiting'`,
		formatTime(now), id,
	)
	return err
}

// MarkInstallationFinished records the final state reported by the
// installer and appends to the installation log.
func (s *Store) MarkInstallationFinished(id int64, status model.InstallationStatus, logLine string, now time.Time) error {
	if status != model.InstallSuccess && status != model.InstallError {
		return fmt.Errorf("invalid final status %q", status)
	}
	_, err := s.db.Exec(
		`UPDATE installations SET status = ?, finished_at = ?, log = log || ? WHERE id = ?`,
		string(status), formatTime(now), logLine, id,
	)
	return err
}

// DeleteInstallation removes a queued installation. Only waiting
// installations may be deleted; running or finished ones are history.
func (s *Store) DeleteInstallation(id int64) error {
	res, err := s.db.Exec(`DELETE FROM installations WHERE id = ? AND status = 'waiting'`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

const installationSelect = `SELECT id, host_id, profile_version_id, status,
	started_at, finished_at, log, token, created_at FROM installations`

func (s *Store) oneInstallation(query string, args ...any) (*model.Installation, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrNotFound
	}
	return scanInstallation(rows)
}

func scanInstallation(rows *sql.Rows) (*model.Installation, error) {
	var inst model.Installation
	var status, createdAt string
	var startedAt, finishedAt sql.NullString
	err := rows.Scan(&inst.ID, &inst.HostID, &inst.ProfileVersionID, &status,
		&startedAt, &finishedAt, &inst.Log, &inst.Token, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan installation: %w", err)
	}
	inst.Status = model.InstallationStatus(status)
	inst.CreatedAt = parseTime(createdAt)
	if startedAt.Valid {
		t := parseTime(startedAt.String)
		inst.StartedAt = &t
	}
	if finishedAt.Valid {
		t := parseTime(finishedAt.String)
		inst.FinishedAt = &t
	}
	return &inst, nil
}

// ProfileVersionByID returns one profile version snapshot.
func (s *Store) ProfileVersionByID(id int64) (*model.ProfileVersion, error) {
	return s.oneProfileVersion(`WHERE id = ?`, id)
}

// ProfileVersionNumber returns a specific version snapshot of a profile.
func (s *Store) ProfileVersionNumber(profileID int64, version int) (*model.ProfileVersion, error) {
	return s.oneProfileVersion(`WHERE profile_id = ? AND version = ?`, profileID, version)
}

func (s *Store) oneProfileVersion(where string, args ...any) (*model.ProfileVersion, error) {
	var v model.ProfileVersion
	var createdAt string
	err := s.db.QueryRow(
		`SELECT id, profile_id, version, config, answer_override, created_at FROM profile_versions `+where,
		args...,
	).Scan(&v.ID, &v.ProfileID, &v.Version, &v.Config, &v.AnswerOverride, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("profile version: %w", err)
	}
	v.CreatedAt = parseTime(createdAt)
	return &v, nil
}
