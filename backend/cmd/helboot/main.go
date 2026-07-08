// Command helboot is the HELBOOT server: one binary that supervises the
// HTTP API/UI and, in later milestones, the DHCP/ProxyDHCP and TFTP boot
// services (ADR-0009).
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kreuzbube88/helboot/backend/api"
	apihttp "github.com/kreuzbube88/helboot/backend/internal/api"
	"github.com/kreuzbube88/helboot/backend/internal/backup"
	"github.com/kreuzbube88/helboot/backend/internal/boot"
	"github.com/kreuzbube88/helboot/backend/internal/config"
	"github.com/kreuzbube88/helboot/backend/internal/db"
	"github.com/kreuzbube88/helboot/backend/internal/iso"
	"github.com/kreuzbube88/helboot/backend/internal/logging"
	"github.com/kreuzbube88/helboot/backend/internal/provider"
	"github.com/kreuzbube88/helboot/backend/internal/service"
	"github.com/kreuzbube88/helboot/backend/internal/store"
	"github.com/kreuzbube88/helboot/backend/internal/web"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "helboot:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.FromEnv()
	// The ring buffer feeds the UI log viewer (§30); 1000 entries cover
	// a comfortable scrollback without measurable memory cost.
	logRing := logging.NewRing(1000)
	log := logging.New(os.Stdout, cfg.LogLevel, cfg.LogFormat, logRing)
	log.Info("starting HELBOOT", "version", version)

	for _, dir := range []string{
		cfg.DataDir,
		filepath.Join(cfg.DataDir, "isos"),
		filepath.Join(cfg.DataDir, "logs"),
		filepath.Join(cfg.AssetsPath(), "tftp"),
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create data directory %s: %w", dir, err)
		}
	}

	// A staged backup restore (POST /api/v1/backup/import) is applied
	// here, before the database is opened (§31).
	if applied, err := backup.ApplyPendingRestore(cfg.DataDir, cfg.DatabasePath()); err != nil {
		return fmt.Errorf("apply staged restore: %w", err)
	} else if applied {
		log.Info("staged backup restore applied")
	}

	sqlDB, err := db.Open(cfg.DatabasePath())
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := db.Migrate(sqlDB); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	st := store.New(sqlDB)

	registry, err := provider.LoadDir(cfg.ProvidersDir, log)
	if err != nil {
		return err
	}

	isoManager := iso.NewManager(log, filepath.Join(cfg.DataDir, "isos"), st, registry)
	server := apihttp.New(apihttp.Deps{
		Log:         log,
		Store:       st,
		Registry:    registry,
		Version:     version,
		OpenAPISpec: api.OpenAPISpec,
		StaticFiles: web.Handler(),
		Boot:        boot.New(log, st, registry, isoManager, cfg.AssetsPath(), cfg.ProvidersDir),
		ISOs:        isoManager,
		Backup:      backup.NewManager(sqlDB, cfg.DataDir, version),
		LogRing:     logRing,
		AssetsDir:   cfg.AssetsPath(),
	})
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Boot-network services (TFTP + DHCP/ProxyDHCP) run supervised in
	// this same process (ADR-0009). Configuration changes require a
	// restart, which the API reports to the UI.
	manager := service.NewManager(log)
	for _, svc := range buildNetworkServices(cfg, st, log) {
		manager.Add(svc)
	}
	managerDone := make(chan struct{})
	go func() {
		manager.Run(ctx)
		close(managerDone)
	}()

	// Housekeeping: purge expired sessions periodically.
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := st.DeleteExpiredSessions(time.Now()); err != nil {
					log.Error("session cleanup failed", "error", err)
				}
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = httpServer.Shutdown(shutdownCtx)
	select {
	case <-managerDone:
	case <-shutdownCtx.Done():
		log.Warn("network services did not stop in time")
	}
	return err
}
