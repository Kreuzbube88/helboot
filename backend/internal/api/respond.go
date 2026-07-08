package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// errorBody is the single error envelope used by every endpoint
// (ADR-0010). Code is a stable identifier the frontend uses as its i18n
// key; Message is an untranslated English fallback.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}

// internalError logs the cause and returns an opaque 500 — internals are
// never leaked to clients.
func (s *Server) internalError(w http.ResponseWriter, err error) {
	s.log.Error("internal error", "error", err)
	writeError(w, http.StatusInternalServerError, "internal", "internal server error")
}

// storeError maps store.ErrNotFound to a 404 and everything else to a 500.
func (s *Server) storeError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	s.internalError(w, err)
}

// decodeJSON parses the request body into v with sane limits and strict
// field checking (input validation, §29).
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB is plenty for JSON
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "validation.invalid_json", "invalid JSON body: "+err.Error())
		return false
	}
	return true
}
