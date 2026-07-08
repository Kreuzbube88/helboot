package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/model"
)

// CreateHost inserts a host. The MAC must already be normalized
// (see netutil.NormalizeMAC); uniqueness is enforced by the schema.
func (s *Store) CreateHost(h model.Host) (*model.Host, error) {
	tags, err := json.Marshal(orEmpty(h.Tags))
	if err != nil {
		return nil, err
	}
	res, err := s.db.Exec(
		`INSERT INTO hosts (mac, hostname, vendor, model, serial, asset_id, tags, firmware, arch, profile_id, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.MAC, h.Hostname, h.Vendor, h.Model, h.Serial, h.AssetID, string(tags),
		h.Firmware, h.Arch, h.ProfileID, string(h.Status),
	)
	if err != nil {
		return nil, fmt.Errorf("create host: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.HostByID(id)
}

// ListHosts returns all hosts ordered by creation time.
func (s *Store) ListHosts() ([]model.Host, error) {
	rows, err := s.db.Query(hostSelect + ` ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}
	defer rows.Close()

	hosts := []model.Host{}
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, *h)
	}
	return hosts, rows.Err()
}

// HostByID returns one host or ErrNotFound.
func (s *Store) HostByID(id int64) (*model.Host, error) {
	rows, err := s.db.Query(hostSelect+` WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrNotFound
	}
	return scanHost(rows)
}

// HostByMAC returns the host with the given normalized MAC or ErrNotFound.
func (s *Store) HostByMAC(mac string) (*model.Host, error) {
	rows, err := s.db.Query(hostSelect+` WHERE mac = ?`, mac)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrNotFound
	}
	return scanHost(rows)
}

// UpdateHost overwrites the mutable fields of an existing host.
func (s *Store) UpdateHost(h model.Host) (*model.Host, error) {
	tags, err := json.Marshal(orEmpty(h.Tags))
	if err != nil {
		return nil, err
	}
	res, err := s.db.Exec(
		`UPDATE hosts SET mac = ?, hostname = ?, vendor = ?, model = ?, serial = ?, asset_id = ?,
		 tags = ?, firmware = ?, arch = ?, profile_id = ?, status = ?, updated_at = ?
		 WHERE id = ?`,
		h.MAC, h.Hostname, h.Vendor, h.Model, h.Serial, h.AssetID, string(tags),
		h.Firmware, h.Arch, h.ProfileID, string(h.Status), formatTime(time.Now()), h.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update host: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.HostByID(h.ID)
}

// DeleteHost removes a host and (via cascade) its installation history.
func (s *Store) DeleteHost(id int64) error {
	res, err := s.db.Exec(`DELETE FROM hosts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

const hostSelect = `SELECT id, mac, hostname, vendor, model, serial, asset_id, tags,
	firmware, arch, profile_id, status, created_at, updated_at FROM hosts`

func scanHost(rows *sql.Rows) (*model.Host, error) {
	var h model.Host
	var tags, status, createdAt, updatedAt string
	var profileID sql.NullInt64
	err := rows.Scan(&h.ID, &h.MAC, &h.Hostname, &h.Vendor, &h.Model, &h.Serial,
		&h.AssetID, &tags, &h.Firmware, &h.Arch, &profileID, &status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan host: %w", err)
	}
	if err := json.Unmarshal([]byte(tags), &h.Tags); err != nil {
		h.Tags = []string{}
	}
	if profileID.Valid {
		h.ProfileID = &profileID.Int64
	}
	h.Status = model.HostStatus(status)
	h.CreatedAt = parseTime(createdAt)
	h.UpdatedAt = parseTime(updatedAt)
	return &h, nil
}

func orEmpty(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
