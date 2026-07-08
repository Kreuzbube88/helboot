package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/model"
)

// CreateSession persists a new session row.
func (s *Store) CreateSession(sess model.Session) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (token, user_id, csrf_token, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		sess.Token, sess.UserID, sess.CSRFToken, formatTime(sess.CreatedAt), formatTime(sess.ExpiresAt),
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionByToken returns the unexpired session for token, or ErrNotFound.
func (s *Store) SessionByToken(token string, now time.Time) (*model.Session, error) {
	var sess model.Session
	var createdAt, expiresAt string
	err := s.db.QueryRow(
		`SELECT token, user_id, csrf_token, created_at, expires_at FROM sessions WHERE token = ?`, token,
	).Scan(&sess.Token, &sess.UserID, &sess.CSRFToken, &createdAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}
	sess.CreatedAt = parseTime(createdAt)
	sess.ExpiresAt = parseTime(expiresAt)
	if !now.UTC().Before(sess.ExpiresAt) {
		return nil, ErrNotFound
	}
	return &sess, nil
}

// DeleteSession removes a session (logout).
func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// DeleteExpiredSessions removes all sessions past their expiry. Called
// periodically by the server's housekeeping loop.
func (s *Store) DeleteExpiredSessions(now time.Time) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, formatTime(now))
	return err
}
