package handlers

import (
	"errors"
	"net/http"

	"github.com/julian-alarcon/dothesplit/server/internal/apigen"
	"github.com/julian-alarcon/dothesplit/server/internal/service"
)

// GetSetupStatus exposes a single boolean: whether the install ceremony has
// finished. It's intentionally public (no auth, no rate limit). The bit
// already leaks via the existence of /setup itself, and the frontend
// middleware needs it on every render to decide whether to redirect into
// the install flow.
func (s *Server) GetSetupStatus(w http.ResponseWriter, r *http.Request) {
	locked, err := s.Setup.Locked(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "could not read setup state")
		return
	}
	// Never let a stale CDN/proxy cache the "still in setup" answer.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, apigen.SetupStatus{Locked: locked})
}

// CompleteSetup is the only path that mints the very first admin user.
// SetupService.CompleteWithToken handles the atomic ceremony (constant-time
// token compare → RegisterTx with bootstrap path → Complete the row →
// audit row), all inside one transaction.
func (s *Server) CompleteSetup(w http.ResponseWriter, r *http.Request) {
	var req apigen.SetupCompleteRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	u, err := s.Setup.CompleteWithToken(
		r.Context(),
		req.Token,
		string(req.Email),
		req.DisplayName,
		req.Password,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			writeErr(w, http.StatusUnauthorized, "invalid_token", "invalid setup token")
		case errors.Is(err, service.ErrSetupCompleted):
			writeErr(w, http.StatusGone, "setup_completed", "setup has already been completed")
		case errors.Is(err, service.ErrEmailTaken):
			writeErr(w, http.StatusConflict, "email_taken", "email already registered")
		default:
			writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, toAPIUser(u))
}
