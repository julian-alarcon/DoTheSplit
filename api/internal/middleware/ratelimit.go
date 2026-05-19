// Package middleware holds HTTP middlewares (auth, rate limiting, logging).
package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// LoginRateLimiter returns a middleware limiting auth endpoints to 10 req/min/IP.
func LoginRateLimiter() gin.HandlerFunc {
	return ipRateLimiter("auth", "10-M")
}

// SetupRateLimiter returns a middleware limiting /v1/setup/admin to 5
// req/min/IP. Tighter than the auth limiter because every successful POST
// is a one-shot install ceremony that mints the very first admin; brute
// force has 256 bits of entropy to work through, so 5 attempts/min is
// generous for a legitimate operator and effectively rate-limit-locked
// for an attacker.
func SetupRateLimiter() gin.HandlerFunc {
	return ipRateLimiter("setup", "5-M")
}

func ipRateLimiter(prefix, rateSpec string) gin.HandlerFunc {
	rate, _ := limiter.NewRateFromFormatted(rateSpec)
	lim := limiter.New(memory.NewStore(), rate)
	return func(c *gin.Context) {
		ip := clientIP(c.Request)
		ctx, err := lim.Get(c.Request.Context(), prefix+":"+ip)
		if err != nil {
			c.Next()
			return
		}
		if ctx.Reached {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    "rate_limited",
				"message": "too many requests",
			})
			return
		}
		c.Next()
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
