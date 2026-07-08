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

// CountAdmins returns the number of administrator accounts; used to
// protect the last admin from demotion or deletion.
func (s *Store) CountAdmins() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&n)
	return n, err
}

// ListUsers returns all accounts ordered by username.
func (s *Store) ListUsers() ([]model.User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, password_hash, role, locale, created_at FROM users ORDER BY username`,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := []model.User{}
	for rows.Next() {
		var u model.User
		var role, createdAt string
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &u.Locale, &createdAt); err != nil {
			return nil, err
		}
		u.Role = model.Role(role)
		u.CreatedAt = parseTime(createdAt)
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpdateUserRole changes a user's role.
func (s *Store) UpdateUserRole(id int64, role model.Role) error {
	res, err := s.db.Exec(`UPDATE users SET role = ? WHERE id = ?`, string(role), id)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateUserPassword replaces a user's password hash and revokes all of
// their sessions — a changed password must log out every device.
func (s *Store) UpdateUserPassword(id int64, passwordHash string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return tx.Commit()
}

// DeleteUser removes an account; its sessions cascade away.
func (s *Store) DeleteUser(id int64) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddAudit appends an audit-log entry (§29: failed logins and privileged
// actions are traceable).
func (s *Store) AddAudit(userID *int64, action, entity, entityID string) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_log (user_id, action, entity, entity_id) VALUES (?, ?, ?, ?)`,
		userID, action, entity, entityID,
	)
	return err
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
