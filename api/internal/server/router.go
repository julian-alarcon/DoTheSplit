// Package server wires middlewares and registers routes onto a net/http ServeMux.
package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/julian-alarcon/dothesplit/api/internal/handlers"
	mw "github.com/julian-alarcon/dothesplit/api/internal/middleware"
)

// New builds the top-level http.Handler using net/http's ServeMux.
func New(s *handlers.Server) http.Handler {
	// Resolve the client IP under the configured trusted-proxy policy. With an
	// empty set nothing is trusted, so a forged X-Forwarded-For can't spoof the
	// IP used for rate limiting + audit logs.
	ip, err := mw.NewTrustedProxies(s.Cfg.TrustedProxies)
	if err != nil {
		panic("server: trusted proxies: " + err.Error())
	}
	s.IP = ip

	mux := http.NewServeMux()

	// Per-route guards, built once and reused.
	requireSession := mw.RequireSession()
	requireAdmin := mw.RequireAdmin(s.Users)
	authLimit := mw.LoginRateLimiter(s.Cfg.AuthRateLimitPerMin, ip)
	setupLimit := mw.SetupRateLimiter(ip)

	// add registers a handler under an optional per-route middleware stack.
	add := func(pattern string, h http.HandlerFunc, mws ...mw.Middleware) {
		mux.Handle(pattern, mw.Chain(h, mws...))
	}

	// Health probes are unversioned infrastructure endpoints.
	mux.HandleFunc("GET /healthz", s.Healthz)
	mux.HandleFunc("GET /readyz", s.Readyz)

	// Setup ceremony. Public: /v1/setup/status returns one bool (consumed by the
	// web router on every render); /v1/setup/admin is the only path that mints
	// the first admin and is rate-limited tighter than /auth/register because
	// every successful POST is a one-shot.
	mux.HandleFunc("GET /v1/setup/status", s.GetSetupStatus)
	add("POST /v1/setup/admin", s.CompleteSetup, setupLimit)

	// Auth (rate-limited register + token issuance).
	add("POST /v1/auth/register", s.Register, authLimit)
	add("POST /v1/auth/verify", s.VerifyEmail, authLimit)
	add("POST /v1/auth/verify/resend", s.ResendVerification, authLimit)
	add("POST /v1/auth/password-reset/request", s.RequestPasswordReset, authLimit)
	add("POST /v1/auth/password-reset/confirm", s.ConfirmPasswordReset, authLimit)
	add("POST /v1/auth/token", s.IssueToken, authLimit)
	// Refresh + revoke are public (they authenticate via the refresh token
	// itself) and not rate-limited so a client can always rotate or log out.
	mux.HandleFunc("POST /v1/auth/refresh", s.RefreshToken)
	mux.HandleFunc("POST /v1/auth/token/revoke", s.RevokeToken)

	// Authenticated endpoints.
	add("GET /v1/me", s.Me, requireSession)
	add("PATCH /v1/me", s.UpdateMe, requireSession)
	add("DELETE /v1/me", s.DeleteMe, requireSession)
	add("POST /v1/me/password", s.ChangePassword, requireSession)
	add("POST /v1/me/email/change-request", s.ChangeEmailRequest, requireSession)
	add("POST /v1/me/email/change-confirm", s.ChangeEmailConfirm, requireSession)
	add("GET /v1/me/notifications", s.GetMyNotifications, requireSession)
	add("PATCH /v1/me/notifications", s.UpdateMyNotifications, requireSession)
	add("PUT /v1/me/avatar", s.SetAvatar, requireSession)
	add("DELETE /v1/me/avatar", s.DeleteAvatar, requireSession)
	add("GET /v1/users/{id}/avatar", s.GetUserAvatar, requireSession)

	add("GET /v1/groups", s.ListGroups, requireSession)
	add("POST /v1/groups", s.CreateGroup, requireSession)
	add("PATCH /v1/groups/{id}", s.UpdateGroup, requireSession)
	add("DELETE /v1/groups/{id}", s.DeleteGroup, requireSession)
	add("GET /v1/groups/{id}/export.csv", s.ExportGroupCSV, requireSession)
	add("POST /v1/groups/{id}/members", s.AddGroupMember, requireSession)
	add("DELETE /v1/groups/{id}/members/{userId}", s.RemoveGroupMember, requireSession)

	add("GET /v1/groups/{id}/expenses", s.ListExpenses, requireSession)
	add("POST /v1/groups/{id}/expenses", s.CreateExpense, requireSession)
	add("GET /v1/expenses/{id}", s.GetExpense, requireSession)
	add("PATCH /v1/expenses/{id}", s.UpdateExpense, requireSession)
	add("DELETE /v1/expenses/{id}", s.DeleteExpense, requireSession)
	add("POST /v1/expenses/{id}/restore", s.RestoreExpense, requireSession)
	add("GET /v1/expenses/{id}/revisions", s.ListExpenseRevisions, requireSession)

	add("GET /v1/categories", s.ListCategories, requireSession)

	add("GET /v1/groups/{id}/balances", s.GetBalances, requireSession)

	add("GET /v1/groups/{id}/settlements", s.ListSettlements, requireSession)
	add("POST /v1/groups/{id}/settlements", s.CreateSettlement, requireSession)
	add("GET /v1/settlements/{id}", s.GetSettlement, requireSession)
	add("PATCH /v1/settlements/{id}", s.UpdateSettlement, requireSession)
	add("DELETE /v1/settlements/{id}", s.DeleteSettlement, requireSession)
	add("POST /v1/settlements/{id}/restore", s.RestoreSettlement, requireSession)

	add("GET /v1/groups/{id}/transactions", s.ListTransactions, requireSession)
	add("GET /v1/groups/{id}/activity", s.ListActivity, requireSession)

	add("GET /v1/search", s.Search, requireSession)

	add("POST /v1/imports/splitwise", s.ImportSplitwise, requireSession)
	add("POST /v1/imports/dothesplit", s.ImportDoTheSplit, requireSession)
	add("POST /v1/groups/{id}/imports/expenses", s.ImportGroupExpensesCSV, requireSession)

	add("GET /v1/groups/{id}/recurring-expenses", s.ListRecurringExpenses, requireSession)
	add("POST /v1/groups/{id}/recurring-expenses", s.CreateRecurringExpense, requireSession)
	add("DELETE /v1/recurring-expenses/{id}", s.DeleteRecurringExpense, requireSession)

	// Admin endpoints. RequireAdmin re-loads the user from DB on every request
	// so role revocation is immediate; it also stamps no-store + X-Frame-Options
	// on the response. RequireSession runs first so it sees the authenticated user.
	add("GET /v1/admin/users", s.AdminListUsers, requireSession, requireAdmin)
	add("POST /v1/admin/users", s.AdminCreateUser, requireSession, requireAdmin)
	add("GET /v1/admin/users/{id}", s.AdminGetUser, requireSession, requireAdmin)
	add("DELETE /v1/admin/users/{id}", s.AdminDeleteUser, requireSession, requireAdmin)
	add("POST /v1/admin/users/{id}/password", s.AdminResetUserPassword, requireSession, requireAdmin)
	add("PATCH /v1/admin/users/{id}/role", s.AdminSetUserRole, requireSession, requireAdmin)
	add("GET /v1/admin/groups", s.AdminListGroups, requireSession, requireAdmin)
	add("DELETE /v1/admin/groups/{id}", s.AdminDeleteGroup, requireSession, requireAdmin)
	add("GET /v1/admin/smtp", s.AdminGetSmtp, requireSession, requireAdmin)
	add("PUT /v1/admin/smtp", s.AdminUpdateSmtp, requireSession, requireAdmin)
	add("GET /v1/admin/smtp/password", s.AdminRevealSmtpPassword, requireSession, requireAdmin)
	add("POST /v1/admin/smtp/test", s.AdminTestSmtp, requireSession, requireAdmin)
	add("POST /v1/admin/smtp/send-test", s.AdminSendSmtpTestEmail, requireSession, requireAdmin)
	add("GET /v1/admin/audit", s.AdminListAudit, requireSession, requireAdmin)

	// Embedded Vue SPA catch-all on "/". ServeMux gives every concrete pattern
	// above priority over this, so /v1, /healthz, and /readyz always win;
	// unknown GETs fall back to the SPA shell for client-side routing, and the
	// SPA handler itself returns JSON 404 for unmatched /v1 paths.
	spa, err := s.NewSPAHandler()
	if err != nil {
		panic("server: load embedded SPA: " + err.Error())
	}
	mux.Handle("/", spa)

	// Global middleware chain. The first listed runs outermost (sees the request
	// first, the response last), matching the previous gin r.Use() order.
	return mw.Chain(mux,
		mw.Recovery(slog.Default()),
		mw.MaxBodyBytes(mw.DefaultMaxBodyBytes),
		requestID(),
		mw.Logger(slog.Default(), ip),
		mw.SecurityHeaders(s.Cfg.CookieSecure),
		mw.CORS(s.Cfg.WebOrigin, strings.Join(s.Cfg.CapacitorOrigins, ",")),
		mw.Bearer(s.Auth),
	)
}

// requestID adds a short request identifier to each request: it echoes an
// inbound X-Request-Id or generates one, sets the response header, and stashes
// it in the request context for the access logger.
func requestID() mw.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" {
				id = randomID()
			}
			w.Header().Set("X-Request-Id", id)
			next.ServeHTTP(w, mw.WithRequestID(r, id))
		})
	}
}
