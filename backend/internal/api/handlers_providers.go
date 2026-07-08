package api

import "net/http"

func (s *Server) handleListProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.registry.All())
}

func (s *Server) handleGetProvider(w http.ResponseWriter, r *http.Request) {
	m := s.registry.Get(r.PathValue("name"))
	if m == nil {
		writeError(w, http.StatusNotFound, "provider.not_found", "unknown provider")
		return
	}
	writeJSON(w, http.StatusOK, m)
}
