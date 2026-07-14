package middleware

import (
	"net/http"
	"strings"
)

// DefaultMaxBodyBytes caps request bodies at 1 MiB. The largest legitimate
// body is a 256 KiB CSV base64-wrapped in JSON (~360 KiB) plus a small members
// array, so 1 MiB is comfortable headroom while still blocking a memory-DoS
// from an unbounded POST.
const DefaultMaxBodyBytes int64 = 1 << 20

// MaxBodyBytes caps the size of every request body. Over-limit reads fail in
// the handler's JSON decoder (or body read), which the existing bad-request
// path already translates to a 400.
func MaxBodyBytes(limit int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets conservative browser-protection headers on every response.
// HSTS is only emitted when cookieSecure is true (otherwise it's useless under HTTP).
func SecurityHeaders(cookieSecure bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("X-Frame-Options", "DENY")
			if cookieSecure {
				h.Set("Strict-Transport-Security", "max-age=31536000")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS allows requests from the configured origins with credentials. Each
// entry may itself be comma-separated; native Capacitor origins
// (capacitor://localhost, https://localhost) are passed in alongside the web
// origin. Authorization is allowed so bearer-token clients can send the header.
func CORS(allowedOrigins ...string) Middleware {
	allowed := map[string]bool{}
	for _, group := range allowedOrigins {
		for _, o := range strings.Split(group, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				allowed[o] = true
			}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowed[origin] {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Vary", "Origin")
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				h.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
