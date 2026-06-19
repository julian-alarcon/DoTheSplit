package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Logger emits a structured access log per request. Designed to stay quiet
// about health checks so they don't flood the log. The client IP is resolved
// through the shared trusted-proxy policy so forged X-Forwarded-For headers
// don't pollute the logs.
func Logger(logger *slog.Logger, ip *TrustedProxies) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			path := r.URL.Path
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if path == "/healthz" || path == "/readyz" {
				return
			}
			logger.Info("http",
				slog.String("method", r.Method),
				slog.String("path", path),
				slog.Int("status", rec.Status()),
				slog.Duration("dur", time.Since(start)),
				slog.String("ip", ip.ClientIP(r)),
				slog.String("req_id", RequestID(r.Context())),
			)
		})
	}
}
