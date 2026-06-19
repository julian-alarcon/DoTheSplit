package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// AccessTokenResolver is the subset of AuthService the bearer middleware needs.
type AccessTokenResolver interface {
	ResolveAccessToken(ctx context.Context, token string) (*service.User, error)
}

// Bearer populates the request context with the authenticated user from an
// `Authorization: Bearer <jwt>` header, if present and valid. It does NOT
// require a token. Pair it with RequireSession to enforce.
func Bearer(r AccessTokenResolver) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if User(req.Context()) != nil {
				next.ServeHTTP(w, req)
				return
			}
			h := req.Header.Get("Authorization")
			const prefix = "Bearer "
			if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
				next.ServeHTTP(w, req)
				return
			}
			token := strings.TrimSpace(h[len(prefix):])
			if token == "" {
				next.ServeHTTP(w, req)
				return
			}
			u, err := r.ResolveAccessToken(req.Context(), token)
			if err != nil {
				next.ServeHTTP(w, req)
				return
			}
			next.ServeHTTP(w, WithUser(req, u))
		})
	}
}

// RequireSession responds 401 if no authenticated user is in the request context.
func RequireSession() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if User(req.Context()) == nil {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
