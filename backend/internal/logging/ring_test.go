package logging

import (
	"io"
	"log/slog"
	"testing"
)

func TestRingCapturesAndFilters(t *testing.T) {
	ring := NewRing(100)
	log := New(io.Discard, "debug", "text", ring)

	log.Debug("d1")
	log.Info("i1", "key", "value")
	log.Warn("w1")
	log.Error("e1")

	all := ring.Entries(slog.LevelDebug, 0)
	if len(all) != 4 {
		t.Fatalf("captured %d entries, want 4", len(all))
	}
	// Newest first.
	if all[0].Message != "e1" || all[3].Message != "d1" {
		t.Errorf("wrong order: %v", all)
	}

	warnPlus := ring.Entries(slog.LevelWarn, 0)
	if len(warnPlus) != 2 {
		t.Errorf("warn filter returned %d, want 2", len(warnPlus))
	}

	limited := ring.Entries(slog.LevelDebug, 2)
	if len(limited) != 2 || limited[0].Message != "e1" {
		t.Errorf("limit broken: %v", limited)
	}

	// Attrs captured.
	found := false
	for _, e := range all {
		if e.Message == "i1" && e.Attrs["key"] == "value" {
			found = true
		}
	}
	if !found {
		t.Error("attrs not captured")
	}
}

func TestRingWrapsAround(t *testing.T) {
	ring := NewRing(3)
	log := New(io.Discard, "debug", "text", ring)
	for _, msg := range []string{"1", "2", "3", "4", "5"} {
		log.Info(msg)
	}
	got := ring.Entries(slog.LevelDebug, 0)
	if len(got) != 3 {
		t.Fatalf("buffer holds %d, want 3", len(got))
	}
	if got[0].Message != "5" || got[2].Message != "3" {
		t.Errorf("oldest entries not evicted: %v", got)
	}
}

func TestWithAttrsPropagatesToRing(t *testing.T) {
	ring := NewRing(10)
	log := New(io.Discard, "debug", "text", ring).With("service", "tftp")
	log.Info("started")
	got := ring.Entries(slog.LevelDebug, 0)
	if len(got) != 1 || got[0].Attrs["service"] != "tftp" {
		t.Errorf("With attrs missing: %v", got)
	}
}
