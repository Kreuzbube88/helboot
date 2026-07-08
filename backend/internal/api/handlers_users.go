package api

import (
	"fmt"
	"net/http"

	"github.com/kreuzbube88/helboot/backend/internal/auth"
	"github.com/kreuzbube88/helboot/backend/internal/model"
)

const minPasswordLength = 10

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Locale   string `json:"locale"`
}

type updateUserRequest struct {
	Role string `json:"role"`
}

type setPasswordRequest struct {
	Password string `json:"password"`
}

type changeOwnPasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, _ *http.Request) {
	users, err := s.store.ListUsers()
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Username) < 3 {
		writeError(w, http.StatusBadRequest, "user.invalid_username", "username must have at least 3 characters")
		return
	}
	if len(req.Password) < minPasswordLength {
		writeError(w, http.StatusBadRequest, "user.weak_password",
			fmt.Sprintf("password must have at least %d characters", minPasswordLength))
		return
	}
	role := model.Role(req.Role)
	if !role.Valid() {
		writeError(w, http.StatusBadRequest, "user.invalid_role", "role must be one of: admin, operator, viewer")
		return
	}
	locale := req.Locale
	if locale == "" {
		locale = "en"
	}
	if _, err := s.store.UserByUsername(req.Username); err == nil {
		writeError(w, http.StatusConflict, "user.exists", "a user with this name already exists")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.internalError(w, err)
		return
	}
	user, err := s.store.CreateUser(req.Username, hash, role, locale)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r, "user.create", "user", user.ID)
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	target, err := s.store.UserByID(id)
	if err != nil {
		s.storeError(w, err)
		return
	}
	var req updateUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	role := model.Role(req.Role)
	if !role.Valid() {
		writeError(w, http.StatusBadRequest, "user.invalid_role", "role must be one of: admin, operator, viewer")
		return
	}
	// The last administrator can never be demoted (§26: the admin role
	// must always exist).
	if target.Role == model.RoleAdmin && role != model.RoleAdmin {
		if s.isLastAdmin(w) {
			return
		}
	}
	if err := s.store.UpdateUserRole(id, role); err != nil {
		s.storeError(w, err)
		return
	}
	s.audit(r, "user.update_role", "user", id)
	updated, err := s.store.UserByID(id)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// handleSetUserPassword lets an administrator reset any user's password.
func (s *Server) handleSetUserPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.store.UserByID(id); err != nil {
		s.storeError(w, err)
		return
	}
	var req setPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Password) < minPasswordLength {
		writeError(w, http.StatusBadRequest, "user.weak_password",
			fmt.Sprintf("password must have at least %d characters", minPasswordLength))
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if err := s.store.UpdateUserPassword(id, hash); err != nil {
		s.storeError(w, err)
		return
	}
	s.audit(r, "user.reset_password", "user", id)
	writeJSON(w, http.StatusOK, map[string]bool{"updated": true})
}

// handleChangeOwnPassword lets any signed-in user rotate their own
// password after proving they know the current one. All sessions are
// revoked, including the caller's — they must sign in again.
func (s *Server) handleChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r)
	var req changeOwnPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	ok, err := auth.VerifyPassword(req.CurrentPassword, user.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusForbidden, "user.wrong_password", "current password is incorrect")
		return
	}
	if len(req.NewPassword) < minPasswordLength {
		writeError(w, http.StatusBadRequest, "user.weak_password",
			fmt.Sprintf("password must have at least %d characters", minPasswordLength))
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if err := s.store.UpdateUserPassword(user.ID, hash); err != nil {
		s.storeError(w, err)
		return
	}
	s.audit(r, "user.change_password", "user", user.ID)
	http.SetCookie(w, s.sessionCookie(r, "", -1))
	writeJSON(w, http.StatusOK, map[string]bool{"updated": true, "reloginRequired": true})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	caller := userFrom(r)
	if caller.ID == id {
		writeError(w, http.StatusConflict, "user.cannot_delete_self", "you cannot delete your own account")
		return
	}
	target, err := s.store.UserByID(id)
	if err != nil {
		s.storeError(w, err)
		return
	}
	if target.Role == model.RoleAdmin && s.isLastAdmin(w) {
		return
	}
	if err := s.store.DeleteUser(id); err != nil {
		s.storeError(w, err)
		return
	}
	s.audit(r, "user.delete", "user", id)
	w.WriteHeader(http.StatusNoContent)
}

// isLastAdmin writes the conflict response and returns true when only
// one administrator exists.
func (s *Server) isLastAdmin(w http.ResponseWriter) bool {
	n, err := s.store.CountAdmins()
	if err != nil {
		s.internalError(w, err)
		return true
	}
	if n <= 1 {
		writeError(w, http.StatusConflict, "user.last_admin", "the last administrator cannot be removed or demoted")
		return true
	}
	return false
}

// audit records a privileged action; failures are logged, never fatal.
func (s *Server) audit(r *http.Request, action, entity string, entityID int64) {
	var userID *int64
	if u := userFrom(r); u != nil {
		userID = &u.ID
	}
	if err := s.store.AddAudit(userID, action, entity, fmt.Sprintf("%d", entityID)); err != nil {
		s.log.Error("audit write failed", "action", action, "error", err)
	}
}
