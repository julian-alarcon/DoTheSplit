package middleware

import (
	"net/http"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

// RequireAdmin must run after RequireSession. It re-loads the user from the
// database on every request rather than trusting the cached service.User
// projection so role revocation takes effect immediately. A soft-deleted
// admin is also rejected.
func RequireAdmin(users repo.UserRepo) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := User(r.Context())
			if u == nil {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			fresh, err := users.FindByID(r.Context(), u.ID)
			if err != nil || fresh == nil || fresh.DeletedAt != nil || fresh.Role != "admin" {
				writeJSONError(w, http.StatusForbidden, "forbidden", "admin role required")
				return
			}
			// The step-up flow re-verifies the password against the same DB row.
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Frame-Options", "DENY")
			next.ServeHTTP(w, r)
		})
	}
}
