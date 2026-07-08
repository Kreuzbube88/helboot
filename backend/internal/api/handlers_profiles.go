package api

import (
	"encoding/json"
	"net/http"
)

type createProfileRequest struct {
	Name     string          `json:"name"`
	Provider string          `json:"provider"`
	ISOID    *int64          `json:"isoId"`
	Config   json.RawMessage `json:"config"`
}

type updateProfileRequest struct {
	Name  string `json:"name"`
	ISOID *int64 `json:"isoId"`
	// Config, when present, creates a new immutable profile version (§13).
	Config json.RawMessage `json:"config"`
}

func (s *Server) handleListProfiles(w http.ResponseWriter, _ *http.Request) {
	profiles, err := s.store.ListProfiles()
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var req createProfileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "profile.invalid_name", "profile name is required")
		return
	}
	if s.registry.Get(req.Provider) == nil {
		writeError(w, http.StatusBadRequest, "profile.unknown_provider", "unknown provider")
		return
	}
	config := "{}"
	if len(req.Config) > 0 {
		if !json.Valid(req.Config) {
			writeError(w, http.StatusBadRequest, "profile.invalid_config", "config must be a valid JSON document")
			return
		}
		config = string(req.Config)
	}
	created, err := s.store.CreateProfile(req.Name, req.Provider, req.ISOID, config)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := s.store.ProfileByID(id)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleProfileVersions(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.store.ProfileByID(id); err != nil {
		s.storeError(w, err)
		return
	}
	versions, err := s.store.ProfileVersions(id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req updateProfileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "profile.invalid_name", "profile name is required")
		return
	}
	var config *string
	if len(req.Config) > 0 {
		if !json.Valid(req.Config) {
			writeError(w, http.StatusBadRequest, "profile.invalid_config", "config must be a valid JSON document")
			return
		}
		c := string(req.Config)
		config = &c
	}
	updated, err := s.store.UpdateProfile(id, req.Name, req.ISOID, config)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteProfile(id); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
