package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger emits a structured access log per request. Designed to stay quiet
// about health checks so they don't flood the log.
func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		if path == "/healthz" || path == "/readyz" {
			return
		}
		logger.Info("http",
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("dur", time.Since(start)),
			slog.String("ip", clientIP(c.Request)),
			slog.String("req_id", c.GetString("request_id")),
		)
	}
}
