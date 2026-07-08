package api

import (
	"net/http"
	"os"
	"path/filepath"
)

// bootMediaFiles maps the public format name to the asset filename.
// The images are generic iPXE media (§21): once booted, iPXE performs
// DHCP and HELBOOT's ProxyDHCP/DHCP service hands it the boot script
// URL — no per-instance image building required.
var bootMediaFiles = map[string]struct{ file, contentType string }{
	"iso": {"ipxe.iso", "application/x-iso9660-image"},
	"img": {"ipxe.usb", "application/octet-stream"},
}

// handleBootMedia serves the downloadable boot medium for machines
// without PXE-capable firmware.
func (s *Server) handleBootMedia(w http.ResponseWriter, r *http.Request) {
	media, ok := bootMediaFiles[r.PathValue("format")]
	if !ok {
		writeError(w, http.StatusBadRequest, "bootmedia.invalid_format", "format must be one of: iso, img")
		return
	}
	if s.assetsDir == "" {
		writeError(w, http.StatusServiceUnavailable, "bootmedia.unavailable", "boot media are not configured")
		return
	}
	path := filepath.Join(s.assetsDir, "tftp", media.file)
	if _, err := os.Stat(path); err != nil {
		writeError(w, http.StatusNotFound, "bootmedia.missing",
			"boot medium not found; run deploy/scripts/fetch-ipxe.sh or use the Docker image")
		return
	}
	w.Header().Set("Content-Type", media.contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="helboot-boot.`+r.PathValue("format")+`"`)
	http.ServeFile(w, r, path)
}
