// Package webui embeds the built Vue SPA (Vite output) and serves it as the
// application's static frontend. The production build copies the SPA's dist/
// into this package directory before `go build`, so the whole frontend ships
// inside the single Go binary - no Node runtime, no separate web container.
package webui

import (
	"embed"
	"io/fs"
)

// dist holds the built SPA. A committed .gitkeep keeps `go:embed all:dist`
// valid in a checkout that hasn't run the web build yet; the real assets
// (gitignored) are copied in at image-build time by `make embed-frontend`.
//
//go:embed all:dist
var dist embed.FS

// FallbackHTML is served when the built SPA is absent (a binary compiled
// without running the web build). Kept in code so the handler always has an
// index.html to fall back to.
const FallbackHTML = `<!doctype html>
<html lang="en"><head><meta charset="UTF-8"><title>DoTheSplit</title></head>
<body><p>Frontend bundle not built. Run <code>make embed-frontend</code> before building the API binary.</p></body></html>`

// FS returns the embedded SPA filesystem rooted at dist/ (so paths look like
// "index.html", "assets/...").
func FS() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}
