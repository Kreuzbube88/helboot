package config

import (
	"path/filepath"
	"testing"
)

func TestFromEnvDefaults(t *testing.T) {
	cfg := FromEnv()
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.DataDir != "./data" {
		t.Errorf("DataDir = %q, want ./data", cfg.DataDir)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func TestFromEnvOverride(t *testing.T) {
	t.Setenv("HELBOOT_HTTP_ADDR", ":9090")
	t.Setenv("HELBOOT_DATA_DIR", "/data")
	cfg := FromEnv()
	if cfg.HTTPAddr != ":9090" {
		t.Errorf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if got, want := cfg.DatabasePath(), filepath.Join("/data", "helboot.db"); got != want {
		t.Errorf("DatabasePath() = %q, want %q", got, want)
	}
}
