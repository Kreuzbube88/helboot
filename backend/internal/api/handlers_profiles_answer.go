package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kreuzbube88/helboot/backend/internal/answer"
	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// answerPreviewResponse is the rendered answer file of one profile
// version, exactly as the boot chain would serve it — with placeholder
// boot parameters instead of live installation tokens (ADR-0014).
type answerPreviewResponse struct {
	Format     string `json:"format"`
	Content    string `json:"content"`
	Overridden bool   `json:"overridden"`
}

type answerOverrideRequest struct {
	// Content replaces the provider template for this version; empty
	// clears the override (= regenerate from the template).
	Content string `json:"content"`
}

// previewParams are recognizable placeholders: real structure, no
// secrets of a live installation.
func previewParams() answer.Params {
	const base = "http://helboot.example"
	return answer.Params{
		AnswerFileURL: base + "/boot/answer/PREVIEW-TOKEN",
		ISOURL:        base + "/boot/isofile/0",
		ISOContentURL: base + "/boot/iso/0",
		CloudInitURL:  base + "/boot/cloudinit/PREVIEW-TOKEN/",
		ReportURL:     base + "/boot/report/PREVIEW-TOKEN",
		Hostname:      "preview-host",
		MAC:           "52:54:00:00:00:01",
	}
}

func (s *Server) handleAnswerPreview(w http.ResponseWriter, r *http.Request) {
	profile, version, ok := s.profileVersionFromPath(w, r)
	if !ok {
		return
	}
	manifest := s.registry.Get(profile.Provider)
	if manifest == nil {
		writeError(w, http.StatusConflict, "profile.provider_missing", "the profile's provider is not installed")
		return
	}

	var out []byte
	var err error
	if version.AnswerOverride != "" {
		out, err = answer.Render(version.AnswerOverride, version.Config, previewParams())
	} else if manifest.AnswerFile.Template == "" {
		writeError(w, http.StatusNotFound, "profile.no_answer_file", "this provider generates no answer file")
		return
	} else {
		templatePath := filepath.Join(manifest.Dir, filepath.Clean("/"+manifest.AnswerFile.Template))
		out, err = answer.RenderFile(templatePath, version.Config, previewParams())
	}
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "profile.no_answer_file", "the provider's answer file template is missing")
			return
		}
		writeError(w, http.StatusUnprocessableEntity, "profile.answer_render_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, answerPreviewResponse{
		Format:     manifest.AnswerFile.Format,
		Content:    string(out),
		Overridden: version.AnswerOverride != "",
	})
}

func (s *Server) handleSetAnswerOverride(w http.ResponseWriter, r *http.Request) {
	profile, version, ok := s.profileVersionFromPath(w, r)
	if !ok {
		return
	}
	var req answerOverrideRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	// Reject overrides the template engine cannot parse now instead of
	// failing at boot time in front of a waiting machine.
	if req.Content != "" {
		if _, err := answer.Render(req.Content, version.Config, previewParams()); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "profile.answer_render_failed", err.Error())
			return
		}
	}
	if err := s.store.SetAnswerOverride(profile.ID, version.Version, req.Content); err != nil {
		s.storeError(w, err)
		return
	}
	action := "profile.answer_override.set"
	if req.Content == "" {
		action = "profile.answer_override.clear"
	}
	s.audit(r, action, "profile", profile.ID)
	w.WriteHeader(http.StatusNoContent)
}

// profileVersionFromPath resolves {id} and {version} to a profile and
// one of its version snapshots. Returns ok=false after writing the
// error response.
func (s *Server) profileVersionFromPath(w http.ResponseWriter, r *http.Request) (*model.Profile, *model.ProfileVersion, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return nil, nil, false
	}
	versionNo, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || versionNo < 1 {
		writeError(w, http.StatusBadRequest, "validation.invalid_id", "invalid version in path")
		return nil, nil, false
	}
	profile, err := s.store.ProfileByID(id)
	if err != nil {
		s.storeError(w, err)
		return nil, nil, false
	}
	version, err := s.store.ProfileVersionNumber(profile.ID, versionNo)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "profile.unknown_version", "the profile has no such version")
		return nil, nil, false
	}
	if err != nil {
		s.internalError(w, err)
		return nil, nil, false
	}
	return profile, version, true
}
