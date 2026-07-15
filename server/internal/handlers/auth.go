package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/julian-alarcon/dothesplit/server/internal/apigen"
	"github.com/julian-alarcon/dothesplit/server/internal/middleware"
	"github.com/julian-alarcon/dothesplit/server/internal/service"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	var req apigen.RegisterRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	res, err := s.Auth.Register(r.Context(), string(req.Email), req.Password, req.DisplayName)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSetupRequired):
			writeErr(w, http.StatusForbidden, "setup_required",
				"instance is in first-run setup mode; visit /setup")
		case errors.Is(err, service.ErrEmailTaken):
			writeErr(w, http.StatusConflict, "email_taken", "email already registered")
		default:
			writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}
	user := toAPIUser(res.User)
	writeJSON(w, http.StatusCreated, apigen.RegisterResponse{
		User:                 user,
		VerificationRequired: res.VerificationRequired,
	})
}

func (s *Server) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req apigen.VerifyEmailRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	u, err := s.Auth.VerifyEmail(r.Context(), string(req.Email), req.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCode):
			writeErr(w, http.StatusBadRequest, "invalid_code", "verification code is incorrect")
		case errors.Is(err, service.ErrCodeExpired):
			writeErr(w, http.StatusGone, "code_expired", "verification code has expired or is no longer valid")
		case errors.Is(err, service.ErrVerifyRateLimited):
			writeErr(w, http.StatusTooManyRequests, "too_many_attempts", "too many incorrect attempts; request a new code")
		default:
			writeErr(w, http.StatusInternalServerError, "internal", "verify failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, toAPIUser(u))
}

func (s *Server) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req apigen.ResendVerificationRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	// Always 204 to avoid account enumeration.
	_ = s.Auth.ResendVerification(r.Context(), string(req.Email))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req apigen.PasswordResetRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	// Always 204 to avoid account enumeration.
	_ = s.Auth.RequestPasswordReset(r.Context(), string(req.Email))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req apigen.PasswordResetConfirm
	if !bindStrictJSON(w, r, &req) {
		return
	}
	u, err := s.Auth.ConfirmPasswordReset(r.Context(), string(req.Email), req.Code, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCode):
			writeErr(w, http.StatusBadRequest, "invalid_code", "verification code is incorrect")
		case errors.Is(err, service.ErrCodeExpired):
			writeErr(w, http.StatusGone, "code_expired", "verification code has expired or is no longer valid")
		case errors.Is(err, service.ErrVerifyRateLimited):
			writeErr(w, http.StatusTooManyRequests, "too_many_attempts", "too many incorrect attempts; request a new code")
		default:
			writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, toAPIUser(u))
}

// IssueToken exchanges credentials for a JWT access token + rotating refresh
// token. Web clients get the refresh token as the dts_refresh httpOnly cookie;
// the response body omits it. (Native clients have no cookie jar, but since the
// server can't tell them apart from the request, we always set the cookie and
// also return the refresh token in the body so native can read it.)
func (s *Server) IssueToken(w http.ResponseWriter, r *http.Request) {
	var req apigen.LoginRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	_, pair, err := s.Auth.IssueTokenPair(r.Context(), string(req.Email), req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailUnverified):
			writeErr(w, http.StatusForbidden, "email_unverified", "email address not yet verified")
		case errors.Is(err, service.ErrTokenAuthDisabled):
			writeErr(w, http.StatusServiceUnavailable, "token_auth_disabled", "token auth is not configured")
		default:
			writeErr(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		}
		return
	}
	s.setRefreshCookie(w, pair.RefreshToken, pair.RefreshTTL)
	writeJSON(w, http.StatusOK, tokenResponse(pair))
}

// RefreshToken rotates the refresh token and mints a new access token.
func (s *Server) RefreshToken(w http.ResponseWriter, r *http.Request) {
	refresh := s.readRefreshToken(r)
	pair, err := s.Auth.RefreshTokenPair(r.Context(), refresh)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTokenAuthDisabled):
			writeErr(w, http.StatusServiceUnavailable, "token_auth_disabled", "token auth is not configured")
		default:
			s.clearRefreshCookie(w)
			writeErr(w, http.StatusUnauthorized, "invalid_token", "refresh token is invalid or expired")
		}
		return
	}
	s.setRefreshCookie(w, pair.RefreshToken, pair.RefreshTTL)
	writeJSON(w, http.StatusOK, tokenResponse(pair))
}

// RevokeToken revokes the presented refresh token (token-client logout).
func (s *Server) RevokeToken(w http.ResponseWriter, r *http.Request) {
	refresh := s.readRefreshToken(r)
	_ = s.Auth.RevokeRefreshToken(r.Context(), refresh)
	s.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func tokenResponse(p *service.TokenPair) apigen.TokenResponse {
	rt := p.RefreshToken
	return apigen.TokenResponse{
		AccessToken:  p.AccessToken,
		TokenType:    apigen.Bearer,
		ExpiresIn:    int(p.AccessTTL.Seconds()),
		RefreshToken: &rt,
	}
}

// readRefreshToken prefers the request body's refresh_token (native clients),
// falling back to the dts_refresh cookie (web). The body read is best-effort;
// an absent/invalid body is fine.
func (s *Server) readRefreshToken(r *http.Request) string {
	var req apigen.RefreshRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err == nil && req.RefreshToken != nil && *req.RefreshToken != "" {
		return *req.RefreshToken
	}
	if ck, err := r.Cookie(refreshCookieName); err == nil {
		return ck.Value
	}
	return ""
}

func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	writeJSON(w, http.StatusOK, toAPIUser(u))
}

func toAPIUser(u *service.User) apigen.User {
	isAdmin := u.IsAdmin
	out := apigen.User{
		Id:              u.ID,
		Email:           openapi_types.Email(u.Email),
		DisplayName:     u.DisplayName,
		CreatedAt:       u.CreatedAt,
		HasAvatar:       u.HasAvatar,
		AvatarUpdatedAt: u.AvatarUpdatedAt,
		DeletedAt:       u.DeletedAt,
		WeekStart:       apigen.UserWeekStart(u.WeekStart),
		IsAdmin:         &isAdmin,
		EmailVerifiedAt: u.EmailVerifiedAt,
	}
	return out
}
