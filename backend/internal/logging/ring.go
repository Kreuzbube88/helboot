package logging

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Entry is one captured log record, shaped for the UI log viewer (§30).
type Entry struct {
	Time    time.Time         `json:"time"`
	Level   string            `json:"level"`
	Message string            `json:"message"`
	Attrs   map[string]string `json:"attrs,omitempty"`
}

// Ring is a fixed-size in-memory buffer of recent log entries, exposed
// through the API so the UI can show logs without file access.
type Ring struct {
	mu      sync.Mutex
	entries []Entry
	next    int
	full    bool
}

// NewRing creates a buffer holding the most recent size entries.
func NewRing(size int) *Ring {
	if size < 1 {
		size = 1
	}
	return &Ring{entries: make([]Entry, size)}
}

func (r *Ring) append(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[r.next] = e
	r.next = (r.next + 1) % len(r.entries)
	if r.next == 0 {
		r.full = true
	}
}

// Entries returns buffered entries newest first, filtered to minLevel,
// capped at limit (0 = everything buffered).
func (r *Ring) Entries(minLevel slog.Level, limit int) []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := r.next
	if r.full {
		count = len(r.entries)
	}
	out := make([]Entry, 0, count)
	for i := 0; i < count; i++ {
		// Walk backwards from the most recently written slot.
		idx := (r.next - 1 - i + len(r.entries)) % len(r.entries)
		e := r.entries[idx]
		if parseLevel(e.Level) < minLevel {
			continue
		}
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// ringHandler tees records into the ring while delegating to the real
// handler. It implements slog.Handler.
type ringHandler struct {
	base  slog.Handler
	ring  *Ring
	attrs []slog.Attr
}

// WrapWithRing returns a handler that captures every record into ring
// and forwards it to base.
func WrapWithRing(base slog.Handler, ring *Ring) slog.Handler {
	return &ringHandler{base: base, ring: ring}
}

func (h *ringHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

func (h *ringHandler) Handle(ctx context.Context, rec slog.Record) error {
	attrs := map[string]string{}
	for _, a := range h.attrs {
		attrs[a.Key] = a.Value.String()
	}
	rec.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	h.ring.append(Entry{
		Time:    rec.Time,
		Level:   rec.Level.String(),
		Message: rec.Message,
		Attrs:   attrs,
	})
	return h.base.Handle(ctx, rec)
}

func (h *ringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ringHandler{
		base:  h.base.WithAttrs(attrs),
		ring:  h.ring,
		attrs: append(append([]slog.Attr{}, h.attrs...), attrs...),
	}
}

func (h *ringHandler) WithGroup(name string) slog.Handler {
	// Groups are flattened in the ring view; the base handler keeps them.
	return &ringHandler{base: h.base.WithGroup(name), ring: h.ring, attrs: h.attrs}
}
