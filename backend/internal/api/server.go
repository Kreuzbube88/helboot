// Package api implements HELBOOT's REST API v1 (ADR-0010). The frontend
// communicates exclusively through these endpoints; there is no
// privileged internal channel.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/backup"
	"github.com/kreuzbube88/helboot/backend/internal/boot"
	"github.com/kreuzbube88/helboot/backend/internal/iso"
	"github.com/kreuzbube88/helboot/backend/internal/logging"
	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/provider"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

// SessionTTL is the absolute lifetime of a login session.
const SessionTTL = 7 * 24 * time.Hour

// Deps are the collaborators the API server exposes over HTTP. Optional
// fields may be nil: StaticFiles (API-only mode, used in tests), Boot
// (no /boot surface); nil ISOs, Backup or LogRing make their endpoints
// return 503.
type Deps struct {
	Log         *slog.Logger
	Store       *store.Store
	Registry    *provider.Registry
	Version     string
	OpenAPISpec []byte
	StaticFiles http.Handler
	Boot        *boot.Handler
	ISOs        *iso.Manager
	Backup      *backup.Manager
	LogRing     *logging.Ring
}

// Server wires the HTTP routes to the application services.
type Server struct {
	log          *slog.Logger
	store        *store.Store
	registry     *provider.Registry
	version      string
	openAPISpec  []byte
	staticFiles  http.Handler
	boot         *boot.Handler
	isos         *iso.Manager
	backup       *backup.Manager
	logRing      *logging.Ring
	loginLimiter *rateLimiter
	handler      http.Handler
}

// New builds the API server from its dependencies.
func New(d Deps) *Server {
	s := &Server{
		log:         d.Log,
		store:       d.Store,
		registry:    d.Registry,
		version:     d.Version,
		openAPISpec: d.OpenAPISpec,
		staticFiles: d.StaticFiles,
		boot:        d.Boot,
		isos:        d.ISOs,
		backup:      d.Backup,
		logRing:     d.LogRing,
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
	mux.Handle("GET /api/v1/logs", s.require(model.RoleViewer, s.handleLogs))
	mux.Handle("GET /api/v1/backup/export", s.require(model.RoleAdmin, s.handleBackupExport))
	mux.Handle("POST /api/v1/backup/import", s.require(model.RoleAdmin, s.handleBackupImport))

	mux.Handle("GET /api/v1/providers", s.require(model.RoleViewer, s.handleListProviders))
	mux.Handle("GET /api/v1/providers/{name}", s.require(model.RoleViewer, s.handleGetProvider))

	mux.Handle("GET /api/v1/hosts", s.require(model.RoleViewer, s.handleListHosts))
	mux.Handle("POST /api/v1/hosts", s.require(model.RoleOperator, s.handleCreateHost))
	mux.Handle("GET /api/v1/hosts/{id}", s.require(model.RoleViewer, s.handleGetHost))
	mux.Handle("PUT /api/v1/hosts/{id}", s.require(model.RoleOperator, s.handleUpdateHost))
	mux.Handle("DELETE /api/v1/hosts/{id}", s.require(model.RoleOperator, s.handleDeleteHost))

	mux.Handle("GET /api/v1/installations", s.require(model.RoleViewer, s.handleListInstallations))
	mux.Handle("POST /api/v1/installations", s.require(model.RoleOperator, s.handleCreateInstallation))
	mux.Handle("GET /api/v1/installations/{id}", s.require(model.RoleViewer, s.handleGetInstallation))
	mux.Handle("DELETE /api/v1/installations/{id}", s.require(model.RoleOperator, s.handleDeleteInstallation))

	mux.Handle("GET /api/v1/isos", s.require(model.RoleViewer, s.handleListISOs))
	mux.Handle("GET /api/v1/isos/{id}", s.require(model.RoleViewer, s.handleGetISO))
	mux.Handle("POST /api/v1/isos/upload", s.require(model.RoleOperator, s.handleUploadISO))
	mux.Handle("POST /api/v1/isos/scan", s.require(model.RoleOperator, s.handleScanISOs))
	mux.Handle("DELETE /api/v1/isos/{id}", s.require(model.RoleOperator, s.handleDeleteISO))

	mux.Handle("GET /api/v1/network/config", s.require(model.RoleViewer, s.handleGetNetworkConfig))
	mux.Handle("PUT /api/v1/network/config", s.require(model.RoleAdmin, s.handlePutNetworkConfig))

	mux.Handle("GET /api/v1/profiles", s.require(model.RoleViewer, s.handleListProfiles))
	mux.Handle("POST /api/v1/profiles", s.require(model.RoleOperator, s.handleCreateProfile))
	mux.Handle("GET /api/v1/profiles/{id}", s.require(model.RoleViewer, s.handleGetProfile))
	mux.Handle("GET /api/v1/profiles/{id}/versions", s.require(model.RoleViewer, s.handleProfileVersions))
	mux.Handle("PUT /api/v1/profiles/{id}", s.require(model.RoleOperator, s.handleUpdateProfile))
	mux.Handle("DELETE /api/v1/profiles/{id}", s.require(model.RoleOperator, s.handleDeleteProfile))

	// Unauthenticated boot surface for firmware/iPXE (ADR-0010).
	if s.boot != nil {
		s.boot.Register(mux)
	}

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
