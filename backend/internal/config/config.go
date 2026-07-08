// Package config loads HELBOOT's runtime configuration from the
// environment. All variables use the HELBOOT_ prefix; every value has a
// default suitable for local development.
package config

import (
	"os"
	"path/filepath"
)

// Config holds all runtime configuration for the HELBOOT server.
type Config struct {
	// DataDir is the single state directory: database, ISOs, generated
	// assets, logs and secrets all live below it.
	DataDir string
	// HTTPAddr is the listen address for the web UI, API and HTTP boot.
	HTTPAddr string
	// ProvidersDir contains the provider manifests (provider.yaml files).
	ProvidersDir string
	// AssetsDir holds boot assets (iPXE binaries under tftp/, extracted
	// kernels, generated images). Empty means "<DataDir>/assets".
	AssetsDir string
	// LogLevel is one of debug, info, warn, error.
	LogLevel string
	// LogFormat is "text" or "json".
	LogFormat string
}

// FromEnv builds a Config from HELBOOT_* environment variables, falling
// back to development defaults.
func FromEnv() Config {
	return Config{
		DataDir:      envOr("HELBOOT_DATA_DIR", "./data"),
		HTTPAddr:     envOr("HELBOOT_HTTP_ADDR", ":8080"),
		ProvidersDir: envOr("HELBOOT_PROVIDERS_DIR", "./providers"),
		AssetsDir:    envOr("HELBOOT_ASSETS_DIR", ""),
		LogLevel:     envOr("HELBOOT_LOG_LEVEL", "info"),
		LogFormat:    envOr("HELBOOT_LOG_FORMAT", "text"),
	}
}

// AssetsPath returns the boot assets directory, defaulting to
// "<DataDir>/assets" when no explicit override is configured.
func (c Config) AssetsPath() string {
	if c.AssetsDir != "" {
		return c.AssetsDir
	}
	return filepath.Join(c.DataDir, "assets")
}

// DatabasePath returns the SQLite database file location inside DataDir.
func (c Config) DatabasePath() string {
	return filepath.Join(c.DataDir, "helboot.db")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
