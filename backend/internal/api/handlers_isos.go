package api

import (
	"errors"
	"net/http"

	"github.com/kreuzbube88/helboot/backend/internal/iso"
)

func (s *Server) handleListISOs(w http.ResponseWriter, _ *http.Request) {
	images, err := s.store.ListISOs()
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, images)
}

func (s *Server) handleGetISO(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	img, err := s.store.ISOByID(id)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, img)
}

// handleUploadISO accepts a multipart upload (field "file"). The body is
// streamed to disk — ISOs are gigabytes, so there is deliberately no
// in-memory buffering and no MaxBytesReader here.
func (s *Server) handleUploadISO(w http.ResponseWriter, r *http.Request) {
	if s.isos == nil {
		writeError(w, http.StatusServiceUnavailable, "iso.unavailable", "ISO management is not configured")
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "iso.invalid_upload", "expected a multipart upload")
		return
	}
	part, err := mr.NextPart()
	if err != nil || part.FormName() != "file" {
		writeError(w, http.StatusBadRequest, "iso.invalid_upload", "expected a single 'file' part")
		return
	}
	defer part.Close()

	img, err := s.isos.Import(part.FileName(), part)
	switch {
	case errors.Is(err, iso.ErrInvalidFilename):
		writeError(w, http.StatusBadRequest, "iso.invalid_filename",
			"filename must be simple and end in .iso or .img")
		return
	case errors.Is(err, iso.ErrExists):
		writeError(w, http.StatusConflict, "iso.exists", "an ISO with this filename already exists")
		return
	case err != nil:
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, img)
}

// handleScanISOs indexes files already present in the ISO directory,
// e.g. an existing NAS share mounted into the container.
func (s *Server) handleScanISOs(w http.ResponseWriter, _ *http.Request) {
	if s.isos == nil {
		writeError(w, http.StatusServiceUnavailable, "iso.unavailable", "ISO management is not configured")
		return
	}
	added, err := s.isos.ScanDir()
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, added)
}

func (s *Server) handleDeleteISO(w http.ResponseWriter, r *http.Request) {
	if s.isos == nil {
		writeError(w, http.StatusServiceUnavailable, "iso.unavailable", "ISO management is not configured")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.isos.Delete(id); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
