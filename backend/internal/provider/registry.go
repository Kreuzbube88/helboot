package provider

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Registry holds all successfully loaded provider manifests.
type Registry struct {
	providers map[string]*Manifest
}

// LoadDir walks dir for <name>/provider.yaml manifests, validates each,
// and returns the registry. A broken manifest disables only that
// provider (logged as an error), never the whole application (ADR-0005).
// A missing directory yields an empty registry: the server must come up
// even before any providers are installed.
func LoadDir(dir string, log *slog.Logger) (*Registry, error) {
	reg := &Registry{providers: map[string]*Manifest{}}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		log.Warn("providers directory does not exist; no providers loaded", "dir", dir)
		return reg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read providers directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "provider.yaml")
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // directory without manifest, e.g. shared assets
			}
			log.Error("skipping invalid provider manifest", "path", manifestPath, "error", err)
			continue
		}
		if manifest.Name != entry.Name() {
			log.Error("skipping provider: manifest name must match directory name",
				"dir", entry.Name(), "name", manifest.Name)
			continue
		}
		reg.providers[manifest.Name] = manifest
		log.Debug("loaded provider", "name", manifest.Name)
	}
	log.Info("providers loaded", "count", len(reg.providers))
	return reg, nil
}

func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// All returns every loaded manifest, sorted by name for stable output.
func (r *Registry) All() []*Manifest {
	out := make([]*Manifest, 0, len(r.providers))
	for _, m := range r.providers {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns the manifest with the given name, or nil.
func (r *Registry) Get(name string) *Manifest {
	return r.providers[name]
}
