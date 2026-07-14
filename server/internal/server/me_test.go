package server_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/julian-alarcon/dothesplit/server/internal/service"
	"github.com/stretchr/testify/require"
)

// makeClientSizedPNG returns a fresh RGBA PNG of the avatar pipeline's client
// side length, filled with the given uniform color, base64-encoded.
func makeClientSizedPNG(t *testing.T, r, g, b uint8) string {
	t.Helper()
	n := service.AvatarClientSize
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			img.Set(x, y, color.RGBA{r, g, b, 0xff})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func makeWrongSizePNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestMeFlows(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	_, cookieA := registerUser(t, base, "me-a@test.dev", "passwordpassword", "Alice")

	// --- Rename ---
	resp, _ := request(t, "PATCH", base+"/v1/me", map[string]any{
		"display_name": "Alice Renamed",
	}, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_, me := request(t, "GET", base+"/v1/me", nil, cookieA)
	require.Equal(t, "Alice Renamed", me["display_name"])

	// --- Change password ---
	// Wrong old password → 401
	resp, _ = request(t, "POST", base+"/v1/me/password", map[string]any{
		"old_password": "wrongwrongwrong",
		"new_password": "newpasswordlong",
	}, cookieA)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Right old password → 200 with a fresh token pair in the body. The change
	// revokes every refresh-token chain and mints a new one so the caller stays
	// logged in; we capture the new access token for the rest of the test.
	resp, pwOut := request(t, "POST", base+"/v1/me/password", map[string]any{
		"old_password": "passwordpassword",
		"new_password": "newpasswordlong",
	}, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	newAccess, _ := pwOut["access_token"].(string)
	require.NotEmpty(t, newAccess, "change-password must return a fresh access token")
	newCookie := bearerCred(newAccess)

	// The fresh access token works.
	resp, _ = request(t, "GET", base+"/v1/me", nil, newCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// The old password no longer issues tokens; the new one does.
	resp, _ = request(t, "POST", base+"/v1/auth/token", map[string]any{
		"email": "me-a@test.dev", "password": "passwordpassword",
	}, nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp, _ = request(t, "POST", base+"/v1/auth/token", map[string]any{
		"email": "me-a@test.dev", "password": "newpasswordlong",
	}, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// --- Avatar ---
	_, me = request(t, "GET", base+"/v1/me", nil, newCookie)
	require.False(t, me["has_avatar"].(bool))

	// Bad: non-PNG.
	resp, _ = request(t, "PUT", base+"/v1/me/avatar", map[string]any{
		"png_base64": base64.StdEncoding.EncodeToString([]byte("not a png")),
	}, newCookie)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Bad: PNG of the wrong size.
	resp, _ = request(t, "PUT", base+"/v1/me/avatar", map[string]any{
		"png_base64": makeWrongSizePNG(t, 16, 16),
	}, newCookie)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Bad: oversized blob (lots of base64 padding → decoded body > 1024 bytes).
	resp, _ = request(t, "PUT", base+"/v1/me/avatar", map[string]any{
		"png_base64": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x00}, 1100)),
	}, newCookie)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Good: proper client-sized PNG.
	goodPNG := makeClientSizedPNG(t, 0xff, 0x00, 0x66)
	resp, _ = request(t, "PUT", base+"/v1/me/avatar", map[string]any{
		"png_base64": goodPNG,
	}, newCookie)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	_, me = request(t, "GET", base+"/v1/me", nil, newCookie)
	require.True(t, me["has_avatar"].(bool))
	require.NotEmpty(t, me["avatar_updated_at"])

	// Download own avatar. The server upscales the validated client-sized PNG
	// to AvatarRenderSize square via nearest-neighbour before storing it, so
	// the served bitmap should be large and crisp regardless of any CSS
	// rendering hints on the client.
	myID := me["id"].(string)
	resp, body := rawRequest(t, "GET", base+"/v1/users/"+myID+"/avatar", nil, newCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "image/png", resp.Header.Get("Content-Type"))
	servedImg, err := png.Decode(bytes.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, service.AvatarRenderSize, servedImg.Bounds().Dx())
	require.Equal(t, service.AvatarRenderSize, servedImg.Bounds().Dy())

	// A stranger (not in any shared group) cannot fetch the avatar.
	_, cookieStranger := registerUser(t, base, "stranger@test.dev", "passwordpassword", "Stranger")
	resp, _ = rawRequest(t, "GET", base+"/v1/users/"+myID+"/avatar", nil, cookieStranger)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Add stranger to a shared group → avatar becomes visible.
	_, groupBody := request(t, "POST", base+"/v1/groups",
		map[string]any{"name": "Shared"}, newCookie)
	groupID := groupBody["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/members",
		map[string]any{"email": "stranger@test.dev"}, newCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp, _ = rawRequest(t, "GET", base+"/v1/users/"+myID+"/avatar", nil, cookieStranger)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Clear avatar.
	resp, _ = request(t, "DELETE", base+"/v1/me/avatar", nil, newCookie)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	_, me = request(t, "GET", base+"/v1/me", nil, newCookie)
	require.False(t, me["has_avatar"].(bool))
	resp, _ = rawRequest(t, "GET", base+"/v1/users/"+myID+"/avatar", nil, newCookie)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteMe(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	userA, cookieA := registerUser(t, base, "delete-me@test.dev", "passwordpassword", "Deleter")
	userAID := userA["id"].(string)

	// Create a group + expense so we can verify the tombstone on historical data.
	_, groupBody := request(t, "POST", base+"/v1/groups",
		map[string]any{"name": "Ledger"}, cookieA)
	groupID := groupBody["id"].(string)
	_, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/expenses", map[string]any{
		"description":  "Paid by future-deleted",
		"amount_cents": 1000,
		"payer_id":     userAID,
		"mode":         "equal",
		"splits":       []map[string]any{{"user_id": userAID}},
	}, cookieA)

	// Wrong password is rejected and the account stays alive.
	resp, _ := request(t, "DELETE", base+"/v1/me",
		map[string]any{"password": "not-the-real-one"}, cookieA)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp, _ = request(t, "GET", base+"/v1/me", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Delete the account with the correct password.
	resp, _ = request(t, "DELETE", base+"/v1/me",
		map[string]any{"password": "passwordpassword"}, cookieA)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// The account is tombstoned: the bearer token resolves the user, sees
	// deleted_at, and /me now returns 401.
	resp, _ = request(t, "GET", base+"/v1/me", nil, cookieA)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Token login with the old credentials no longer works.
	resp, _ = request(t, "POST", base+"/v1/auth/token", map[string]any{
		"email": "delete-me@test.dev", "password": "passwordpassword",
	}, nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Re-registering with the same email works (the partial unique index
	// ignores soft-deleted rows).
	userB, _ := registerUser(t, base, "delete-me@test.dev", "passwordpassword", "Revival")
	require.NotEqual(t, userAID, userB["id"].(string))

	// The tombstone display_name must reference the deleted user's UUID.
	tombstone, err := ts.raw().UserDisplayName(t.Context(), userAID)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(tombstone, "Deleted user #"), "got %q", tombstone)
	require.Contains(t, tombstone, strings.Split(userAID, "-")[0])
}
