package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *Server) Register(c *gin.Context) {
	var req apigen.RegisterRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	res, err := s.Auth.Register(c.Request.Context(), string(req.Email), req.Password, req.DisplayName)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSetupRequired):
			writeErr(c, http.StatusForbidden, "setup_required",
				"instance is in first-run setup mode; visit /setup")
		case errors.Is(err, service.ErrEmailTaken):
			writeErr(c, http.StatusConflict, "email_taken", "email already registered")
		default:
			writeErr(c, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}
	if !res.VerificationRequired && res.SessionToken != "" {
		s.setSessionCookie(c, res.SessionToken)
	}
	user := toAPIUser(res.User)
	c.JSON(http.StatusCreated, apigen.RegisterResponse{
		User:                 user,
		VerificationRequired: res.VerificationRequired,
	})
}

func (s *Server) VerifyEmail(c *gin.Context) {
	var req apigen.VerifyEmailRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	u, token, err := s.Auth.VerifyEmail(c.Request.Context(), string(req.Email), req.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCode):
			writeErr(c, http.StatusBadRequest, "invalid_code", "verification code is incorrect")
		case errors.Is(err, service.ErrCodeExpired):
			writeErr(c, http.StatusGone, "code_expired", "verification code has expired or is no longer valid")
		case errors.Is(err, service.ErrVerifyRateLimited):
			writeErr(c, http.StatusTooManyRequests, "too_many_attempts", "too many incorrect attempts; request a new code")
		default:
			writeErr(c, http.StatusInternalServerError, "internal", "verify failed")
		}
		return
	}
	s.setSessionCookie(c, token)
	c.JSON(http.StatusOK, toAPIUser(u))
}

func (s *Server) ResendVerification(c *gin.Context) {
	var req apigen.ResendVerificationRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	// Always 204 to avoid account enumeration.
	_ = s.Auth.ResendVerification(c.Request.Context(), string(req.Email))
	c.Status(http.StatusNoContent)
}

func (s *Server) RequestPasswordReset(c *gin.Context) {
	var req apigen.PasswordResetRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	// Always 204 to avoid account enumeration.
	_ = s.Auth.RequestPasswordReset(c.Request.Context(), string(req.Email))
	c.Status(http.StatusNoContent)
}

func (s *Server) ConfirmPasswordReset(c *gin.Context) {
	var req apigen.PasswordResetConfirm
	if !bindStrictJSON(c, &req) {
		return
	}
	u, token, err := s.Auth.ConfirmPasswordReset(c.Request.Context(), string(req.Email), req.Code, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCode):
			writeErr(c, http.StatusBadRequest, "invalid_code", "verification code is incorrect")
		case errors.Is(err, service.ErrCodeExpired):
			writeErr(c, http.StatusGone, "code_expired", "verification code has expired or is no longer valid")
		case errors.Is(err, service.ErrVerifyRateLimited):
			writeErr(c, http.StatusTooManyRequests, "too_many_attempts", "too many incorrect attempts; request a new code")
		default:
			writeErr(c, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}
	s.setSessionCookie(c, token)
	c.JSON(http.StatusOK, toAPIUser(u))
}

func (s *Server) Login(c *gin.Context) {
	var req apigen.LoginRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	u, token, err := s.Auth.Login(c.Request.Context(), string(req.Email), req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailUnverified):
			writeErr(c, http.StatusForbidden, "email_unverified", "email address not yet verified")
		default:
			writeErr(c, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		}
		return
	}
	s.setSessionCookie(c, token)
	c.JSON(http.StatusOK, toAPIUser(u))
}

func (s *Server) Logout(c *gin.Context) {
	if token, err := c.Cookie(middleware.SessionCookieName(s.Cfg.CookieSecure)); err == nil {
		_ = s.Auth.Logout(c.Request.Context(), token)
	}
	s.clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

// IssueToken exchanges credentials for a JWT access token + rotating refresh
// token. Web clients get the refresh token as the dts_refresh httpOnly cookie;
// the response body omits it. (Native clients have no cookie jar, but since the
// server can't tell them apart from the request, we always set the cookie and
// also return the refresh token in the body so native can read it.)
func (s *Server) IssueToken(c *gin.Context) {
	var req apigen.LoginRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	_, pair, err := s.Auth.IssueTokenPair(c.Request.Context(), string(req.Email), req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailUnverified):
			writeErr(c, http.StatusForbidden, "email_unverified", "email address not yet verified")
		case errors.Is(err, service.ErrTokenAuthDisabled):
			writeErr(c, http.StatusServiceUnavailable, "token_auth_disabled", "token auth is not configured")
		default:
			writeErr(c, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		}
		return
	}
	s.setRefreshCookie(c, pair.RefreshToken, pair.RefreshTTL)
	c.JSON(http.StatusOK, tokenResponse(pair))
}

// RefreshToken rotates the refresh token and mints a new access token.
func (s *Server) RefreshToken(c *gin.Context) {
	refresh := s.readRefreshToken(c)
	pair, err := s.Auth.RefreshTokenPair(c.Request.Context(), refresh)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTokenAuthDisabled):
			writeErr(c, http.StatusServiceUnavailable, "token_auth_disabled", "token auth is not configured")
		default:
			s.clearRefreshCookie(c)
			writeErr(c, http.StatusUnauthorized, "invalid_token", "refresh token is invalid or expired")
		}
		return
	}
	s.setRefreshCookie(c, pair.RefreshToken, pair.RefreshTTL)
	c.JSON(http.StatusOK, tokenResponse(pair))
}

// RevokeToken revokes the presented refresh token (token-client logout).
func (s *Server) RevokeToken(c *gin.Context) {
	refresh := s.readRefreshToken(c)
	_ = s.Auth.RevokeRefreshToken(c.Request.Context(), refresh)
	s.clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
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
func (s *Server) readRefreshToken(c *gin.Context) string {
	var req apigen.RefreshRequest
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err == nil && req.RefreshToken != nil && *req.RefreshToken != "" {
		return *req.RefreshToken
	}
	if ck, err := c.Cookie(refreshCookieName); err == nil {
		return ck
	}
	return ""
}

func (s *Server) Me(c *gin.Context) {
	u := middleware.User(c)
	if u == nil {
		writeErr(c, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	c.JSON(http.StatusOK, toAPIUser(u))
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
		Timezone:        u.Timezone,
		IsAdmin:         &isAdmin,
		EmailVerifiedAt: u.EmailVerifiedAt,
	}
	return out
}
