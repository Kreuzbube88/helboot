package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/kreuzbube88/helboot/backend/internal/model"
)

// CreateISO inserts an ISO record.
func (s *Store) CreateISO(img model.ISOImage) (*model.ISOImage, error) {
	res, err := s.db.Exec(
		`INSERT INTO iso_images (filename, provider, os_name, version, arch, bootloader,
		 install_method, size_bytes, sha256, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		img.Filename, img.Provider, img.OSName, img.Version, img.Arch, img.Bootloader,
		img.InstallMethod, img.SizeBytes, img.SHA256, img.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("create iso: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.ISOByID(id)
}

// ListISOs returns all ISO records ordered by filename.
func (s *Store) ListISOs() ([]model.ISOImage, error) {
	rows, err := s.db.Query(isoSelect + ` ORDER BY filename`)
	if err != nil {
		return nil, fmt.Errorf("list isos: %w", err)
	}
	defer rows.Close()

	images := []model.ISOImage{}
	for rows.Next() {
		img, err := scanISO(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, *img)
	}
	return images, rows.Err()
}

// ISOByID returns one ISO record or ErrNotFound.
func (s *Store) ISOByID(id int64) (*model.ISOImage, error) {
	rows, err := s.db.Query(isoSelect+` WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrNotFound
	}
	return scanISO(rows)
}

// ISOByFilename returns the record for filename or ErrNotFound.
func (s *Store) ISOByFilename(filename string) (*model.ISOImage, error) {
	rows, err := s.db.Query(isoSelect+` WHERE filename = ?`, filename)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrNotFound
	}
	return scanISO(rows)
}

// DeleteISO removes an ISO record. Profiles referencing it keep working
// (iso_id is set NULL by the schema).
func (s *Store) DeleteISO(id int64) error {
	res, err := s.db.Exec(`DELETE FROM iso_images WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

const isoSelect = `SELECT id, filename, provider, os_name, version, arch, bootloader,
	install_method, size_bytes, sha256, status, created_at FROM iso_images`

func scanISO(rows *sql.Rows) (*model.ISOImage, error) {
	var img model.ISOImage
	var createdAt string
	err := rows.Scan(&img.ID, &img.Filename, &img.Provider, &img.OSName, &img.Version,
		&img.Arch, &img.Bootloader, &img.InstallMethod, &img.SizeBytes, &img.SHA256,
		&img.Status, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan iso: %w", err)
	}
	img.CreatedAt = parseTime(createdAt)
	return &img, nil
}
