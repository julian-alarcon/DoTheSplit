package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// ctxUserKey is the gin.Context key where the authenticated user is stored.
const ctxUserKey = "dts_user"

// SessionCookieName returns the cookie name that carries the session token.
// The __Host- prefix requires Secure + no Domain, so browsers silently drop it
// over plain HTTP. Fall back to a plain name when the deployment is not HTTPS.
func SessionCookieName(secure bool) string {
	if secure {
		return "__Host-dts_session"
	}
	return "dts_session"
}

// Resolver is the subset of AuthService that session middleware depends on.
type Resolver interface {
	Resolve(ctx context.Context, token string) (*service.User, error)
}

// Session populates the request with the authenticated user if the session
// cookie is present and valid. It does NOT require a session.
func Session(r Resolver, cookieSecure bool) gin.HandlerFunc {
	name := SessionCookieName(cookieSecure)
	return func(c *gin.Context) {
		cookie, err := c.Cookie(name)
		if err != nil || cookie == "" {
			c.Next()
			return
		}
		u, err := r.Resolve(c.Request.Context(), cookie)
		if err != nil {
			c.Next()
			return
		}
		c.Set(ctxUserKey, u)
		c.Next()
	}
}

// RequireSession aborts with 401 if no user is in context.
func RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if User(c) == nil {
			c.AbortWithStatusJSON(401, gin.H{"code": "unauthorized", "message": "authentication required"})
			return
		}
		c.Next()
	}
}

// User returns the authenticated user or nil.
func User(c *gin.Context) *service.User {
	v, ok := c.Get(ctxUserKey)
	if !ok {
		return nil
	}
	u, _ := v.(*service.User)
	return u
}
