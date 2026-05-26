package middleware_test

import (
	"testing"

	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/stretchr/testify/require"
)

// TestSessionCookieName pins the contract documented in AGENTS.md
// "Cookie naming": the __Host- prefix is browser-rejected without
// Secure=true, so HTTPS deployments use it and plain-HTTP LAN deployments
// don't. The frontend's middleware substring-matches "dts_session=" to
// cover both - if anyone renames either variant, the web bridge breaks.
func TestSessionCookieName(t *testing.T) {
	require.Equal(t, "__Host-dts_session", middleware.SessionCookieName(true),
		"HTTPS must use the __Host- prefix")
	require.Equal(t, "dts_session", middleware.SessionCookieName(false),
		"plain HTTP must drop the prefix or browsers reject the cookie")
}
