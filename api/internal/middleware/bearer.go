package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// AccessTokenResolver is the subset of AuthService the bearer middleware needs.
type AccessTokenResolver interface {
	ResolveAccessToken(ctx context.Context, token string) (*service.User, error)
}

// Bearer populates the request with the authenticated user from an
// `Authorization: Bearer <jwt>` header, if present and valid. It does NOT
// require a token, and it does NOT overwrite a user already set by the cookie
// Session middleware (cookie wins). Pair it with RequireSession to enforce.
func Bearer(r AccessTokenResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if User(c) != nil {
			c.Next()
			return
		}
		h := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
			c.Next()
			return
		}
		token := strings.TrimSpace(h[len(prefix):])
		if token == "" {
			c.Next()
			return
		}
		u, err := r.ResolveAccessToken(c.Request.Context(), token)
		if err != nil {
			c.Next()
			return
		}
		c.Set(ctxUserKey, u)
		c.Next()
	}
}
