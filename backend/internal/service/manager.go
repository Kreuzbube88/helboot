// Package service implements the in-process supervisor from ADR-0009:
// one Go binary runs all long-lived services (HTTP, DHCP, TFTP, ...) as
// supervised goroutines with restart-on-failure.
package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Service is a long-running component managed by the Manager. Run must
// block until ctx is cancelled and return ctx.Err() (or nil) on clean
// shutdown; any other error triggers a supervised restart.
type Service interface {
	Name() string
	Run(ctx context.Context) error
}

const (
	initialBackoff = time.Second
	maxBackoff     = 30 * time.Second
)

// Manager supervises a set of services.
type Manager struct {
	log      *slog.Logger
	services []Service
}

// NewManager creates an empty Manager.
func NewManager(log *slog.Logger) *Manager {
	return &Manager{log: log}
}

// Add registers a service. Must be called before Run.
func (m *Manager) Add(s Service) {
	m.services = append(m.services, s)
}

// Run starts every service and blocks until ctx is cancelled and all
// services have stopped. A crashing service is restarted with
// exponential backoff; one bad service never takes the others down.
func (m *Manager) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, svc := range m.services {
		wg.Add(1)
		go func(svc Service) {
			defer wg.Done()
			m.supervise(ctx, svc)
		}(svc)
	}
	wg.Wait()
}

func (m *Manager) supervise(ctx context.Context, svc Service) {
	backoff := initialBackoff
	for {
		m.log.Info("service starting", "service", svc.Name())
		start := time.Now()
		err := svc.Run(ctx)

		if ctx.Err() != nil {
			m.log.Info("service stopped", "service", svc.Name())
			return
		}
		if err == nil || errors.Is(err, context.Canceled) {
			m.log.Info("service exited", "service", svc.Name())
			return
		}

		// A service that ran for a while before failing gets a fresh
		// backoff window; rapid crash loops back off exponentially.
		if time.Since(start) > time.Minute {
			backoff = initialBackoff
		}
		m.log.Error("service crashed, restarting", "service", svc.Name(),
			"error", err, "backoff", backoff.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}
