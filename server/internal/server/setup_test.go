package server_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSetupHappyPath exercises the install ceremony from a fresh DB:
// status reports unlocked, wrong token is rejected, the right token mints
// the first user as admin, status flips to locked, and a re-attempt with
// the same token returns 410 Gone.
func TestSetupHappyPath(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	// Pre-setup: status is unlocked.
	resp, body := request(t, "GET", base+"/v1/setup/status", nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, false, body["locked"])

	// Wrong token → 401.
	resp, _ = request(t, "POST", base+"/v1/setup/admin", map[string]any{
		"token":        "WRONG",
		"email":        "admin@test.dev",
		"password":     "passwordpassword",
		"display_name": "Admin",
	}, nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Right token → 201, role=admin.
	resp, body = request(t, "POST", base+"/v1/setup/admin", map[string]any{
		"token":        ts.setupToken,
		"email":        "admin@test.dev",
		"password":     "passwordpassword",
		"display_name": "Admin",
	}, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, body)
	require.Equal(t, true, body["is_admin"])

	// The SPA follows setup with a token login; do the same for a bearer cred.
	cred := tokenLogin(t, base, "admin@test.dev", "passwordpassword")

	// /v1/me confirms the admin role.
	resp, body = request(t, "GET", base+"/v1/me", nil, cred)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, true, body["is_admin"])

	// Status now locked.
	resp, body = request(t, "GET", base+"/v1/setup/status", nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, true, body["locked"])

	// Replaying the ceremony returns 410 even with the original token.
	resp, _ = request(t, "POST", base+"/v1/setup/admin", map[string]any{
		"token":        ts.setupToken,
		"email":        "second@test.dev",
		"password":     "passwordpassword",
		"display_name": "Second",
	}, nil)
	require.Equal(t, http.StatusGone, resp.StatusCode)
}

// TestRegisterBlockedDuringSetup asserts that /v1/auth/register is closed
// while setup is unlocked, and reopens (with role=user) once consumed.
func TestRegisterBlockedDuringSetup(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	// Pre-setup: register is gated.
	resp, body := request(t, "POST", base+"/v1/auth/register", map[string]any{
		"email":        "early@test.dev",
		"password":     "passwordpassword",
		"display_name": "Early",
	}, nil)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Equal(t, "setup_required", body["code"])

	// Consume setup via the proper path.
	resp, _ = request(t, "POST", base+"/v1/setup/admin", map[string]any{
		"token":        ts.setupToken,
		"email":        "admin@test.dev",
		"password":     "passwordpassword",
		"display_name": "Admin",
	}, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Post-setup: register works and the new user is NOT admin.
	resp, body = request(t, "POST", base+"/v1/auth/register", map[string]any{
		"email":        "second@test.dev",
		"password":     "passwordpassword",
		"display_name": "Second",
	}, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, body)
	if v, ok := body["is_admin"]; ok {
		require.Equal(t, false, v, "second user must not be admin")
	}
}
