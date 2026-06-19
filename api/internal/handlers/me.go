package handlers

import (
	"errors"
	"net/http"

	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// UpdateMe applies a partial update to the current user. Currently supports
// display_name and week_start; either or both may be supplied.
func (s *Server) UpdateMe(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req apigen.UpdateMeRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	if req.DisplayName == nil && req.WeekStart == nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "nothing to update")
		return
	}
	if req.DisplayName != nil {
		if err := s.MeSvc.Rename(r.Context(), u.ID, *req.DisplayName); err != nil {
			switch {
			case errors.Is(err, repo.ErrNotFound):
				writeErr(w, http.StatusNotFound, "not_found", "user not found")
			default:
				writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
			}
			return
		}
	}
	if req.WeekStart != nil {
		if err := s.MeSvc.SetWeekStart(r.Context(), u.ID, int16(*req.WeekStart)); err != nil {
			switch {
			case errors.Is(err, repo.ErrNotFound):
				writeErr(w, http.StatusNotFound, "not_found", "user not found")
			default:
				writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
			}
			return
		}
	}
	// Reload through AuthService so the response reflects any newly-set fields.
	fresh, err := s.Auth.GetUser(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPIUser(fresh))
}

// ChangePassword verifies the old password and rotates to a new one. Every
// other token chain is revoked; a fresh token pair is minted and returned (with
// a rotated refresh cookie) so the current client stays logged in.
func (s *Server) ChangePassword(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req apigen.ChangePasswordRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	if err := s.MeSvc.ChangePassword(r.Context(), u.ID, req.OldPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, service.ErrWrongPassword):
			writeErr(w, http.StatusUnauthorized, "invalid_credentials", "old password is incorrect")
		case errors.Is(err, service.ErrUserDeleted):
			writeErr(w, http.StatusUnauthorized, "unauthorized", "account is deleted")
		default:
			writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}
	// All token chains (including ours) were revoked. Mint a fresh pair so the
	// user doesn't have to log in again from the same browser.
	pair, err := s.Auth.MintTokenPairForUser(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.setRefreshCookie(w, pair.RefreshToken, pair.RefreshTTL)
	writeJSON(w, http.StatusOK, tokenResponse(pair))
}

