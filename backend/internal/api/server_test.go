package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kreuzbube88/helboot/backend/internal/boot"
	"github.com/kreuzbube88/helboot/backend/internal/db"
	"github.com/kreuzbube88/helboot/backend/internal/iso"
	"github.com/kreuzbube88/helboot/backend/internal/provider"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// testServer spins up a fully wired API server on a temp database with
// one debian test provider loaded.
func testServer(t *testing.T) *httptest.Server {
	t.Helper()

	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.Migrate(sqlDB); err != nil {
		t.Fatal(err)
	}

	providersDir := t.TempDir()
	pdir := filepath.Join(providersDir, "debian")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `
name: debian
display_name: "Debian"
family: debian
capabilities: {iso: true, pxe: true, unattended_install: true}
answer_file: {format: preseed, template: templates/preseed.cfg.tmpl}
`
	if err := os.WriteFile(filepath.Join(pdir, "provider.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pdir, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	tmpl := "lang {{ .language }}\nhost {{ .Hostname }}\nreport {{ .ReportURL }}\n"
	if err := os.WriteFile(filepath.Join(pdir, "templates", "preseed.cfg.tmpl"), []byte(tmpl), 0o644); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry, err := provider.LoadDir(providersDir, log)
	if err != nil {
		t.Fatal(err)
	}

	st := store.New(sqlDB)
	isoManager := iso.NewManager(log, t.TempDir(), st, registry)
	assetsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(assetsDir, "tftp"), 0o755); err != nil {
		t.Fatal(err)
	}
	server := New(Deps{
		Log:         log,
		Store:       st,
		Registry:    registry,
		Version:     "test",
		OpenAPISpec: []byte("openapi: 3.1.0"),
		Boot:        boot.New(log, st, registry, isoManager, assetsDir, providersDir),
		ISOs:        isoManager,
		AssetsDir:   assetsDir,
	})
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	return ts
}

// client wraps http.Client with session cookie and CSRF handling.
type client struct {
	t    *testing.T
	base string
	http *http.Client
	csrf string
}

func newClient(t *testing.T, ts *httptest.Server) *client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &client{t: t, base: ts.URL, http: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any) (*http.Response, []byte) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		c.t.Fatal(err)
	}
	return resp, data
}

// setupAndLogin completes the wizard and logs in as the admin.
func (c *client) setupAndLogin() {
	c.t.Helper()
	resp, body := c.do(http.MethodPost, "/api/v1/setup", map[string]any{
		"language": "en",
		"admin":    map[string]string{"username": "admin", "password": "super-secret-pw"},
		"network":  map[string]string{"mode": "proxy_dhcp"},
	})
	if resp.StatusCode != http.StatusCreated {
		c.t.Fatalf("setup: status %d: %s", resp.StatusCode, body)
	}
	resp, body = c.do(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "super-secret-pw",
	})
	if resp.StatusCode != http.StatusOK {
		c.t.Fatalf("login: status %d: %s", resp.StatusCode, body)
	}
	var sess struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(body, &sess); err != nil {
		c.t.Fatal(err)
	}
	c.csrf = sess.CSRFToken
}

func TestHealthIsPublic(t *testing.T) {
	ts := testServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status = %d, want 200", resp.StatusCode)
	}
}

func TestProtectedEndpointsRequireAuth(t *testing.T) {
	ts := testServer(t)
	for _, path := range []string{"/api/v1/hosts", "/api/v1/profiles", "/api/v1/providers", "/api/v1/system/info"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s = %d, want 401", path, resp.StatusCode)
		}
	}
}

func TestSetupFlow(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)

	// Weak password must be rejected.
	resp, _ := c.do(http.MethodPost, "/api/v1/setup", map[string]any{
		"language": "en",
		"admin":    map[string]string{"username": "admin", "password": "short"},
		"network":  map[string]string{"mode": "proxy_dhcp"},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("weak password: status %d, want 400", resp.StatusCode)
	}

	c.setupAndLogin()

	// Second setup attempt must conflict.
	resp, _ = c.do(http.MethodPost, "/api/v1/setup", map[string]any{
		"language": "en",
		"admin":    map[string]string{"username": "admin2", "password": "another-secret-pw"},
		"network":  map[string]string{"mode": "dhcp"},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("second setup: status %d, want 409", resp.StatusCode)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin()

	resp, _ := c.do(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "wrong-password",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("bad credentials: status %d, want 401", resp.StatusCode)
	}
	resp, _ = c.do(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "ghost", "password": "wrong-password",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unknown user: status %d, want 401", resp.StatusCode)
	}
}

func TestMutationsRequireCSRFToken(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin()

	goodCSRF := c.csrf
	c.csrf = "" // simulate a cross-site request lacking the header
	resp, _ := c.do(http.MethodPost, "/api/v1/hosts", map[string]any{"mac": "aa:bb:cc:dd:ee:01"})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("missing CSRF token: status %d, want 403", resp.StatusCode)
	}
	c.csrf = goodCSRF
	resp, _ = c.do(http.MethodPost, "/api/v1/hosts", map[string]any{"mac": "aa:bb:cc:dd:ee:01"})
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("with CSRF token: status %d, want 201", resp.StatusCode)
	}
}

