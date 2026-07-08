// Package boot serves the HTTP endpoints consumed by iPXE and firmware
// under /boot/ (ADR-0010). These are unauthenticated by protocol
// necessity, minimal, read-only and MAC-scoped.
package boot

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/kreuzbube88/helboot/backend/internal/iso"
	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/netutil"
	"github.com/kreuzbube88/helboot/backend/internal/provider"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// manifestType keeps install.go free of a direct provider import cycle
// concern and documents the single point of coupling.
type manifestType = *provider.Manifest

// Handler implements the /boot/ HTTP surface.
type Handler struct {
	log          *slog.Logger
	store        *store.Store
	registry     *provider.Registry
	isos         *iso.Manager
	assetsDir    string
	providersDir string
}

// New creates the boot handler. assetsDir is the boot assets root; its
// files are exposed read-only under /boot/assets/. providersDir is where
// answer-file templates live.
func New(log *slog.Logger, st *store.Store, reg *provider.Registry, isos *iso.Manager, assetsDir, providersDir string) *Handler {
	return &Handler{
		log: log, store: st, registry: reg, isos: isos,
		assetsDir: assetsDir, providersDir: providersDir,
	}
}

// Register mounts the boot routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /boot/ipxe", h.handleScript)
	mux.Handle("GET /boot/assets/",
		http.StripPrefix("/boot/assets/", http.FileServer(http.Dir(h.assetsDir))))
	mux.HandleFunc("GET /boot/isofile/{id}", h.handleISOFile)
	mux.HandleFunc("GET /boot/iso/{id}/{path...}", h.handleISOContent)
	mux.HandleFunc("GET /boot/answer/{token}", h.handleAnswerFile)
	mux.HandleFunc("GET /boot/cloudinit/{token}/{file}", h.handleCloudInit)
	mux.HandleFunc("POST /boot/report/{token}", h.handleReport)
}

// handleScript returns the per-machine iPXE script. iPXE requests it as
// http://server/boot/ipxe?mac=${net0/mac}&arch=${buildarch}&firmware=${platform}
// Unknown machines are recorded as "discovered" so they show up in the
// UI (§15); machines without pending work boot from local disk.
func (h *Handler) handleScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	mac, err := netutil.NormalizeMAC(r.URL.Query().Get("mac"))
	if err != nil {
		// Without a MAC we cannot identify the machine; tell iPXE to
		// retry with the parameter filled in.
		fmt.Fprint(w, "#!ipxe\nchain /boot/ipxe?mac=${net0/mac}&arch=${buildarch}&firmware=${platform}\n")
		return
	}

	host, err := h.store.HostByMAC(mac)
	if errors.Is(err, store.ErrNotFound) {
		host = h.discover(mac, r)
	} else if err != nil {
		h.log.Error("boot: host lookup failed", "mac", mac, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, h.scriptFor(baseURL(r), host))
}

// discover records a previously unknown machine (§15). Failures are
// logged but still produce a usable boot script.
func (h *Handler) discover(mac string, r *http.Request) *model.Host {
	host := model.Host{
		MAC:      mac,
		Arch:     sanitizeParam(r.URL.Query().Get("arch")),
		Firmware: firmwareParam(r.URL.Query().Get("firmware")),
		Status:   model.HostDiscovered,
	}
	created, err := h.store.CreateHost(host)
	if err != nil {
		h.log.Error("boot: recording discovered host failed", "mac", mac, "error", err)
		return &host
	}
	h.log.Info("boot: new host discovered", "mac", mac, "arch", host.Arch, "firmware", host.Firmware)
	return created
}

// scriptFor renders the iPXE script for a host: queued installations
// boot into the installer, discovered hosts get a hint, everything else
// boots from local disk.
func (h *Handler) scriptFor(baseURL string, host *model.Host) string {
	ic, reason, err := h.loadInstallContext(host)
	if err != nil {
		h.log.Error("boot: install lookup failed", "mac", host.MAC, "error", err)
		return messageScript("internal error, check the HELBOOT logs")
	}
	if ic != nil {
		script := h.installScript(baseURL, ic)
		if strings.Contains(script, "\nboot\n") {
			h.markInstalling(ic)
		}
		return script
	}
	if reason != "" {
		h.log.Warn("boot: installation not bootable", "mac", host.MAC, "reason", reason)
		return messageScript(fmt.Sprintf("installation for %s is not bootable: %s", host.MAC, reason))
	}

	var b strings.Builder
	b.WriteString("#!ipxe\n")
	switch host.Status {
	case model.HostDiscovered:
		fmt.Fprintf(&b, "echo HELBOOT: host %s discovered - assign a profile in the web UI\n", host.MAC)
		b.WriteString("sleep 5\n")
		b.WriteString("exit\n")
	default:
		fmt.Fprintf(&b, "echo HELBOOT: host %s has no pending installation, booting local disk\n", host.MAC)
		b.WriteString("exit\n")
	}
	return b.String()
}

// sanitizeParam keeps discovery inputs boring: short, printable, no
// control characters (these values come straight off the network).
func sanitizeParam(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e {
			return -1
		}
		return r
	}, s)
	if len(s) > 32 {
		s = s[:32]
	}
	return s
}

// firmwareParam maps iPXE's ${platform} to the host firmware enum.
func firmwareParam(s string) string {
	switch strings.ToLower(s) {
	case "efi":
		return "uefi"
	case "pcbios":
		return "bios"
	default:
		return ""
	}
}
