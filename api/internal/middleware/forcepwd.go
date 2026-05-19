package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// EnforcePasswordChange gates every authenticated endpoint when the
// must_change_password flag is set on the current user. Only the password-
// change endpoint and logout are allowed through, so the user can clear the
// flag and continue. The frontend uses the same signal to redirect to a
// dedicated change-password page.
//
// The check uses the user populated by Session(); RequireAdmin/RequireSession
// run before this on the protected group.
func EnforcePasswordChange() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := User(c)
		if u == nil || !u.MustChangePassword {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		// Allow the user to set a new password and to log out. Avatar fetches
		// for the same user are also allowed because the change-password page
		// renders the avatar.
		if path == "/v1/me/password" && c.Request.Method == http.MethodPost {
			c.Next()
			return
		}
		if path == "/v1/auth/logout" {
			c.Next()
			return
		}
		if path == "/v1/me" && c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		if strings.HasPrefix(path, "/v1/users/") && strings.HasSuffix(path, "/avatar") {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"code":    "password_change_required",
			"message": "this account must change its password before using the API",
		})
	}
}
