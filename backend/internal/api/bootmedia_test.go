package api

import (
	"net/http"
	"testing"
)

func TestBootMediaDownload(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin()

	// Missing asset → clear 404 with hint code.
	resp, body := c.do(http.MethodGet, "/api/v1/bootmedia/iso", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing medium: %d %s", resp.StatusCode, body)
	}

	// Invalid format rejected.
	resp, _ = c.do(http.MethodGet, "/api/v1/bootmedia/floppy", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid format: %d, want 400", resp.StatusCode)
	}
}
