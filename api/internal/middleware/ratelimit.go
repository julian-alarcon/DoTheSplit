// Package middleware holds HTTP middlewares (auth, rate limiting, logging).
package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// LoginRateLimiter returns a middleware limiting auth endpoints to perMin
// req/min/IP. perMin <= 0 falls back to the production default of 10.
func LoginRateLimiter(perMin int) gin.HandlerFunc {
	if perMin <= 0 {
		perMin = 10
	}
	return ipRateLimiter("auth", fmt.Sprintf("%d-M", perMin))
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
		ip := c.ClientIP()
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
