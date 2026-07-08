package boot

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/answer"
	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// installContext bundles everything needed to boot one installation.
type installContext struct {
	host     *model.Host
	install  *model.Installation
	version  *model.ProfileVersion
	profile  *model.Profile
	iso      *model.ISOImage
	manifest providerManifest
}

// providerManifest is the subset of provider.Manifest the boot chain
// needs (aliased to avoid a wide import surface in this file).
type providerManifest = manifestType

// loadInstallContext resolves the full chain host → installation →
// profile version → profile → provider → ISO. A nil return with reason
// set means "explainable, not bootable"; err means storage failure.
func (h *Handler) loadInstallContext(host *model.Host) (*installContext, string, error) {
	inst, err := h.store.ActiveInstallationForHost(host.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, "", nil // nothing queued — not an error
	}
	if err != nil {
		return nil, "", err
	}
	version, err := h.store.ProfileVersionByID(inst.ProfileVersionID)
	if err != nil {
		return nil, "profile version missing", nil
	}
	profile, err := h.store.ProfileByID(version.ProfileID)
	if err != nil {
		return nil, "profile missing", nil
	}
	manifest := h.registry.Get(profile.Provider)
	if manifest == nil {
		return nil, fmt.Sprintf("provider %s is not installed", profile.Provider), nil
	}
	if profile.ISOID == nil {
		return nil, "profile has no ISO assigned", nil
	}
	isoImg, err := h.store.ISOByID(*profile.ISOID)
	if err != nil {
		return nil, "assigned ISO is missing", nil
	}
	return &installContext{
		host: host, install: inst, version: version,
		profile: profile, iso: isoImg, manifest: manifest,
	}, "", nil
}

// installScript renders the iPXE script that boots an installation.
func (h *Handler) installScript(baseURL string, ic *installContext) string {
	bootCfg, ok := ic.manifest.Boot["pxe"]
	if !ok {
		bootCfg, ok = ic.manifest.Boot["http_boot"]
	}
	if !ok {
		return messageScript(fmt.Sprintf("provider %s does not support network boot", ic.manifest.Name))
	}

	params := h.answerParams(baseURL, ic)
	cmdline, err := answer.Render(bootCfg.Cmdline, ic.version.Config, params)
	if err != nil {
		h.log.Error("boot: cmdline template failed", "provider", ic.manifest.Name, "error", err)
		return messageScript("boot configuration error, check the HELBOOT logs")
	}

	// Paths containing "/" live inside the ISO; bare names (wimboot,
	// helper binaries) come from the boot assets directory.
	resolve := func(p string) string {
		if strings.Contains(p, "/") {
			return fmt.Sprintf("%s/boot/iso/%d/%s", baseURL, ic.iso.ID, strings.TrimPrefix(p, "/"))
		}
		return fmt.Sprintf("%s/boot/assets/%s", baseURL, p)
	}

	var b strings.Builder
	b.WriteString("#!ipxe\n")
	fmt.Fprintf(&b, "echo HELBOOT: installing %s on %s\n", ic.profile.Name, ic.host.MAC)
	fmt.Fprintf(&b, "kernel %s %s\n", resolve(bootCfg.Kernel), strings.TrimSpace(string(cmdline)))
	for _, initrd := range bootCfg.Initrd {
		fmt.Fprintf(&b, "initrd %s\n", resolve(initrd))
	}
	b.WriteString("boot\n")
	return b.String()
}

// answerParams builds the template parameters for one installation.
func (h *Handler) answerParams(baseURL string, ic *installContext) answer.Params {
	token := ic.install.Token
	return answer.Params{
		AnswerFileURL: fmt.Sprintf("%s/boot/answer/%s", baseURL, token),
		ISOURL:        fmt.Sprintf("%s/boot/isofile/%d", baseURL, ic.iso.ID),
		ISOContentURL: fmt.Sprintf("%s/boot/iso/%d", baseURL, ic.iso.ID),
		CloudInitURL:  fmt.Sprintf("%s/boot/cloudinit/%s/", baseURL, token),
		ReportURL:     fmt.Sprintf("%s/boot/report/%s", baseURL, token),
		Hostname:      ic.host.Hostname,
		MAC:           ic.host.MAC,
	}
}

// markInstalling transitions the queue entry and host when the boot
// script is actually served (§16: waiting → installing).
func (h *Handler) markInstalling(ic *installContext) {
	if ic.install.Status != model.InstallWaiting {
		return
	}
	now := time.Now()
	if err := h.store.MarkInstallationStarted(ic.install.ID, now); err != nil {
		h.log.Error("boot: cannot mark installation started", "id", ic.install.ID, "error", err)
		return
	}
	ic.host.Status = model.HostInstalling
	if _, err := h.store.UpdateHost(*ic.host); err != nil {
		h.log.Error("boot: cannot update host status", "mac", ic.host.MAC, "error", err)
	}
	h.log.Info("installation started", "mac", ic.host.MAC, "profile", ic.profile.Name)
}

