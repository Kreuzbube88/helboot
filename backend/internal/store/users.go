package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/kreuzbube88/helboot/backend/internal/model"
)

// CreateUser inserts a new local account and returns it with its ID.
func (s *Store) CreateUser(username, passwordHash string, role model.Role, locale string) (*model.User, error) {
	res, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, role, locale) VALUES (?, ?, ?, ?)`,
		username, passwordHash, string(role), locale,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.UserByID(id)
}

// UserByID returns the user with the given ID or ErrNotFound.
func (s *Store) UserByID(id int64) (*model.User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, username, password_hash, role, locale, created_at FROM users WHERE id = ?`, id,
	))
}

// UserByUsername returns the user with the given (case-insensitive)
// username or ErrNotFound.
func (s *Store) UserByUsername(username string) (*model.User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, username, password_hash, role, locale, created_at FROM users WHERE username = ?`, username,
	))
}

// CountUsers returns the number of local accounts. Zero means the
// first-run wizard has not created the administrator yet.
func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) scanUser(row *sql.Row) (*model.User, error) {
	var u model.User
	var role, createdAt string
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &u.Locale, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.Role = model.Role(role)
	u.CreatedAt = parseTime(createdAt)
	return &u, nil
}
