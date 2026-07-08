package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/logging"
)

// handleLogs serves recent log entries for the UI log viewer (§30).
// Query: ?level=debug|info|warn|error (default info), ?limit=N.
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if s.logRing == nil {
		writeError(w, http.StatusServiceUnavailable, "logs.unavailable", "log buffer is not configured")
		return
	}
	level := logging.ParseLevel(r.URL.Query().Get("level"))
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 1000 {
			writeError(w, http.StatusBadRequest, "logs.invalid_limit", "limit must be between 1 and 1000")
			return
		}
		limit = n
	}
	writeJSON(w, http.StatusOK, s.logRing.Entries(level, limit))
}

// handleBackupExport streams a full-state backup archive (admin).
func (s *Server) handleBackupExport(w http.ResponseWriter, _ *http.Request) {
	if s.backup == nil {
		writeError(w, http.StatusServiceUnavailable, "backup.unavailable", "backup is not configured")
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="helboot-backup-%s.tar.gz"`, time.Now().UTC().Format("20060102-150405")))
	if err := s.backup.Export(w); err != nil {
		// Headers are already sent; all we can do is log and abort.
		s.log.Error("backup export failed", "error", err)
	}
}

// handleBackupImport stages a backup archive for restore on the next
// restart (admin). Multipart with a single "file" part.
func (s *Server) handleBackupImport(w http.ResponseWriter, r *http.Request) {
	if s.backup == nil {
		writeError(w, http.StatusServiceUnavailable, "backup.unavailable", "backup is not configured")
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "backup.invalid_upload", "expected a multipart upload")
		return
	}
	part, err := mr.NextPart()
	if err != nil || part.FormName() != "file" {
		writeError(w, http.StatusBadRequest, "backup.invalid_upload", "expected a single 'file' part")
		return
	}
	defer part.Close()
	if err := s.backup.Import(part); err != nil {
		writeError(w, http.StatusBadRequest, "backup.invalid_archive", err.Error())
		return
	}
	s.log.Info("backup import staged; restart required to apply")
	s.audit(r, "backup.import", "backup", 0)
	writeJSON(w, http.StatusOK, map[string]bool{"staged": true, "restartRequired": true})
}
