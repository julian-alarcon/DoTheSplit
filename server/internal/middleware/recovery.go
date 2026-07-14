package middleware

import (
	"log/slog"
	"net/http"
)

// Recovery replaces gin.Recovery(): it catches a panic from any downstream
// handler, logs it, and emits a 500 JSON error (best-effort, in case bytes were
// already written). Install it outermost so it covers the whole chain.
func Recovery(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						slog.Any("err", rec),
						slog.String("path", r.URL.Path),
					)
					writeJSONError(w, http.StatusInternalServerError, "internal", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
