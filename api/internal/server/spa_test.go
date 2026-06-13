package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSPAFallbackServesShell asserts that unknown extensionless paths fall
// back to the embedded SPA shell (client-side routing), while /v1 routes are
// untouched by the catch-all and a missing asset is a real 404.
func TestSPAFallbackServesShell(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	// Deep link to a client route returns the HTML shell with a strict CSP.
	resp, body := rawRequest(t, "GET", base+"/groups/123", nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	require.Contains(t, resp.Header.Get("Content-Security-Policy"), "script-src 'self'")
	require.NotContains(t, resp.Header.Get("Content-Security-Policy"), "'unsafe-inline' 'self' http")
	require.True(t, strings.Contains(string(body), "<html") || strings.Contains(string(body), "<!doctype"))

	// The root path also serves the shell.
	resp, _ = rawRequest(t, "GET", base+"/", nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// A missing hashed asset is a genuine 404, not a shell fallback.
	resp, _ = rawRequest(t, "GET", base+"/assets/does-not-exist.js", nil, nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	// An unknown /v1 path is still handled by the API (JSON 404-ish), never the
	// SPA shell.
	resp, _ = rawRequest(t, "GET", base+"/v1/nope", nil, nil)
	require.NotEqual(t, http.StatusOK, resp.StatusCode)
	require.NotContains(t, resp.Header.Get("Content-Type"), "text/html")
}