// SetAvatar validates and stores an 8x8 PNG.
func (s *Server) SetAvatar(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req apigen.SetAvatarRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	if err := s.MeSvc.SetAvatarFromBase64(r.Context(), u.ID, req.PngBase64); err != nil {
		if errors.Is(err, service.ErrBadAvatar) {
			writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteAvatar clears the avatar; the UI falls back to initials.
func (s *Server) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if err := s.MeSvc.ClearAvatar(r.Context(), u.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteMe soft-deletes the calling account, scrubs PII, revokes refresh
// tokens, and clears the refresh cookie. Requires the caller to re-enter their
// password (step-up) to make session hijack → instant account loss harder.
func (s *Server) DeleteMe(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req apigen.DeleteMeJSONRequestBody
	if !bindStrictJSON(w, r, &req) {
		return
	}
	if err := s.Auth.VerifyPassword(r.Context(), u.ID, req.Password); err != nil {
		switch {
		case errors.Is(err, service.ErrStepUpRateLimited):
			writeErr(w, http.StatusLocked, "rate_limited", "too many failed password attempts")
		default:
			writeErr(w, http.StatusUnauthorized, "invalid_credentials", "password is incorrect")
		}
		return
	}
	if err := s.MeSvc.SoftDelete(r.Context(), u.ID); err != nil {
		if errors.Is(err, service.ErrUserDeleted) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "account is already deleted")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// GetUserAvatar serves the 8x8 PNG for a user the caller shares a group with.
func (s *Server) GetUserAvatar(w http.ResponseWriter, r *http.Request) {
	me := middleware.User(r.Context())
	if me == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	target, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if me.ID != target {
		shares, err := s.Groups.ShareAnyGroup(r.Context(), me.ID, target)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if !shares {
			writeErr(w, http.StatusForbidden, "forbidden", "not authorized to view this avatar")
			return
		}
	}
	png, err := s.MeSvc.GetAvatar(r.Context(), target)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "no avatar set")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

// ChangeEmailRequest begins the change-email flow: re-verifies the password
// (step-up), persists a token row keyed on the *new* email, and enqueues a
// 6-digit code to that new address. The caller's email is unchanged until
// they confirm.
func (s *Server) ChangeEmailRequest(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req apigen.ChangeEmailRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	err := s.Auth.RequestEmailChange(r.Context(), u.ID, string(req.NewEmail), req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			writeErr(w, http.StatusUnauthorized, "invalid_credentials", "current password is incorrect")
		case errors.Is(err, service.ErrStepUpRateLimited):
			writeErr(w, http.StatusLocked, "rate_limited", "too many failed password attempts")
		case errors.Is(err, service.ErrEmailTaken):
			writeErr(w, http.StatusConflict, "email_taken", "email already in use")
		default:
			writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// ChangeEmailConfirm consumes the code, swaps the email, revokes the user's
// other token chains, and mints a fresh pair (rotating the refresh cookie) so
// the current browser stays logged in.
func (s *Server) ChangeEmailConfirm(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req apigen.ConfirmEmailChangeRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	user, err := s.Auth.ConfirmEmailChange(r.Context(), u.ID, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCode):
			writeErr(w, http.StatusBadRequest, "invalid_code", "verification code is incorrect")
		case errors.Is(err, service.ErrCodeExpired):
			writeErr(w, http.StatusGone, "code_expired", "verification code has expired or is no longer valid")
		case errors.Is(err, service.ErrVerifyRateLimited):
			writeErr(w, http.StatusTooManyRequests, "too_many_attempts", "too many incorrect attempts; request a new code")
		case errors.Is(err, service.ErrEmailTaken):
			writeErr(w, http.StatusConflict, "email_taken", "email already in use")
		default:
			writeErr(w, http.StatusInternalServerError, "internal", "confirm failed")
		}
		return
	}
	if pair, err := s.Auth.MintTokenPairForUser(r.Context(), u.ID); err == nil {
		s.setRefreshCookie(w, pair.RefreshToken, pair.RefreshTTL)
	}
	writeJSON(w, http.StatusOK, toAPIUser(user))
}

// GetMyNotifications returns the caller's notification preferences.
func (s *Server) GetMyNotifications(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	p, err := s.Notifications.GetPrefs(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "load prefs failed")
		return
	}
	writeJSON(w, http.StatusOK, toAPIPrefs(p))
}

// UpdateMyNotifications writes the caller's notification preferences.
func (s *Server) UpdateMyNotifications(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req apigen.NotificationPrefs
	if !bindStrictJSON(w, r, &req) {
		return
	}
	in := &service.NotificationPrefs{}
	if req.NotifyRecurringRun != nil {
		in.NotifyRecurringRun = *req.NotifyRecurringRun
	}
	if req.NotifySettlement != nil {
		in.NotifySettlement = *req.NotifySettlement
	}
	if req.NotifyGroupAdded != nil {
		in.NotifyGroupAdded = *req.NotifyGroupAdded
	}
	out, err := s.Notifications.UpdatePrefs(r.Context(), u.ID, in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "save prefs failed")
		return
	}
	writeJSON(w, http.StatusOK, toAPIPrefs(out))
}

func toAPIPrefs(p *service.NotificationPrefs) apigen.NotificationPrefs {
	rr := p.NotifyRecurringRun
	st := p.NotifySettlement
	ga := p.NotifyGroupAdded
	return apigen.NotificationPrefs{
		NotifyRecurringRun: &rr,
		NotifySettlement:   &st,
		NotifyGroupAdded:   &ga,
	}
}
