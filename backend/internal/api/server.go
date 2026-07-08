// Package api implements HELBOOT's REST API v1 (ADR-0010). The frontend
// communicates exclusively through these endpoints; there is no
// privileged internal channel.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/provider"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// SessionTTL is the absolute lifetime of a login session.
const SessionTTL = 7 * 24 * time.Hour

// Server wires the HTTP routes to the application services.
type Server struct {
	log          *slog.Logger
	store        *store.Store
	registry     *provider.Registry
	version      string
	openAPISpec  []byte
	staticFiles  http.Handler
	loginLimiter *rateLimiter
	handler      http.Handler
}

// New builds the API server. staticFiles serves the embedded frontend
// and may be nil (API-only mode, used in tests). openAPISpec is the raw
// OpenAPI document served at /api/v1/openapi.yaml.
func New(log *slog.Logger, st *store.Store, reg *provider.Registry, version string, openAPISpec []byte, staticFiles http.Handler) *Server {
	s := &Server{
		log:         log,
		store:       st,
		registry:    reg,
		version:     version,
		openAPISpec: openAPISpec,
		staticFiles: staticFiles,
		// 5 attempts immediately, then one attempt every 2 seconds per
		// client IP — brute-force protection on login (§29).
		loginLimiter: newRateLimiter(5, 2*time.Second),
	}
	s.handler = s.buildRoutes()
	return s
}

// Handler returns the fully wrapped HTTP handler.
func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) buildRoutes() http.Handler {
	mux := http.NewServeMux()

	// Unauthenticated endpoints.
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/openapi.yaml", s.handleOpenAPI)
	mux.HandleFunc("GET /api/v1/setup/status", s.handleSetupStatus)
	mux.HandleFunc("POST /api/v1/setup", s.handleSetupComplete)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)

	// Session-protected endpoints, by minimum role.
	mux.Handle("POST /api/v1/auth/logout", s.require(model.RoleViewer, s.handleLogout))
	mux.Handle("GET /api/v1/auth/me", s.require(model.RoleViewer, s.handleMe))

	mux.Handle("GET /api/v1/system/info", s.require(model.RoleViewer, s.handleSystemInfo))

	mux.Handle("GET /api/v1/providers", s.require(model.RoleViewer, s.handleListProviders))
	mux.Handle("GET /api/v1/providers/{name}", s.require(model.RoleViewer, s.handleGetProvider))

	mux.Handle("GET /api/v1/hosts", s.require(model.RoleViewer, s.handleListHosts))
	mux.Handle("POST /api/v1/hosts", s.require(model.RoleOperator, s.handleCreateHost))
	mux.Handle("GET /api/v1/hosts/{id}", s.require(model.RoleViewer, s.handleGetHost))
	mux.Handle("PUT /api/v1/hosts/{id}", s.require(model.RoleOperator, s.handleUpdateHost))
	mux.Handle("DELETE /api/v1/hosts/{id}", s.require(model.RoleOperator, s.handleDeleteHost))

	mux.Handle("GET /api/v1/profiles", s.require(model.RoleViewer, s.handleListProfiles))
	mux.Handle("POST /api/v1/profiles", s.require(model.RoleOperator, s.handleCreateProfile))
	mux.Handle("GET /api/v1/profiles/{id}", s.require(model.RoleViewer, s.handleGetProfile))
	mux.Handle("GET /api/v1/profiles/{id}/versions", s.require(model.RoleViewer, s.handleProfileVersions))
	mux.Handle("PUT /api/v1/profiles/{id}", s.require(model.RoleOperator, s.handleUpdateProfile))
	mux.Handle("DELETE /api/v1/profiles/{id}", s.require(model.RoleOperator, s.handleDeleteProfile))

	// Unknown API paths must return a JSON 404, not the SPA fallback.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "api.not_found", "unknown API endpoint")
	})

	// Everything else is the embedded frontend (SPA fallback).
	if s.staticFiles != nil {
		mux.Handle("/", s.staticFiles)
	}

	return s.recoverPanics(s.securityHeaders(s.logRequests(mux)))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(s.openAPISpec)
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, _ *http.Request) {
	completed, err := s.store.SetupCompleted()
	if err != nil {
		s.internalError(w, err)
		return
	}
	mode, err := s.store.GetSetting(store.SettingNetworkMode)
	if err != nil && err != store.ErrNotFound {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":        s.version,
		"setupCompleted": completed,
		"networkMode":    mode,
		"providers":      len(s.registry.All()),
	})
}
