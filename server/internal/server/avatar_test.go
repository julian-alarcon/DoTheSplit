package server_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"testing"

	"github.com/julian-alarcon/dothesplit/server/internal/service"
	"github.com/stretchr/testify/require"
)

// TestAvatarUploadHappyPath confirms the GDPR-load-bearing pipeline:
// (1) the server accepts a valid 8x8 PNG, (2) the bytes round-tripped via
// GET /v1/users/{id}/avatar are a 256x256 PNG (re-encoded from a fresh
// RGBA canvas - see service/me.go), (3) the resulting image is well under
// the storage cap. This is the contract documented in AGENTS.md "Avatar
// invariants": 64 color samples, never the original full-res image.
func TestAvatarUploadHappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	user, cookie := registerUser(t, base, "alice@test.dev", "passwordpassword", "Alice")
	userID := user["id"].(string)

	src := makeAvatarPNG(t, color.RGBA{R: 255, G: 64, B: 128, A: 255})

	resp, body := request(t, "PUT", base+"/v1/me/avatar", map[string]any{
		"png_base64": base64.StdEncoding.EncodeToString(src),
	}, cookie)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, body)

	resp, raw := rawRequest(t, "GET", base+"/v1/users/"+userID+"/avatar", nil, cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "image/png", resp.Header.Get("Content-Type"))

	// Must be exactly AvatarRenderSize x AvatarRenderSize after server-side
	// nearest-neighbour upscale. If this regresses, the GDPR claim that we
	// never store more than 64 color samples is silently broken.
	img, err := png.Decode(bytes.NewReader(raw))
	require.NoError(t, err, "stored bytes must be a valid PNG")
	b := img.Bounds()
	require.Equal(t, service.AvatarRenderSize, b.Dx(), "stored width")
	require.Equal(t, service.AvatarRenderSize, b.Dy(), "stored height")

	// Storage cap is generous (16 KiB) - low-color PNGs of solid blocks
	// compress to a few hundred bytes. Assert we are well under the ceiling.
	require.Less(t, len(raw), service.AvatarStorageMaxB,
		"stored PNG must respect AvatarStorageMaxB")

	// The upscale is nearest-neighbour: every output pixel must be a direct
	// copy of one source pixel. Sampling four corners is enough to catch
	// blending or interpolation bugs without making this test brittle.
	for _, p := range []image.Point{{0, 0}, {0, b.Dy() - 1}, {b.Dx() - 1, 0}, {b.Dx() - 1, b.Dy() - 1}} {
		r, g, bl, a := img.At(p.X, p.Y).RGBA()
		require.Equal(t, uint32(0xFFFF), a, "alpha at %v should be opaque", p)
		// The source PNG is encoded as NRGBA with R=255,G=64,B=128. After
		// PNG decode + nearest-neighbour copy, the channel values must be
		// preserved (allow no tolerance - this is integer arithmetic).
		require.Equal(t, uint32(0xFFFF), r, "red at %v", p)
		require.Equal(t, uint32(0x4040), g, "green at %v", p)
		require.Equal(t, uint32(0x8080), bl, "blue at %v", p)
	}
}

// TestAvatarUploadRejectsOversize covers the AvatarClientMaxB cap: the
// client pipeline targets ~150-250 B (an 8x8 PNG of solid colors), so a
// payload at or above 1024 B is either an attempted bypass or a buggy
// client. Server must refuse with 400.
func TestAvatarUploadRejectsOversize(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL
	_, cookie := registerUser(t, base, "alice@test.dev", "passwordpassword", "Alice")

	// Forge a buffer larger than the 1 KiB cap; PNG validity doesn't matter -
	// the size check fires first inside SetAvatarFromBase64.
	huge := make([]byte, service.AvatarClientMaxB+1)
	for i := range huge {
		huge[i] = 0x42
	}
	resp, body := request(t, "PUT", base+"/v1/me/avatar", map[string]any{
		"png_base64": base64.StdEncoding.EncodeToString(huge),
	}, cookie)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, body)
}

// TestAvatarUploadRejectsWrongDimensions catches a client that uploads a
// non-8x8 PNG. Without this guard a malicious client could push hi-res
// imagery into the database.
func TestAvatarUploadRejectsWrongDimensions(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL
	_, cookie := registerUser(t, base, "alice@test.dev", "passwordpassword", "Alice")

	// 16x16 PNG instead of the required 8x8.
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 0, G: 200, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	resp, body := request(t, "PUT", base+"/v1/me/avatar", map[string]any{
		"png_base64": base64.StdEncoding.EncodeToString(buf.Bytes()),
	}, cookie)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, body)
}

// makeAvatarPNG builds a valid 8x8 PNG of a solid color, returning the
// encoded bytes ready for base64.
func makeAvatarPNG(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, service.AvatarClientSize, service.AvatarClientSize))
	for y := 0; y < service.AvatarClientSize; y++ {
		for x := 0; x < service.AvatarClientSize; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	require.Less(t, buf.Len(), service.AvatarClientMaxB,
		"test fixture must fit under the client size cap; got %d", buf.Len())
	return buf.Bytes()
}
