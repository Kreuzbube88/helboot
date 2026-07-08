package api

import (
	"net/http"

	"github.com/kreuzbube88/helboot/backend/internal/auth"
	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// setupRequest is the payload of the first-run wizard (§24). ISO upload
// and first profile are separate optional steps handled by their own
// endpoints after login.
type setupRequest struct {
	Language string `json:"language"`
	Admin    struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"admin"`
	Network struct {
		// Mode is "proxy_dhcp" (existing DHCP server, Mode A) or "dhcp"
		// (HELBOOT is the DHCP server, Mode B). See ADR-0006.
		Mode string `json:"mode"`
	} `json:"network"`
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, _ *http.Request) {
	completed, err := s.store.SetupCompleted()
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"completed": completed})
}

// handleSetupComplete finishes the first-run wizard. It is deliberately
// unauthenticated but only usable exactly once: afterwards it always
// returns 409.
func (s *Server) handleSetupComplete(w http.ResponseWriter, r *http.Request) {
	completed, err := s.store.SetupCompleted()
	if err != nil {
		s.internalError(w, err)
		return
	}
	if completed {
		writeError(w, http.StatusConflict, "setup.already_completed", "setup has already been completed")
		return
	}

	var req setupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Language != "en" && req.Language != "de" {
		writeError(w, http.StatusBadRequest, "setup.invalid_language", "language must be one of: en, de")
		return
	}
	if len(req.Admin.Username) < 3 {
		writeError(w, http.StatusBadRequest, "setup.invalid_username", "username must have at least 3 characters")
		return
	}
	if len(req.Admin.Password) < 10 {
		writeError(w, http.StatusBadRequest, "setup.weak_password", "password must have at least 10 characters")
		return
	}
	if req.Network.Mode != "proxy_dhcp" && req.Network.Mode != "dhcp" {
		writeError(w, http.StatusBadRequest, "setup.invalid_network_mode", "network mode must be one of: proxy_dhcp, dhcp")
		return
	}

	hash, err := auth.HashPassword(req.Admin.Password)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if _, err := s.store.CreateUser(req.Admin.Username, hash, model.RoleAdmin, req.Language); err != nil {
		s.internalError(w, err)
		return
	}
	for key, value := range map[string]string{
		store.SettingUILanguage:     req.Language,
		store.SettingNetworkMode:    req.Network.Mode,
		store.SettingSetupCompleted: "true",
	} {
		if err := s.store.SetSetting(key, value); err != nil {
			s.internalError(w, err)
			return
		}
	}
	s.log.Info("first-run setup completed", "admin", req.Admin.Username, "networkMode", req.Network.Mode)
	writeJSON(w, http.StatusCreated, map[string]bool{"completed": true})
}
