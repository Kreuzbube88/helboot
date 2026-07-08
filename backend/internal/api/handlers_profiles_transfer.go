package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// profileDocument is the portable profile format (§13 export/import).
// ISO references are deliberately excluded: ISO ids are not portable
// between instances — the user re-assigns an ISO after import.
type profileDocument struct {
	Type     string          `json:"type"`
	Version  int             `json:"version"`
	Name     string          `json:"name"`
	Provider string          `json:"provider"`
	Config   json.RawMessage `json:"config"`
}

const profileDocumentType = "helboot-profile"

type cloneProfileRequest struct {
	Name string `json:"name"`
}

// handleCloneProfile copies a profile: the clone starts at version 1
// with the source's current configuration snapshot.
func (s *Server) handleCloneProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	source, err := s.store.ProfileByID(id)
	if err != nil {
		s.storeError(w, err)
		return
	}
	var req cloneProfileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("%s (copy)", source.Name)
	}
	version, err := s.store.ProfileVersionNumber(source.ID, source.CurrentVersion)
	if err != nil {
		s.internalError(w, err)
		return
	}
	clone, err := s.store.CreateProfile(name, source.Provider, source.ISOID, version.Config)
	if err != nil {
		writeError(w, http.StatusConflict, "profile.name_exists", "a profile with this name already exists")
		return
	}
	s.audit(r, "profile.clone", "profile", clone.ID)
	writeJSON(w, http.StatusCreated, clone)
}

// handleExportProfile downloads the current configuration as a portable
// JSON document.
func (s *Server) handleExportProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	profile, err := s.store.ProfileByID(id)
	if err != nil {
		s.storeError(w, err)
		return
	}
	version, err := s.store.ProfileVersionNumber(profile.ID, profile.CurrentVersion)
	if err != nil {
		s.internalError(w, err)
		return
	}
	doc := profileDocument{
		Type:     profileDocumentType,
		Version:  1,
		Name:     profile.Name,
		Provider: profile.Provider,
		Config:   json.RawMessage(version.Config),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="helboot-profile-%d-%s.json"`, profile.ID, time.Now().UTC().Format("20060102")))
	json.NewEncoder(w).Encode(doc)
}

// handleImportProfile creates a profile from an exported document.
func (s *Server) handleImportProfile(w http.ResponseWriter, r *http.Request) {
	var doc profileDocument
	if !decodeJSON(w, r, &doc) {
		return
	}
	if doc.Type != profileDocumentType || doc.Version != 1 {
		writeError(w, http.StatusBadRequest, "profile.invalid_document", "not a HELBOOT profile document")
		return
	}
	if doc.Name == "" {
		writeError(w, http.StatusBadRequest, "profile.invalid_name", "profile name is required")
		return
	}
	if s.registry.Get(doc.Provider) == nil {
		writeError(w, http.StatusBadRequest, "profile.unknown_provider", "unknown provider")
		return
	}
	config := "{}"
	if len(doc.Config) > 0 {
		if !json.Valid(doc.Config) {
			writeError(w, http.StatusBadRequest, "profile.invalid_config", "config must be a valid JSON document")
			return
		}
		config = string(doc.Config)
	}
	created, err := s.store.CreateProfile(doc.Name, doc.Provider, nil, config)
	if err != nil {
		writeError(w, http.StatusConflict, "profile.name_exists", "a profile with this name already exists")
		return
	}
	s.audit(r, "profile.import", "profile", created.ID)
	writeJSON(w, http.StatusCreated, created)
}
