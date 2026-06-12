package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// bearerRequest issues an authenticated call using an Authorization: Bearer
// header instead of a cookie.
func bearerRequest(t *testing.T, method, url, accessToken string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req, err := http.NewRequest(method, url, &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	resp, err := testHTTPClient.Do(req)
	require.NoError(t, err)
	var out map[string]any
	if resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
		_ = json.NewDecoder(resp.Body).Decode(&out)
	}
	return resp, out
}

// refreshCookie extracts the dts_refresh cookie from a response, or nil.
func refreshCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == "dts_refresh" && c.Value != "" {
			return c
		}
	}
	return nil
}

// TestTokenAuthGoldenPath exercises the full bearer flow: register (so an
// account exists), exchange credentials for a token pair, use the access token
// against an authenticated endpoint, then rotate the refresh token.
func TestTokenAuthGoldenPath(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	user, _ := registerUser(t, base, "ada@test.dev", "passwordpassword", "Ada")

	// Exchange credentials for tokens.
	resp, tok := request(t, "POST", base+"/v1/auth/token", map[string]any{
		"email": "ada@test.dev", "password": "passwordpassword",
	}, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, tok)
	access, _ := tok["access_token"].(string)
	require.NotEmpty(t, access)
	require.Equal(t, "Bearer", tok["token_type"])
	require.NotZero(t, tok["expires_in"])
	rc := refreshCookie(resp)
	require.NotNil(t, rc, "refresh cookie must be set")
	require.True(t, rc.HttpOnly)

	// Bearer token authenticates /me.
	resp, me := bearerRequest(t, "GET", base+"/v1/me", access, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, me)
	require.Equal(t, user["id"], me["id"])

	// A garbage bearer token is rejected as unauthenticated.
	resp, _ = bearerRequest(t, "GET", base+"/v1/me", "not-a-jwt", nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Refresh via the cookie rotates and returns a new access token.
	resp2, refreshed := request(t, "POST", base+"/v1/auth/refresh", nil, rc)
	require.Equal(t, http.StatusOK, resp2.StatusCode, refreshed)
	access2, _ := refreshed["access_token"].(string)
	require.NotEmpty(t, access2)
	rc2 := refreshCookie(resp2)
	require.NotNil(t, rc2)
	require.NotEqual(t, rc.Value, rc2.Value, "refresh token must rotate")

	// The new access token still works.
	resp, _ = bearerRequest(t, "GET", base+"/v1/me", access2, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestRefreshReuseRevokesChain asserts that presenting an already-rotated
// refresh token (theft signal) revokes the whole chain: the rotated successor
// stops working too.
func TestRefreshReuseRevokesChain(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL
	registerUser(t, base, "eve@test.dev", "passwordpassword", "Eve")

	resp, _ := request(t, "POST", base+"/v1/auth/token", map[string]any{
		"email": "eve@test.dev", "password": "passwordpassword",
	}, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	rc := refreshCookie(resp)
	require.NotNil(t, rc)

	// First refresh rotates rc -> rc2.
	resp2, _ := request(t, "POST", base+"/v1/auth/refresh", nil, rc)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	rc2 := refreshCookie(resp2)
	require.NotNil(t, rc2)

	// Replaying the original (now-rotated) rc is reuse -> 401 and chain kill.
	resp3, _ := request(t, "POST", base+"/v1/auth/refresh", nil, rc)
	require.Equal(t, http.StatusUnauthorized, resp3.StatusCode)

	// The legitimate successor rc2 is now revoked as collateral.
	resp4, _ := request(t, "POST", base+"/v1/auth/refresh", nil, rc2)
	require.Equal(t, http.StatusUnauthorized, resp4.StatusCode)
}

// TestRevokeToken logs a token client out: the refresh token stops working.
func TestRevokeToken(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL
	registerUser(t, base, "mal@test.dev", "passwordpassword", "Mal")

	resp, _ := request(t, "POST", base+"/v1/auth/token", map[string]any{
		"email": "mal@test.dev", "password": "passwordpassword",
	}, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	rc := refreshCookie(resp)
	require.NotNil(t, rc)

	resp2, _ := request(t, "POST", base+"/v1/auth/token/revoke", nil, rc)
	require.Equal(t, http.StatusNoContent, resp2.StatusCode)

	resp3, _ := request(t, "POST", base+"/v1/auth/refresh", nil, rc)
	require.Equal(t, http.StatusUnauthorized, resp3.StatusCode)
}

// TestTokenWrongPassword rejects bad credentials at the token endpoint.
func TestTokenWrongPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL
	registerUser(t, base, "ada@test.dev", "passwordpassword", "Ada")

	resp, _ := request(t, "POST", base+"/v1/auth/token", map[string]any{
		"email": "ada@test.dev", "password": "wrongwrongwrong",
	}, nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
