package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Lease is one persisted DHCP lease (Mode B).
type Lease struct {
	MAC       string
	IP        string
	Hostname  string
	ExpiresAt time.Time
}

// UpsertLease creates or refreshes the lease for a MAC.
func (s *Store) UpsertLease(l Lease) error {
	_, err := s.db.Exec(
		`INSERT INTO dhcp_leases (mac, ip, hostname, expires_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (mac) DO UPDATE SET ip = excluded.ip, hostname = excluded.hostname,
		 expires_at = excluded.expires_at`,
		l.MAC, l.IP, l.Hostname, formatTime(l.ExpiresAt),
	)
	if err != nil {
		return fmt.Errorf("upsert lease: %w", err)
	}
	return nil
}

// ActiveLeases returns all leases that have not expired yet.
func (s *Store) ActiveLeases(now time.Time) ([]Lease, error) {
	rows, err := s.db.Query(
		`SELECT mac, ip, hostname, expires_at FROM dhcp_leases WHERE expires_at > ?`,
		formatTime(now),
	)
	if err != nil {
		return nil, fmt.Errorf("list leases: %w", err)
	}
	defer rows.Close()

	leases := []Lease{}
	for rows.Next() {
		var l Lease
		var expires string
		if err := rows.Scan(&l.MAC, &l.IP, &l.Hostname, &expires); err != nil {
			return nil, err
		}
		l.ExpiresAt = parseTime(expires)
		leases = append(leases, l)
	}
	return leases, rows.Err()
}

// LeaseByMAC returns the lease for a MAC (even if expired) or ErrNotFound.
func (s *Store) LeaseByMAC(mac string) (*Lease, error) {
	var l Lease
	var expires string
	err := s.db.QueryRow(
		`SELECT mac, ip, hostname, expires_at FROM dhcp_leases WHERE mac = ?`, mac,
	).Scan(&l.MAC, &l.IP, &l.Hostname, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lease by mac: %w", err)
	}
	l.ExpiresAt = parseTime(expires)
	return &l, nil
}

// DeleteLease releases a lease (DHCPRELEASE or manual cleanup).
func (s *Store) DeleteLease(mac string) error {
	_, err := s.db.Exec(`DELETE FROM dhcp_leases WHERE mac = ?`, mac)
	return err
}
