package handlers

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/julian-alarcon/dothesplit/api/internal/webui"
)

// spaFS is the embedded SPA filesystem, loaded once at startup.
type spaServer struct {
	files     fs.FS
	indexHTML []byte
	cookieSec bool
}

// NewSPAHandler builds the catch-all handler that serves the embedded Vue SPA.
// Static asset requests are served directly with long-lived caching (Vite
// hashes filenames); everything else falls back to index.html so the client
// router can take over (deep links, reloads). The Go API's /v1, /healthz, and
// /readyz routes are registered before this catch-all, so they win.
func (s *Server) NewSPAHandler() (gin.HandlerFunc, error) {
	files, err := webui.FS()
	if err != nil {
		return nil, err
	}
	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		// No built bundle in this binary (e.g. a Go-only `go build` without
		// `make embed-frontend`). Serve a helpful placeholder rather than failing
		// to boot, so the API is still usable for development.
		index = []byte(webui.FallbackHTML)
	}
	srv := &spaServer{files: files, indexHTML: index, cookieSec: s.Cfg.CookieSecure}
	return srv.handle, nil
}

// apiPrefixes are reserved for the Go API surface. An unmatched request under
// these never falls back to the SPA shell; it is a genuine 404 so clients get
// a clear error instead of an HTML page where they expected JSON.
var apiPrefixes = []string{"/v1/", "/healthz", "/readyz"}

func (srv *spaServer) handle(c *gin.Context) {
	urlPath := c.Request.URL.Path
	for _, p := range apiPrefixes {
		if urlPath == strings.TrimSuffix(p, "/") || strings.HasPrefix(urlPath, p) {
			writeErr(c, http.StatusNotFound, "not_found", "no such endpoint")
			return
		}
	}

	// Only GET/HEAD reach the SPA. A non-GET to an unknown path is a 404 from
	// the API surface, not an SPA route.
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.Status(http.StatusNotFound)
		return
	}

	reqPath := strings.TrimPrefix(path.Clean(urlPath), "/")
	if reqPath == "" {
		srv.serveIndex(c)
		return
	}

	f, err := srv.files.Open(reqPath)
	if err != nil {
		// Unknown path with a file extension is a genuine 404 (missing asset);
		// an extensionless path is a client route → serve the shell.
		if path.Ext(reqPath) != "" {
			c.Status(http.StatusNotFound)
			return
		}
		srv.serveIndex(c)
		return
	}
	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		srv.serveIndex(c)
		return
	}
	seeker, ok := f.(io.ReadSeeker)
	if !ok {
		srv.serveIndex(c)
		return
	}

	// Hashed assets under assets/ are immutable; everything else is revalidated.
	if strings.HasPrefix(reqPath, "assets/") {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "no-cache")
	}
	srv.securityHeaders(c)
	http.ServeContent(c.Writer, c.Request, stat.Name(), stat.ModTime(), seeker)
}

func (srv *spaServer) serveIndex(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	srv.securityHeaders(c)
	c.Data(http.StatusOK, "text/html; charset=utf-8", srv.indexHTML)
}

// securityHeaders applies a strict CSP suited to a CSR SPA: no inline scripts
// (Vite emits external modules), connect-back to same origin only. HSTS is
// added only over HTTPS.
func (srv *spaServer) securityHeaders(c *gin.Context) {
	c.Header("Content-Security-Policy", strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"connect-src 'self'",
		"font-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; "))
	if srv.cookieSec {
		c.Header("Strict-Transport-Security", "max-age=31536000")
	}
}
