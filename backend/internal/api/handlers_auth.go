package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/kreuzbube88/helboot/backend/internal/auth"
	"github.com/kreuzbube88/helboot/backend/internal/model"
	"github.com/kreuzbube88/helboot/backend/internal/store"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type sessionResponse struct {
	User      *model.User `json:"user"`
	CSRFToken string      `json:"csrfToken"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.loginLimiter.allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "auth.rate_limited", "too many login attempts, slow down")
		return
	}
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	user, err := s.store.UserByUsername(req.Username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.internalError(w, err)
		return
	}
	// Verify against a constant dummy hash when the user does not exist,
	// so response timing does not reveal valid usernames.
	hash := dummyHash
	if user != nil {
		hash = user.PasswordHash
	}
	ok, verr := auth.VerifyPassword(req.Password, hash)
	if verr != nil || !ok || user == nil {
		s.log.Warn("failed login attempt", "username", req.Username, "ip", clientIP(r))
		writeError(w, http.StatusUnauthorized, "auth.invalid_credentials", "invalid username or password")
		return
	}

	token, err := auth.NewToken()
	if err != nil {
		s.internalError(w, err)
		return
	}
	csrf, err := auth.NewToken()
	if err != nil {
		s.internalError(w, err)
		return
	}
	now := time.Now()
	sess := model.Session{
		Token:     token,
		UserID:    user.ID,
		CSRFToken: csrf,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionTTL),
	}
	if err := s.store.CreateSession(sess); err != nil {
		s.internalError(w, err)
		return
	}
	http.SetCookie(w, s.sessionCookie(r, token, SessionTTL))
	writeJSON(w, http.StatusOK, sessionResponse{User: user, CSRFToken: csrf})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)
	if err := s.store.DeleteSession(sess.Token); err != nil {
		s.internalError(w, err)
		return
	}
	http.SetCookie(w, s.sessionCookie(r, "", -time.Hour))
	writeJSON(w, http.StatusOK, map[string]bool{"loggedOut": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sessionResponse{
		User:      userFrom(r),
		CSRFToken: sessionFrom(r).CSRFToken,
	})
}

func (s *Server) sessionCookie(r *http.Request, value string, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	}
}

// dummyHash is an Argon2id hash of a throwaway value, computed once at
// startup; it only serves to equalize login timing for unknown usernames.
var dummyHash = func() string {
	h, err := auth.HashPassword("helboot-timing-equalizer")
	if err != nil {
		panic(err)
	}
	return h
}()
