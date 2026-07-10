// Package handlers implements the HTTP surface using net/http.
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/config"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/realtime"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// Server bundles dependencies for all handlers.
type Server struct {
	Cfg              *config.Config
	Store            repo.Store
	Auth             *service.AuthService
	MeSvc            *service.MeService
	Groups           *service.GroupService
	Categories       *service.CategoryService
	Expenses         *service.ExpenseService
	Balances         *service.BalanceService
	Settlements      *service.SettlementService
	Recurring        *service.RecurringService
	Transactions     *service.TransactionService
	Activity         *service.ActivityService
	SearchSvc        *service.SearchService
	Imports          *service.SplitwiseImporter
	GroupExpenseImps *service.GroupExpenseImporter
	Exporter         *service.GroupCSVExporter
	Admin            *service.AdminService
	Smtp             *service.SmtpService
	Setup            *service.SetupService
	Mailer           *service.MailerService
	Notifications    *service.NotificationService
	Users            repo.UserRepo
	Audit            repo.AuditRepo

	// Hub fans out activity events to SSE clients. Optional: when nil, the
	// stream endpoint reports 503 (used by unit tests that don't wire realtime).
	Hub *realtime.Hub

	// IP resolves the originating client IP under the configured trusted-proxy
	// policy. Populated by server.New; used by audit logging and step-up.
	IP *middleware.TrustedProxies

	// Version and Commit are stamped into the binary at build time and
	// reported by /healthz so deployments can self-identify.
	Version string
	Commit  string
}

// clientIP resolves the request's client IP via the configured trusted-proxy
// policy, tolerating a nil IP (e.g. a Server built directly in a unit test).
func (s *Server) clientIP(r *http.Request) string {
	if s.IP == nil {
		return ""
	}
	return s.IP.ClientIP(r)
}

// writeJSON encodes v as the response body with the given status. Replaces
// gin's c.JSON.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apigen.Error{Code: code, Message: message})
}

// bindStrictJSON decodes the request body into dst, rejecting unknown fields
// and any trailing tokens. Matches additionalProperties: false in the spec.
// On failure it writes a 400 and returns false; callers should return early.
func bindStrictJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body: "+err.Error())
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "bad_request", "unexpected trailing JSON")
		return false
	}
	return true
}

// refreshCookieName is the httpOnly cookie carrying the rotating refresh token
// for web SPA clients. Scoped to /v1/auth so only the refresh + revoke
// endpoints ever receive it, minimizing its exposure surface.
const refreshCookieName = "dts_refresh"
const refreshCookiePath = "/v1/auth"

func (s *Server) setRefreshCookie(w http.ResponseWriter, token string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		Domain:   s.Cfg.CookieDomain,
		MaxAge:   int(ttl / time.Second),
		Secure:   s.Cfg.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		Domain:   s.Cfg.CookieDomain,
		MaxAge:   -1,
		Secure:   s.Cfg.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
