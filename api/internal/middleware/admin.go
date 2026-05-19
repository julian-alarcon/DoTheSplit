package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

// RequireAdmin must run after RequireSession. It re-loads the user from the
// database on every request rather than trusting the cached service.User
// projection so role revocation takes effect immediately. A soft-deleted
// admin is also rejected.
func RequireAdmin(users *repo.UserRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := User(c)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "authentication required"})
			return
		}
		fresh, err := users.FindByID(c.Request.Context(), u.ID)
		if err != nil || fresh == nil || fresh.DeletedAt != nil || fresh.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "admin role required"})
			return
		}
		// Make the freshly-loaded role available to downstream handlers; the
		// step-up flow re-verifies the password against the same DB row.
		c.Header("Cache-Control", "no-store")
		c.Header("X-Frame-Options", "DENY")
		c.Next()
	}
}
