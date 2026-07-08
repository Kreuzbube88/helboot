package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

type fakeService struct {
	name  string
	runs  atomic.Int32
	fail  int32 // fail this many times before behaving
	block bool
}

func (f *fakeService) Name() string { return f.name }

func (f *fakeService) Run(ctx context.Context) error {
	n := f.runs.Add(1)
	if n <= f.fail {
		return errors.New("boom")
	}
	if f.block {
		<-ctx.Done()
	}
	return ctx.Err()
}

func TestManagerRestartsCrashedService(t *testing.T) {
	m := NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc := &fakeService{name: "crashy", fail: 2, block: true}
	m.Add(svc)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(done)
	}()

	deadline := time.After(10 * time.Second)
	for svc.runs.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("service was not restarted; runs=%d", svc.runs.Load())
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("manager did not stop after cancel")
	}
}

func TestManagerStopsCleanly(t *testing.T) {
	m := NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.Add(&fakeService{name: "a", block: true})
	m.Add(&fakeService{name: "b", block: true})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("manager did not shut down")
	}
}
