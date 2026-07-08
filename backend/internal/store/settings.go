package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Well-known settings keys.
const (
	SettingSetupCompleted = "setup.completed" // "true" once the wizard finished
	SettingNetworkMode    = "network.mode"    // "proxy_dhcp" or "dhcp"
	SettingUILanguage     = "ui.language"     // default UI language
)

// GetSetting returns the value for key, or ErrNotFound.
func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get setting %s: %w", key, err)
	}
	return value, nil
}

// SetSetting inserts or updates a setting.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}

// SetupCompleted reports whether the first-run wizard has finished.
func (s *Store) SetupCompleted() (bool, error) {
	v, err := s.GetSetting(SettingSetupCompleted)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v == "true", nil
}
