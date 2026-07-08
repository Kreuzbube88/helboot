package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

type contextKey int

const (
	ctxUser contextKey = iota
	ctxSession
)

// SessionCookie is the name of the session cookie (ADR-0007).
const SessionCookie = "helboot_session"

// recoverPanics converts handler panics into opaque 500 responses so a
// single bad request can never take the server down.
func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic in handler", "panic", rec, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeaders sets baseline browser protections on every response.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// logRequests emits one structured log line per request.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Debug("http request",
			"method", r.Method, "path", r.URL.Path,
			"status", rec.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// require wraps a handler with session authentication, role
// authorization and — for mutating methods — CSRF verification.
func (s *Server) require(min model.Role, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "auth.unauthorized", "authentication required")
			return
		}
		sess, err := s.store.SessionByToken(cookie.Value, time.Now())
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "auth.session_expired", "session expired or invalid")
			return
		}
		if err != nil {
			s.internalError(w, err)
			return
		}
		user, err := s.store.UserByID(sess.UserID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "auth.unauthorized", "authentication required")
			return
		}
		if !user.Role.AtLeast(min) {
			writeError(w, http.StatusForbidden, "auth.forbidden", "insufficient permissions")
			return
		}
		// Double-submit is not needed with server-side sessions: mutating
		// requests must echo the session's CSRF token in a header, which
		// cross-site forms cannot do (§29).
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if r.Header.Get("X-CSRF-Token") != sess.CSRFToken {
				writeError(w, http.StatusForbidden, "auth.csrf", "missing or invalid CSRF token")
				return
			}
		}
		ctx := context.WithValue(r.Context(), ctxUser, user)
		ctx = context.WithValue(ctx, ctxSession, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userFrom(r *http.Request) *model.User {
	u, _ := r.Context().Value(ctxUser).(*model.User)
	return u
}

func sessionFrom(r *http.Request) *model.Session {
	s, _ := r.Context().Value(ctxSession).(*model.Session)
	return s
}

// rateLimiter is a per-client token bucket, used on the login endpoint.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	burst   float64
	refill  time.Duration // time to earn one token back
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter(burst int, refill time.Duration) *rateLimiter {
	return &rateLimiter{
		buckets: map[string]*bucket{},
		burst:   float64(burst),
		refill:  refill,
	}
}

// allow reports whether the client identified by key may proceed.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() / l.refill.Seconds()
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// clientIP extracts the remote IP for rate limiting. HELBOOT is served
// directly (no trusted proxy layer by default), so RemoteAddr is
// authoritative; X-Forwarded-For is deliberately ignored to prevent
// spoofed limiter keys.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