func TestHostCRUD(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin()

	// Invalid MAC is rejected.
	resp, _ := c.do(http.MethodPost, "/api/v1/hosts", map[string]any{"mac": "not-a-mac"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid MAC: status %d, want 400", resp.StatusCode)
	}

	// Create normalizes the MAC.
	resp, body := c.do(http.MethodPost, "/api/v1/hosts", map[string]any{
		"mac": "AA-BB-CC-DD-EE-02", "hostname": "node1", "tags": []string{"lab"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create host: status %d: %s", resp.StatusCode, body)
	}
	var host struct {
		ID  int64  `json:"id"`
		MAC string `json:"mac"`
	}
	json.Unmarshal(body, &host)
	if host.MAC != "aa:bb:cc:dd:ee:02" {
		t.Errorf("MAC not normalized: %q", host.MAC)
	}

	// Duplicate MAC conflicts.
	resp, _ = c.do(http.MethodPost, "/api/v1/hosts", map[string]any{"mac": "aa:bb:cc:dd:ee:02"})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate MAC: status %d, want 409", resp.StatusCode)
	}

	// Update, then delete.
	resp, body = c.do(http.MethodPut, "/api/v1/hosts/1", map[string]any{
		"mac": "aa:bb:cc:dd:ee:02", "hostname": "renamed",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update host: status %d: %s", resp.StatusCode, body)
	}
	resp, _ = c.do(http.MethodDelete, "/api/v1/hosts/1", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete host: status %d, want 204", resp.StatusCode)
	}
	resp, _ = c.do(http.MethodGet, "/api/v1/hosts/1", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("deleted host: status %d, want 404", resp.StatusCode)
	}
}

func TestProfileVersioning(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin()

	// Unknown provider is rejected.
	resp, _ := c.do(http.MethodPost, "/api/v1/profiles", map[string]any{
		"name": "bad", "provider": "atari-tos",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown provider: status %d, want 400", resp.StatusCode)
	}

	resp, body := c.do(http.MethodPost, "/api/v1/profiles", map[string]any{
		"name": "Debian Base", "provider": "debian",
		"config": map[string]any{"language": "de"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create profile: status %d: %s", resp.StatusCode, body)
	}

	// Updating with a config edits version 1 in place (ADR-0013).
	resp, body = c.do(http.MethodPut, "/api/v1/profiles/1", map[string]any{
		"name": "Debian Base", "config": map[string]any{"language": "en"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update profile: status %d: %s", resp.StatusCode, body)
	}
	var p struct {
		CurrentVersion int `json:"currentVersion"`
	}
	json.Unmarshal(body, &p)
	if p.CurrentVersion != 1 {
		t.Errorf("in-place edit: currentVersion = %d, want 1", p.CurrentVersion)
	}

	// Saving as a new version is the explicit trigger for version 2.
	resp, body = c.do(http.MethodPut, "/api/v1/profiles/1", map[string]any{
		"name": "Debian Base", "config": map[string]any{"language": "fr"},
		"saveAsNewVersion": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save new version: status %d: %s", resp.StatusCode, body)
	}
	json.Unmarshal(body, &p)
	if p.CurrentVersion != 2 {
		t.Errorf("new version: currentVersion = %d, want 2", p.CurrentVersion)
	}

	resp, body = c.do(http.MethodGet, "/api/v1/profiles/1/versions", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("versions: status %d", resp.StatusCode)
	}
	var versions []struct {
		Version int    `json:"version"`
		Config  string `json:"config"`
	}
	json.Unmarshal(body, &versions)
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2", len(versions))
	}
	if !strings.Contains(versions[0].Config, `"en"`) {
		t.Errorf("version 1 config = %s, want the in-place edit", versions[0].Config)
	}

	// Once an installation references the head version, in-place edits
	// are refused — history must stay immutable.
	resp, body = c.do(http.MethodPost, "/api/v1/hosts", map[string]any{
		"mac": "aa:bb:cc:dd:ee:10", "profileId": 1,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create host: status %d: %s", resp.StatusCode, body)
	}
	var host struct {
		ID             int64 `json:"id"`
		ProfileVersion int   `json:"profileVersion"`
	}
	json.Unmarshal(body, &host)
	if host.ProfileVersion != 2 {
		t.Errorf("host pin = %d, want current version 2", host.ProfileVersion)
	}
	resp, body = c.do(http.MethodPost, "/api/v1/installations", map[string]any{"hostId": host.ID})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("queue installation: status %d: %s", resp.StatusCode, body)
	}
	resp, _ = c.do(http.MethodPut, "/api/v1/profiles/1", map[string]any{
		"name": "Debian Base", "config": map[string]any{"language": "it"},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("in-place edit of referenced version: status %d, want 409", resp.StatusCode)
	}
	resp, body = c.do(http.MethodPut, "/api/v1/profiles/1", map[string]any{
		"name": "Debian Base", "config": map[string]any{"language": "it"},
		"saveAsNewVersion": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save new version after conflict: status %d: %s", resp.StatusCode, body)
	}
	json.Unmarshal(body, &p)
	if p.CurrentVersion != 3 {
		t.Errorf("after conflict: currentVersion = %d, want 3", p.CurrentVersion)
	}
}

func TestAnswerPreviewAndOverride(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin()

	resp, body := c.do(http.MethodPost, "/api/v1/profiles", map[string]any{
		"name": "Debian Base", "provider": "debian",
		"config": map[string]any{"language": "de"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create profile: status %d: %s", resp.StatusCode, body)
	}

	var preview struct {
		Format     string `json:"format"`
		Content    string `json:"content"`
		Overridden bool   `json:"overridden"`
	}
	resp, body = c.do(http.MethodGet, "/api/v1/profiles/1/versions/1/answer", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview: status %d: %s", resp.StatusCode, body)
	}
	json.Unmarshal(body, &preview)
	if preview.Format != "preseed" || preview.Overridden {
		t.Errorf("preview = %+v, want preseed / not overridden", preview)
	}
	if !strings.Contains(preview.Content, "lang de") {
		t.Errorf("preview does not render config values: %s", preview.Content)
	}
	if !strings.Contains(preview.Content, "PREVIEW-TOKEN") {
		t.Errorf("preview must use placeholder boot params: %s", preview.Content)
	}

	// A syntactically broken override is rejected up front.
	resp, _ = c.do(http.MethodPut, "/api/v1/profiles/1/versions/1/answer-override",
		map[string]any{"content": "{{ .broken"})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("broken override: status %d, want 422", resp.StatusCode)
	}

	// A valid override replaces the template and stays a template itself.
	resp, _ = c.do(http.MethodPut, "/api/v1/profiles/1/versions/1/answer-override",
		map[string]any{"content": "custom for {{ .MAC }}"})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("set override: status %d", resp.StatusCode)
	}
	resp, body = c.do(http.MethodGet, "/api/v1/profiles/1/versions/1/answer", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview after override: status %d", resp.StatusCode)
	}
	json.Unmarshal(body, &preview)
	if !preview.Overridden || !strings.Contains(preview.Content, "custom for 52:54:00") {
		t.Errorf("override not served: %+v", preview)
	}

	// Empty content clears the override (= regenerate from template).
	resp, _ = c.do(http.MethodPut, "/api/v1/profiles/1/versions/1/answer-override",
		map[string]any{"content": ""})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("clear override: status %d", resp.StatusCode)
	}
	resp, body = c.do(http.MethodGet, "/api/v1/profiles/1/versions/1/answer", nil)
	json.Unmarshal(body, &preview)
	if preview.Overridden {
		t.Error("override should be cleared")
	}

	resp, _ = c.do(http.MethodGet, "/api/v1/profiles/1/versions/9/answer", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown version: status %d, want 404", resp.StatusCode)
	}
}

func TestProvidersEndpoint(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin()

	resp, body := c.do(http.MethodGet, "/api/v1/providers", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("providers: status %d", resp.StatusCode)
	}
	var providers []struct {
		Name         string          `json:"name"`
		Capabilities map[string]bool `json:"capabilities"`
	}
	json.Unmarshal(body, &providers)
	if len(providers) != 1 || providers[0].Name != "debian" {
		t.Fatalf("unexpected providers: %s", body)
	}
	if !providers[0].Capabilities["pxe"] {
		t.Error("pxe capability missing")
	}

	resp, _ = c.do(http.MethodGet, "/api/v1/providers/nope", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown provider: status %d, want 404", resp.StatusCode)
	}
}

func TestUnknownAPIPathReturnsJSON404(t *testing.T) {
	ts := testServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}
}
