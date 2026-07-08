// Package logging configures HELBOOT's structured logger (log/slog).
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// New returns a slog.Logger writing to w with the given level
// (debug|info|warn|error) and format (text|json). Unknown values fall
// back to info/text so a misconfigured logger never silences errors.
// When ring is non-nil every record is additionally captured for the
// API log viewer.
func New(w io.Writer, level, format string, ring *Ring) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	var handler slog.Handler
	if strings.EqualFold(format, "json") {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	if ring != nil {
		handler = WrapWithRing(handler, ring)
	}
	return slog.New(handler)
}

// ParseLevel maps a level name to its slog.Level (info on unknown input).
func ParseLevel(s string) slog.Level { return parseLevel(s) }

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