// handleAnswerFile serves the rendered answer file for an installation.
func (h *Handler) handleAnswerFile(w http.ResponseWriter, r *http.Request) {
	ic, ok := h.contextByToken(w, r)
	if !ok {
		return
	}
	if ic.manifest.AnswerFile.Template == "" {
		http.Error(w, "provider has no answer file", http.StatusNotFound)
		return
	}
	templatePath := filepath.Join(h.providersDir, ic.manifest.Name,
		filepath.Clean("/"+ic.manifest.AnswerFile.Template))
	out, err := answer.RenderFile(templatePath, ic.version.Config, h.answerParams(baseURL(r), ic))
	if err != nil {
		h.log.Error("boot: answer file rendering failed", "provider", ic.manifest.Name, "error", err)
		http.Error(w, "answer file rendering failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(out)
}

// handleCloudInit serves the NoCloud seed: user-data is the rendered
// answer file, meta-data identifies the instance.
func (h *Handler) handleCloudInit(w http.ResponseWriter, r *http.Request) {
	ic, ok := h.contextByToken(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	switch r.PathValue("file") {
	case "user-data":
		h.handleAnswerFile(w, r)
	case "meta-data":
		fmt.Fprintf(w, "instance-id: helboot-%d\nlocal-hostname: %s\n", ic.install.ID, ic.host.Hostname)
	case "vendor-data":
		// Present but empty: cloud-init requests it and treats 404s as
		// retryable on some versions.
	default:
		http.NotFound(w, r)
	}
}

// handleReport lets installer late-commands report the final status:
// POST /boot/report/<token>?status=success|error
func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	ic, ok := h.contextByToken(w, r)
	if !ok {
		return
	}
	var status model.InstallationStatus
	switch r.URL.Query().Get("status") {
	case "success":
		status = model.InstallSuccess
	case "error":
		status = model.InstallError
	default:
		http.Error(w, "status must be success or error", http.StatusBadRequest)
		return
	}
	logLine := fmt.Sprintf("[%s] installer reported %s\n", time.Now().UTC().Format(time.RFC3339), status)
	if err := h.store.MarkInstallationFinished(ic.install.ID, status, logLine, time.Now()); err != nil {
		h.log.Error("boot: cannot finish installation", "id", ic.install.ID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if status == model.InstallSuccess {
		ic.host.Status = model.HostReady
	} else {
		ic.host.Status = model.HostError
	}
	if _, err := h.store.UpdateHost(*ic.host); err != nil {
		h.log.Error("boot: cannot update host status", "mac", ic.host.MAC, "error", err)
	}
	h.log.Info("installation finished", "mac", ic.host.MAC, "status", string(status))
	w.WriteHeader(http.StatusNoContent)
}

// handleISOFile serves the complete original ISO with range support
// (installers like Ubuntu's fetch the whole image via HTTP).
func (h *Handler) handleISOFile(w http.ResponseWriter, r *http.Request) {
	img, ok := h.isoByPathID(w, r)
	if !ok {
		return
	}
	http.ServeFile(w, r, h.isos.Path(img.Filename))
}

// handleISOContent streams one file from inside an ISO (kernels,
// initrds) without extracting or modifying anything.
func (h *Handler) handleISOContent(w http.ResponseWriter, r *http.Request) {
	img, ok := h.isoByPathID(w, r)
	if !ok {
		return
	}
	inner := r.PathValue("path")
	rc, size, err := h.isos.OpenFileInISO(img.Filename, inner)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	if _, err := io.Copy(w, rc); err != nil {
		h.log.Debug("boot: iso content transfer aborted", "iso", img.Filename, "path", inner, "error", err)
	}
}

// contextByToken resolves the installation context for token-scoped
// endpoints; writes the error response when it returns ok=false.
func (h *Handler) contextByToken(w http.ResponseWriter, r *http.Request) (*installContext, bool) {
	inst, err := h.store.InstallationByToken(r.PathValue("token"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return nil, false
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, false
	}
	host, err := h.store.HostByID(inst.HostID)
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}
	ic, reason, err := h.loadInstallContext(host)
	if err != nil || ic == nil {
		h.log.Warn("boot: token resolved but context incomplete", "reason", reason, "error", err)
		http.NotFound(w, r)
		return nil, false
	}
	return ic, true
}

func (h *Handler) isoByPathID(w http.ResponseWriter, r *http.Request) (*model.ISOImage, bool) {
	var id int64
	if _, err := fmt.Sscanf(r.PathValue("id"), "%d", &id); err != nil || id < 1 {
		http.NotFound(w, r)
		return nil, false
	}
	img, err := h.store.ISOByID(id)
	if err != nil {
		http.NotFound(w, r)
		return nil, false
	}
	return img, true
}

func messageScript(msg string) string {
	return fmt.Sprintf("#!ipxe\necho HELBOOT: %s\nsleep 5\nexit\n", msg)
}

// baseURL reconstructs the URL clients used to reach us; boot clients
// always speak plain HTTP directly to HELBOOT.
func baseURL(r *http.Request) string {
	return "http://" + r.Host
}
