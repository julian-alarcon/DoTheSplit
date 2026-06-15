package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// GetSetupStatus exposes a single boolean: whether the install ceremony has
// finished. It's intentionally public (no auth, no rate limit). The bit
// already leaks via the existence of /setup itself, and the frontend
// middleware needs it on every render to decide whether to redirect into
// the install flow.
func (s *Server) GetSetupStatus(c *gin.Context) {
	locked, err := s.Setup.Locked(c.Request.Context())
	if err != nil {
		writeErr(c, http.StatusInternalServerError, "internal", "could not read setup state")
		return
	}
	// Never let a stale CDN/proxy cache the "still in setup" answer.
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, apigen.SetupStatus{Locked: locked})
}

// CompleteSetup is the only path that mints the very first admin user.
// SetupService.CompleteWithToken handles the atomic ceremony (constant-time
// token compare → RegisterTx with bootstrap path → Complete the row →
// audit row), all inside one transaction.
func (s *Server) CompleteSetup(c *gin.Context) {
	var req apigen.SetupCompleteRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	u, err := s.Setup.CompleteWithToken(
		c.Request.Context(),
		req.Token,
		string(req.Email),
		req.DisplayName,
		req.Password,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			writeErr(c, http.StatusUnauthorized, "invalid_token", "invalid setup token")
		case errors.Is(err, service.ErrSetupCompleted):
			writeErr(c, http.StatusGone, "setup_completed", "setup has already been completed")
		case errors.Is(err, service.ErrEmailTaken):
			writeErr(c, http.StatusConflict, "email_taken", "email already registered")
		default:
			writeErr(c, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}
	c.JSON(http.StatusCreated, toAPIUser(u))
}
