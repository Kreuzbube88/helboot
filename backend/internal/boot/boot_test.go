package boot

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kdomanski/iso9660"

	"github.com/kreuzbube88/helboot/backend/internal/db"
	"github.com/kreuzbube88/helboot/backend/internal/iso"
	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/provider"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

const testProviderManifest = `
name: debian
display_name: "Debian"
family: debian
capabilities: {iso: true, pxe: true, unattended_install: true}
answer_file:
  format: preseed
  template: templates/preseed.cfg.tmpl
detection:
  volume_id_patterns: ["Debian *"]
boot:
  pxe:
    kernel: install.amd/vmlinuz
    initrd: [install.amd/initrd.gz]
    cmdline: "auto=true url={{ .AnswerFileURL }}"
`

const testAnswerTemplate = "hostname={{ .Hostname }}\nuser={{ .Username }}\nreport={{ .ReportURL }}\n"

func testHandler(t *testing.T) (*Handler, *store.Store, *iso.Manager) {
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

	providersDir := t.TempDir()
	pdir := filepath.Join(providersDir, "debian")
	if err := os.MkdirAll(filepath.Join(pdir, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "provider.yaml"), []byte(testProviderManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "templates", "preseed.cfg.tmpl"), []byte(testAnswerTemplate), 0o644); err != nil {
		t.Fatal(err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry, err := provider.LoadDir(providersDir, log)
	if err != nil {
		t.Fatal(err)
	}
	isos := iso.NewManager(log, t.TempDir(), st, registry)
	return New(log, st, registry, isos, t.TempDir(), providersDir), st, isos
}

func testServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func get(t *testing.T, ts *httptest.Server, path string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(body)
}

// makeDebianISO builds a tiny bootable-looking Debian ISO.
func makeDebianISO(t *testing.T) []byte {
	t.Helper()
	w, err := iso9660.NewWriter()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Cleanup()
	for path, content := range map[string]string{
		"install.amd/vmlinuz":   "fake-kernel-bytes",
		"install.amd/initrd.gz": "fake-initrd-bytes",
	} {
		if err := w.AddFile(strings.NewReader(content), path); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := w.WriteTo(&buf, "Debian 13 test"); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// queueInstallation wires host + ISO + profile + installation.
func queueInstallation(t *testing.T, st *store.Store, isos *iso.Manager) (*model.Host, *model.Installation) {
	t.Helper()
	img, err := isos.Import("debian.iso", bytes.NewReader(makeDebianISO(t)))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := st.CreateProfile("Debian Test", "debian", &img.ID, `{"Username": "pi"}`)
	if err != nil {
		t.Fatal(err)
	}
	host, err := st.CreateHost(model.Host{
		MAC: "aa:bb:cc:dd:ee:20", Hostname: "node20",
		ProfileID: &profile.ID, Status: model.HostReady,
	})
	if err != nil {
		t.Fatal(err)
	}
	version, err := st.ProfileVersionNumber(profile.ID, profile.CurrentVersion)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := st.CreateInstallation(host.ID, version.ID, "test-token-123")
	if err != nil {
		t.Fatal(err)
	}
	return host, inst
}

func TestScriptRecordsDiscoveredHost(t *testing.T) {
	h, st, _ := testHandler(t)
	ts := testServer(t, h)
	resp, body := get(t, ts, "/boot/ipxe?mac=AA:BB:CC:DD:EE:10&arch=x86_64&firmware=efi")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.HasPrefix(body, "#!ipxe") || !strings.Contains(body, "discovered") {
		t.Errorf("unexpected script: %q", body)
	}

	host, err := st.HostByMAC("aa:bb:cc:dd:ee:10")
	if err != nil {
		t.Fatalf("discovered host not stored: %v", err)
	}
	if host.Status != model.HostDiscovered || host.Firmware != "uefi" || host.Arch != "x86_64" {
		t.Errorf("discovery data wrong: %+v", host)
	}
}

func TestScriptForKnownHostBootsLocal(t *testing.T) {
	h, st, _ := testHandler(t)
	if _, err := st.CreateHost(model.Host{MAC: "aa:bb:cc:dd:ee:11", Status: model.HostReady}); err != nil {
		t.Fatal(err)
	}
	_, body := get(t, testServer(t, h), "/boot/ipxe?mac=aa:bb:cc:dd:ee:11")
	if !strings.Contains(body, "booting local disk") {
		t.Errorf("known idle host should boot locally, got: %q", body)
	}
}

func TestFullInstallationBootChain(t *testing.T) {
	h, st, isos := testHandler(t)
	ts := testServer(t, h)
	host, inst := queueInstallation(t, st, isos)

	// 1. The boot script boots the installer and pins the answer URL.
	_, script := get(t, ts, "/boot/ipxe?mac="+host.MAC)
	for _, want := range []string{
		"kernel " + ts.URL + "/boot/iso/1/install.amd/vmlinuz",
		"auto=true url=" + ts.URL + "/boot/answer/test-token-123",
		"initrd " + ts.URL + "/boot/iso/1/install.amd/initrd.gz",
		"boot",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q:\n%s", want, script)
		}
	}

	// 2. Serving the script transitions waiting → installing (§16).
	updated, err := st.InstallationByID(inst.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != model.InstallRunning || updated.StartedAt == nil {
		t.Errorf("installation not marked running: %+v", updated)
	}
	hostNow, _ := st.HostByMAC(host.MAC)
	if hostNow.Status != model.HostInstalling {
		t.Errorf("host status = %q, want installing", hostNow.Status)
	}

	// 3. Kernel and initrd stream straight out of the unmodified ISO.
	resp, kernel := get(t, ts, "/boot/iso/1/install.amd/vmlinuz")
	if resp.StatusCode != http.StatusOK || kernel != "fake-kernel-bytes" {
		t.Errorf("kernel serving failed: %d %q", resp.StatusCode, kernel)
	}

	// 4. The answer file renders profile config + host + boot params.
	_, answerBody := get(t, ts, "/boot/answer/test-token-123")
	for _, want := range []string{"hostname=node20", "user=pi", "report=" + ts.URL + "/boot/report/test-token-123"} {
		if !strings.Contains(answerBody, want) {
			t.Errorf("answer file missing %q:\n%s", want, answerBody)
		}
	}

	// 5. The installer reports success; queue and host reflect it.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/boot/report/test-token-123?status=success", nil)
	reportResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	reportResp.Body.Close()
	if reportResp.StatusCode != http.StatusNoContent {
		t.Fatalf("report status = %d", reportResp.StatusCode)
	}
	final, _ := st.InstallationByID(inst.ID)
	if final.Status != model.InstallSuccess || final.FinishedAt == nil {
		t.Errorf("installation not finished: %+v", final)
	}
	hostFinal, _ := st.HostByMAC(host.MAC)
	if hostFinal.Status != model.HostReady {
		t.Errorf("host status after success = %q, want ready", hostFinal.Status)
	}
}

func TestCloudInitSeed(t *testing.T) {
	h, st, isos := testHandler(t)
	ts := testServer(t, h)
	queueInstallation(t, st, isos)

	_, userData := get(t, ts, "/boot/cloudinit/test-token-123/user-data")
	if !strings.Contains(userData, "user=pi") {
		t.Errorf("user-data is not the rendered answer file: %q", userData)
	}
	_, metaData := get(t, ts, "/boot/cloudinit/test-token-123/meta-data")
	if !strings.Contains(metaData, "local-hostname: node20") {
		t.Errorf("meta-data wrong: %q", metaData)
	}
}

func TestBootTokenIsRequired(t *testing.T) {
	h, st, isos := testHandler(t)
	ts := testServer(t, h)
	queueInstallation(t, st, isos)

	resp, _ := get(t, ts, "/boot/answer/wrong-token")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("wrong token: status = %d, want 404", resp.StatusCode)
	}
}

func TestScriptWithoutMACAsksIPXEToRetry(t *testing.T) {
	h, _, _ := testHandler(t)
	_, body := get(t, testServer(t, h), "/boot/ipxe")
	if !strings.Contains(body, "${net0/mac}") {
		t.Errorf("expected chain with mac variable, got: %q", body)
	}
}

func TestAssetsAreServed(t *testing.T) {
	h, _, _ := testHandler(t)
	if err := os.WriteFile(filepath.Join(h.assetsDir, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, body := get(t, testServer(t, h), "/boot/assets/hello.txt")
	if resp.StatusCode != http.StatusOK || body != "hi" {
		t.Errorf("asset not served: status=%d body=%q", resp.StatusCode, body)
	}
}

func TestISOFileServedWithRanges(t *testing.T) {
	h, st, isos := testHandler(t)
	ts := testServer(t, h)
	queueInstallation(t, st, isos)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/boot/isofile/1", nil)
	req.Header.Set("Range", "bytes=0-9")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Errorf("range request: status = %d, want 206", resp.StatusCode)
	}
}

const testWimbootManifest = `
name: windows11
display_name: "Windows 11"
family: windows
capabilities: {iso: true, pxe: true}
answer_file:
  format: autounattend.xml
  template: templates/autounattend.xml.tmpl
detection:
  volume_id_patterns: ["CCCOMA_X64FRE*"]
boot:
  pxe:
    kernel: wimboot
    initrd:
      - "boot/bcd=BCD"
      - "sources/boot.wim"
`

// TestInstallScriptWimbootInitrd covers the wimboot boot chain: a bare
// kernel name resolves against /boot/assets/, and an initrd entry with a
// "source=localname" suffix is rendered as a two-argument iPXE "initrd"
// line so wimboot sees the file under the exact name it requires (e.g.
// "BCD", regardless of the path/case inside the ISO).
func TestInstallScriptWimbootInitrd(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.Migrate(sqlDB); err != nil {
		t.Fatal(err)
	}
	st := store.New(sqlDB)

	providersDir := t.TempDir()
	pdir := filepath.Join(providersDir, "windows11")
	if err := os.MkdirAll(filepath.Join(pdir, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "provider.yaml"), []byte(testWimbootManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "templates", "autounattend.xml.tmpl"), []byte("<unattend/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry, err := provider.LoadDir(providersDir, log)
	if err != nil {
		t.Fatal(err)
	}
	isos := iso.NewManager(log, t.TempDir(), st, registry)
	h := New(log, st, registry, isos, t.TempDir(), providersDir)
	ts := testServer(t, h)

	w, err := iso9660.NewWriter()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Cleanup()
	for path, content := range map[string]string{
		"boot/bcd":         "fake-bcd-bytes",
		"sources/boot.wim": "fake-wim-bytes",
	} {
		if err := w.AddFile(strings.NewReader(content), path); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := w.WriteTo(&buf, "CCCOMA_X64FRE_EN-US_DV9"); err != nil {
		t.Fatal(err)
	}

	img, err := isos.Import("win11.iso", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if img.Provider != "windows11" {
		t.Fatalf("iso provider = %q, want windows11", img.Provider)
	}
	profile, err := st.CreateProfile("Win11 Test", "windows11", &img.ID, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	host, err := st.CreateHost(model.Host{
		MAC: "aa:bb:cc:dd:ee:30", Hostname: "node30",
		ProfileID: &profile.ID, Status: model.HostReady,
	})
	if err != nil {
		t.Fatal(err)
	}
	version, err := st.ProfileVersionNumber(profile.ID, profile.CurrentVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateInstallation(host.ID, version.ID, "win11-token"); err != nil {
		t.Fatal(err)
	}

	_, script := get(t, ts, "/boot/ipxe?mac="+host.MAC)
	for _, want := range []string{
		"kernel " + ts.URL + "/boot/assets/wimboot",
		"initrd " + ts.URL + "/boot/iso/1/boot/bcd BCD",
		"initrd " + ts.URL + "/boot/iso/1/sources/boot.wim\n",
		"boot",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q:\n%s", want, script)
		}
	}
}
