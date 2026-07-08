package api

import (
	"errors"
	"net/http"

	"github.com/kreuzbube88/helboot/backend/internal/auth"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

type createInstallationRequest struct {
	HostID int64 `json:"hostId"`
	// ProfileID overrides the host's assigned profile when set.
	ProfileID *int64 `json:"profileId"`
}

func (s *Server) handleListInstallations(w http.ResponseWriter, _ *http.Request) {
	installs, err := s.store.ListInstallations()
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, installs)
}

func (s *Server) handleGetInstallation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	inst, err := s.store.InstallationByID(id)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inst)
}

// handleCreateInstallation queues an installation: the next network boot
// of the host starts the installer (§16).
func (s *Server) handleCreateInstallation(w http.ResponseWriter, r *http.Request) {
	var req createInstallationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	host, err := s.store.HostByID(req.HostID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "installation.unknown_host", "unknown host")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}

	profileID := host.ProfileID
	if req.ProfileID != nil {
		profileID = req.ProfileID
	}
	if profileID == nil {
		writeError(w, http.StatusBadRequest, "installation.no_profile",
			"host has no profile assigned and none was provided")
		return
	}
	profile, err := s.store.ProfileByID(*profileID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "installation.unknown_profile", "unknown profile")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}

	if _, err := s.store.ActiveInstallationForHost(host.ID); err == nil {
		writeError(w, http.StatusConflict, "installation.already_active",
			"host already has a waiting or running installation")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		s.internalError(w, err)
		return
	}

	// Installations pin the profile's current version snapshot so later
	// profile edits never change what a queued host installs (§13).
	version, err := s.store.ProfileVersionNumber(profile.ID, profile.CurrentVersion)
	if err != nil {
		s.internalError(w, err)
		return
	}
	token, err := auth.NewToken()
	if err != nil {
		s.internalError(w, err)
		return
	}
	inst, err := s.store.CreateInstallation(host.ID, version.ID, token)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.log.Info("installation queued", "host", host.MAC, "profile", profile.Name, "version", version.Version)
	writeJSON(w, http.StatusCreated, inst)
}

// handleDeleteInstallation cancels a queued installation. Running or
// finished installations are history and cannot be deleted.
func (s *Server) handleDeleteInstallation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.store.InstallationByID(id); err != nil {
		s.storeError(w, err)
		return
	}
	if err := s.store.DeleteInstallation(id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusConflict, "installation.not_cancellable",
			"only waiting installations can be cancelled")
		return
	} else if err != nil {
		s.internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
