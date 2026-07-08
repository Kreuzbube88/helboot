package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestProfileCloneExportImport(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin()

	// Source profile with config.
	resp, body := c.do(http.MethodPost, "/api/v1/profiles", map[string]any{
		"name": "Debian Base", "provider": "debian",
		"config": map[string]any{"Username": "pi", "Packages": []string{"curl"}},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}

	// Clone keeps provider and config, starts at version 1.
	resp, body = c.do(http.MethodPost, "/api/v1/profiles/1/clone", map[string]string{"name": "Debian Clone"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("clone: %d %s", resp.StatusCode, body)
	}
	var clone struct {
		ID             int64  `json:"id"`
		Provider       string `json:"provider"`
		CurrentVersion int    `json:"currentVersion"`
	}
	json.Unmarshal(body, &clone)
	if clone.Provider != "debian" || clone.CurrentVersion != 1 {
		t.Errorf("clone metadata wrong: %+v", clone)
	}
	resp, body = c.do(http.MethodGet, "/api/v1/profiles/2/versions", nil)
	var versions []struct {
		Config string `json:"config"`
	}
	json.Unmarshal(body, &versions)
	if len(versions) != 1 || versions[0].Config == "" || versions[0].Config == "{}" {
		t.Errorf("clone did not copy config: %+v", versions)
	}

	// Duplicate clone name conflicts.
	resp, _ = c.do(http.MethodPost, "/api/v1/profiles/1/clone", map[string]string{"name": "Debian Clone"})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate clone name: %d, want 409", resp.StatusCode)
	}

	// Export produces a portable document.
	resp, body = c.do(http.MethodGet, "/api/v1/profiles/1/export", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export: %d", resp.StatusCode)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("export is not JSON: %v", err)
	}
	if doc["type"] != "helboot-profile" || doc["provider"] != "debian" {
		t.Errorf("export document wrong: %v", doc)
	}

	// Import the exported document under a new name.
	doc["name"] = "Debian Imported"
	resp, body = c.do(http.MethodPost, "/api/v1/profiles/import", doc)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("import: %d %s", resp.StatusCode, body)
	}

	// Import with an unknown provider is rejected.
	doc["name"] = "Broken"
	doc["provider"] = "beos"
	resp, _ = c.do(http.MethodPost, "/api/v1/profiles/import", doc)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown provider import: %d, want 400", resp.StatusCode)
	}

	// Garbage document rejected.
	resp, _ = c.do(http.MethodPost, "/api/v1/profiles/import", map[string]any{"type": "other", "version": 1})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("garbage import: %d, want 400", resp.StatusCode)
	}
}
