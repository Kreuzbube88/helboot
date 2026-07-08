package api

import (
	"net/http"
	"strconv"

	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/netutil"
)

// hostRequest is the writable subset of a host (§14).
type hostRequest struct {
	MAC       string   `json:"mac"`
	Hostname  string   `json:"hostname"`
	Vendor    string   `json:"vendor"`
	Model     string   `json:"model"`
	Serial    string   `json:"serial"`
	AssetID   string   `json:"assetId"`
	Tags      []string `json:"tags"`
	Firmware  string   `json:"firmware"`
	Arch      string   `json:"arch"`
	ProfileID *int64   `json:"profileId"`
}

// validate normalizes the MAC and checks enum fields. Returns false
// after writing the error response.
func (req *hostRequest) validate(w http.ResponseWriter) bool {
	mac, err := netutil.NormalizeMAC(req.MAC)
	if err != nil {
		writeError(w, http.StatusBadRequest, "host.invalid_mac", err.Error())
		return false
	}
	req.MAC = mac
	switch req.Firmware {
	case "", "bios", "uefi":
	default:
		writeError(w, http.StatusBadRequest, "host.invalid_firmware", "firmware must be one of: bios, uefi")
		return false
	}
	return true
}

func (req *hostRequest) apply(h *model.Host) {
	h.MAC = req.MAC
	h.Hostname = req.Hostname
	h.Vendor = req.Vendor
	h.Model = req.Model
	h.Serial = req.Serial
	h.AssetID = req.AssetID
	h.Tags = req.Tags
	h.Firmware = req.Firmware
	h.Arch = req.Arch
	h.ProfileID = req.ProfileID
}

func (s *Server) handleListHosts(w http.ResponseWriter, _ *http.Request) {
	hosts, err := s.store.ListHosts()
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, hosts)
}

func (s *Server) handleCreateHost(w http.ResponseWriter, r *http.Request) {
	var req hostRequest
	if !decodeJSON(w, r, &req) || !req.validate(w) {
		return
	}
	if _, err := s.store.HostByMAC(req.MAC); err == nil {
		writeError(w, http.StatusConflict, "host.mac_exists", "a host with this MAC address already exists")
		return
	}
	var h model.Host
	req.apply(&h)
	h.Status = model.HostReady // manually registered hosts skip discovery
	created, err := s.store.CreateHost(h)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r, "host.create", "host", created.ID)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetHost(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	h, err := s.store.HostByID(id)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func (s *Server) handleUpdateHost(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	existing, err := s.store.HostByID(id)
	if err != nil {
		s.storeError(w, err)
		return
	}
	var req hostRequest
	if !decodeJSON(w, r, &req) || !req.validate(w) {
		return
	}
	req.apply(existing)
	updated, err := s.store.UpdateHost(*existing)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteHost(id); err != nil {
		s.storeError(w, err)
		return
	}
	s.audit(r, "host.delete", "host", id)
	w.WriteHeader(http.StatusNoContent)
}

// pathID parses the {id} path segment. Returns false after writing the
// error response.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "validation.invalid_id", "invalid id in path")
		return 0, false
	}
	return id, true
}
