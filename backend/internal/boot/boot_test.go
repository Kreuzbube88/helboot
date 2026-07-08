package boot

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kreuzbube88/helboot/backend/internal/db"
	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

func testHandler(t *testing.T) (*Handler, *store.Store, string) {
	t.Helper()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.Migrate(sqlDB); err != nil {
		t.Fatal(err)
	}
	st := store.New(sqlDB)
	assets := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(log, st, assets), st, assets
}

func serve(t *testing.T, h *Handler, path string) (*http.Response, string) {
	t.Helper()
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(body)
}

func TestScriptRecordsDiscoveredHost(t *testing.T) {
	h, st, _ := testHandler(t)
	resp, body := serve(t, h, "/boot/ipxe?mac=AA:BB:CC:DD:EE:10&arch=x86_64&firmware=efi")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.HasPrefix(body, "#!ipxe") {
		t.Errorf("not an iPXE script: %q", body)
	}
	if !strings.Contains(body, "discovered") {
		t.Errorf("unknown host should get the discovery script, got: %q", body)
	}

	host, err := st.HostByMAC("aa:bb:cc:dd:ee:10")
	if err != nil {
		t.Fatalf("discovered host not stored: %v", err)
	}
	if host.Status != model.HostDiscovered {
		t.Errorf("status = %q, want discovered", host.Status)
	}
	if host.Firmware != "uefi" || host.Arch != "x86_64" {
		t.Errorf("firmware/arch not recorded: %q/%q", host.Firmware, host.Arch)
	}
}

func TestScriptForKnownHostBootsLocal(t *testing.T) {
	h, st, _ := testHandler(t)
	if _, err := st.CreateHost(model.Host{MAC: "aa:bb:cc:dd:ee:11", Status: model.HostReady}); err != nil {
		t.Fatal(err)
	}
	_, body := serve(t, h, "/boot/ipxe?mac=aa:bb:cc:dd:ee:11")
	if !strings.Contains(body, "booting local disk") {
		t.Errorf("known idle host should boot locally, got: %q", body)
	}
}

func TestScriptWithoutMACAsksIPXEToRetry(t *testing.T) {
	h, _, _ := testHandler(t)
	_, body := serve(t, h, "/boot/ipxe")
	if !strings.Contains(body, "${net0/mac}") {
		t.Errorf("expected chain with mac variable, got: %q", body)
	}
}

func TestAssetsAreServed(t *testing.T) {
	h, _, assets := testHandler(t)
	if err := os.WriteFile(filepath.Join(assets, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, body := serve(t, h, "/boot/assets/hello.txt")
	if resp.StatusCode != http.StatusOK || body != "hi" {
		t.Errorf("asset not served: status=%d body=%q", resp.StatusCode, body)
	}
}
