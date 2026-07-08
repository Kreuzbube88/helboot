package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestUserManagement(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin() // admin

	// Create an operator.
	resp, body := c.do(http.MethodPost, "/api/v1/users", map[string]string{
		"username": "opuser", "password": "operator-pass-1", "role": "operator", "locale": "de",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create user: %d %s", resp.StatusCode, body)
	}
	var created struct {
		ID   int64  `json:"id"`
		Role string `json:"role"`
	}
	json.Unmarshal(body, &created)
	if created.Role != "operator" {
		t.Errorf("role = %q", created.Role)
	}

	// Weak password and bad role rejected.
	resp, _ = c.do(http.MethodPost, "/api/v1/users", map[string]string{
		"username": "weak", "password": "short", "role": "viewer",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("weak password accepted: %d", resp.StatusCode)
	}
	resp, _ = c.do(http.MethodPost, "/api/v1/users", map[string]string{
		"username": "badrole", "password": "long-enough-pass", "role": "root",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid role accepted: %d", resp.StatusCode)
	}

	// Duplicate username conflicts.
	resp, _ = c.do(http.MethodPost, "/api/v1/users", map[string]string{
		"username": "opuser", "password": "operator-pass-1", "role": "viewer",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate username: %d, want 409", resp.StatusCode)
	}

	// List shows both accounts.
	resp, body = c.do(http.MethodGet, "/api/v1/users", nil)
	var users []struct {
		Username string `json:"username"`
	}
	json.Unmarshal(body, &users)
	if resp.StatusCode != http.StatusOK || len(users) != 2 {
		t.Errorf("list users: %d entries (want 2)", len(users))
	}
}

func TestLastAdminIsProtected(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin() // admin is user 1

	// Demoting the only admin must fail.
	resp, _ := c.do(http.MethodPut, "/api/v1/users/1", map[string]string{"role": "viewer"})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("last admin demotion: %d, want 409", resp.StatusCode)
	}
	// Self-deletion must fail.
	resp, _ = c.do(http.MethodDelete, "/api/v1/users/1", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("self deletion: %d, want 409", resp.StatusCode)
	}

	// With a second admin, demotion works.
	c.do(http.MethodPost, "/api/v1/users", map[string]string{
		"username": "admin2", "password": "second-admin-pw", "role": "admin",
	})
	resp, _ = c.do(http.MethodPut, "/api/v1/users/1", map[string]string{"role": "operator"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("demotion with second admin: %d, want 200", resp.StatusCode)
	}
}

func TestOperatorCannotManageUsers(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin()
	c.do(http.MethodPost, "/api/v1/users", map[string]string{
		"username": "opuser", "password": "operator-pass-1", "role": "operator",
	})

	// Log in as the operator.
	op := newClient(t, ts)
	resp, body := op.do(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "opuser", "password": "operator-pass-1",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("operator login: %d %s", resp.StatusCode, body)
	}
	var sess struct {
		CSRFToken string `json:"csrfToken"`
	}
	json.Unmarshal(body, &sess)
	op.csrf = sess.CSRFToken

	resp, _ = op.do(http.MethodGet, "/api/v1/users", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("operator listed users: %d, want 403", resp.StatusCode)
	}
	// But operators can manage hosts.
	resp, _ = op.do(http.MethodPost, "/api/v1/hosts", map[string]any{"mac": "aa:bb:cc:dd:ee:30"})
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("operator host create: %d, want 201", resp.StatusCode)
	}
}

func TestChangeOwnPasswordRevokesSessions(t *testing.T) {
	ts := testServer(t)
	c := newClient(t, ts)
	c.setupAndLogin()

	// Wrong current password rejected.
	resp, _ := c.do(http.MethodPost, "/api/v1/auth/password", map[string]string{
		"currentPassword": "wrong", "newPassword": "brand-new-password",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("wrong current password: %d, want 403", resp.StatusCode)
	}

	resp, _ = c.do(http.MethodPost, "/api/v1/auth/password", map[string]string{
		"currentPassword": "super-secret-pw", "newPassword": "brand-new-password",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("change password: %d", resp.StatusCode)
	}

	// Old session is gone.
	resp, _ = c.do(http.MethodGet, "/api/v1/auth/me", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("old session still valid after password change: %d", resp.StatusCode)
	}

	// New password works.
	resp, _ = c.do(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "brand-new-password",
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("login with new password: %d", resp.StatusCode)
	}
}
