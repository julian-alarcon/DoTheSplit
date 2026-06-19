// Package middleware holds HTTP middlewares (auth, rate limiting, logging).
package middleware

import (
	"net/http"
	"sync"
	"time"
)

// fixedWindowLimiter is a small in-memory per-key fixed-window rate limiter. It
// replaces the external ulule/limiter dependency: at this scale (human-paced
// auth/setup traffic on a LAN) a per-minute counter map is plenty, and it keeps
// the API on the standard library. Buckets are lazily pruned when their window
// has rolled over, so memory stays bounded by the number of recently-active IPs.
type fixedWindowLimiter struct {
	perMin int
	mu     sync.Mutex
	hits   map[string]*window
}

type window struct {
	start time.Time
	count int
}

func newFixedWindowLimiter(perMin int) *fixedWindowLimiter {
	return &fixedWindowLimiter{perMin: perMin, hits: map[string]*window{}}
}

// allow records a hit for key and reports whether it is within the limit. The
// limit is inclusive: with perMin=10 the 11th hit inside a minute is rejected,
// matching the previous ulule "10-M" behavior.
func (l *fixedWindowLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.hits[key]
	if w == nil || now.Sub(w.start) >= time.Minute {
		l.hits[key] = &window{start: now, count: 1}
		return true
	}
	w.count++
	return w.count <= l.perMin
}

// LoginRateLimiter returns a middleware limiting auth endpoints to perMin
// req/min/IP. perMin <= 0 falls back to the production default of 10.
func LoginRateLimiter(perMin int, ip *TrustedProxies) Middleware {
	if perMin <= 0 {
		perMin = 10
	}
	return ipRateLimiter("auth", perMin, ip)
}

// SetupRateLimiter returns a middleware limiting /v1/setup/admin to 5
// req/min/IP. Tighter than the auth limiter because every successful POST
// is a one-shot install ceremony that mints the very first admin; brute
// force has 256 bits of entropy to work through, so 5 attempts/min is
// generous for a legitimate operator and effectively rate-limit-locked
// for an attacker.
func SetupRateLimiter(ip *TrustedProxies) Middleware {
	return ipRateLimiter("setup", 5, ip)
}

func ipRateLimiter(prefix string, perMin int, ip *TrustedProxies) Middleware {
	lim := newFixedWindowLimiter(perMin)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := prefix + ":" + ip.ClientIP(r)
			if !lim.allow(key, time.Now()) {
				writeJSONError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
