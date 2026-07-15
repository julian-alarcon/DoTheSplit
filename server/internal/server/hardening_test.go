package server_test

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRateLimitIgnoresForgedXFF proves the auth limiter can't be bypassed by
// rotating X-Forwarded-For. The test server configures no trusted proxies, so
// c.ClientIP() ignores the header and keys every request on the real
// (loopback) RemoteAddr. The 10-per-minute bucket must therefore trip even
// though each request carries a distinct forged IP.
func TestRateLimitIgnoresForgedXFF(t *testing.T) {
	ts := setup(t, withAuthRateLimit(10))
	base := ts.srv.URL

	var got429 bool
	for i := 0; i < 12; i++ {
		req, err := http.NewRequest(http.MethodPost, base+"/v1/auth/token",
			strings.NewReader(`{"email":"nobody@test.dev","password":"wrongpassword"}`))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
		resp, err := testHTTPClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	require.True(t, got429, "rate limiter should trip despite rotating X-Forwarded-For")
}

// TestRequestBodyTooLarge proves the global MaxBodyBytes cap rejects an
// oversized request body with a 400 rather than buffering it into memory.
func TestRequestBodyTooLarge(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	// 2 MiB body, well over the 1 MiB cap.
	huge := bytes.Repeat([]byte("a"), 2<<20)
	body := []byte(`{"email":"big@test.dev","password":"` + string(huge) + `"}`)

	req, err := http.NewRequest(http.MethodPost, base+"/v1/auth/token", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testHTTPClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
