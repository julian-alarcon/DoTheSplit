package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/julian-alarcon/dothesplit/api/internal/config"
	"github.com/julian-alarcon/dothesplit/api/internal/crypto"
	"github.com/julian-alarcon/dothesplit/api/internal/handlers"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
	"github.com/julian-alarcon/dothesplit/api/internal/server"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// TestSessionCookieNamingSwitch boots two stacks - one with CookieSecure=true,
// one with CookieSecure=false - and asserts the Set-Cookie name on the
// /v1/setup/admin response matches the contract documented in AGENTS.md
// "Cookie naming": __Host-dts_session for HTTPS, dts_session for plain.
//
// This is the wiring test. The pure-unit version of the contract lives in
// middleware/session_test.go; this one catches a regression where someone
// hardcodes the cookie name in a handler instead of routing through
// middleware.SessionCookieName.
func TestSessionCookieNamingSwitch(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}

	cases := []struct {
		name         string
		cookieSecure bool
		want         string
	}{
		{"https_uses_host_prefix", true, "__Host-dts_session"},
		{"plain_http_drops_prefix", false, "dts_session"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := bootStackWithCookieSecure(t, tc.cookieSecure)
			resp, _ := registerForCookieTest(t, srv.URL, "first@test.dev", "passwordpassword", "First")
			defer resp.Body.Close()
			require.Equal(t, http.StatusCreated, resp.StatusCode)

			// Find the session cookie among Set-Cookie headers; assert no
			// alternate name leaked.
			var got *http.Cookie
			for _, c := range resp.Cookies() {
				if c.Value != "" && (c.Name == "__Host-dts_session" || c.Name == "dts_session") {
					got = c
					break
				}
			}
			require.NotNil(t, got, "expected a session cookie on /v1/setup/admin response")
			require.Equal(t, tc.want, got.Name)

			// HTTPS deployments must set Secure=true; plain HTTP must not, or
			// browsers refuse the __Host- cookie / leak it over plain text.
			require.Equal(t, tc.cookieSecure, got.Secure, "Secure flag must match CookieSecure")
		})
	}
}

// bootStackWithCookieSecure mirrors setup() but takes a CookieSecure flag.
// Kept narrow on purpose: the existing setup() is wide and we don't want to
// destabilise its 30+ callers for this one test.
func bootStackWithCookieSecure(t *testing.T, cookieSecure bool) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	pgc, err := tcpg.Run(ctx,
		"postgres:16-alpine",
		tcpg.WithDatabase("dts"),
		tcpg.WithUsername("dts"),
		tcpg.WithPassword("dts"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)

	dsn, err := pgc.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	poolCfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	poolCfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, readMigrations(t))
	require.NoError(t, err)

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cfg := &config.Config{
		DatabaseURL:    dsn,
		SessionTTLDay:  30,
		EmailEncKey:    key,
		EmailHMACKey:   key,
		PasswordPepper: key,
		CookieSecure:   cookieSecure,
	}
	emailC, err := crypto.NewEmailCipher(cfg.EmailEncKey, cfg.EmailHMACKey)
	require.NoError(t, err)

	users := repo.NewUserRepo(pool)
	sessions := repo.NewSessionRepo(pool)
	groups := repo.NewGroupRepo(pool)
	expenses := repo.NewExpenseRepo(pool)
	settlements := repo.NewSettlementRepo(pool)
	balances := repo.NewBalanceRepo(pool)
	recurring := repo.NewRecurringRepo(pool)
	categories := repo.NewCategoryRepo(pool)
	categorySvc := service.NewCategoryService(categories)
	activityRepo := repo.NewActivityRepo(pool)
	auditRepo := repo.NewAuditRepo(pool)
	smtpRepo := repo.NewSmtpRepo(pool)
	setupRepo := repo.NewSetupRepo(pool)
	verificationRepo := repo.NewVerificationRepo(pool)
	outboxRepo := repo.NewEmailOutboxRepo(pool)

	ttl := time.Duration(cfg.SessionTTLDay) * 24 * time.Hour
	groupSvc := service.NewGroupService(groups, users, balances, emailC)
	mailerSvc := service.NewMailerService(smtpRepo, outboxRepo, emailC, cfg.WebOrigin, nil)
	authSvc := service.NewAuthService(pool, users, sessions, auditRepo, verificationRepo, mailerSvc, setupRepo, emailC, cfg.PasswordPepper, ttl)
	setupSvc := service.NewSetupService(pool, setupRepo, authSvc, auditRepo)
	tok, _, _, err := setupSvc.EnsureToken(ctx)
	require.NoError(t, err)
	settlementSvc := service.NewSettlementService(settlements, groups)
	recurringSvc := service.NewRecurringService(recurring, expenses, groups, categorySvc)

	h := server.New(&handlers.Server{
		Cfg:         cfg,
		Pool:        pool,
		Auth:        authSvc,
		MeSvc:       service.NewMeService(users, sessions, emailC, cfg.PasswordPepper),
		Groups:      groupSvc,
		Categories:  categorySvc,
		Expenses:    service.NewExpenseService(expenses, groups, categorySvc),
		Balances:    service.NewBalanceService(balances, groups),
		Settlements: settlementSvc,
		Recurring:   recurringSvc,
		Activity:    service.NewActivityService(groupSvc, activityRepo, expenses, settlements, recurring),
		SearchSvc:   service.NewSearchService(groupSvc, groups, repo.NewSearchRepo(pool), expenses, settlements),
		Admin:       service.NewAdminService(pool, users, groups, sessions, auditRepo, authSvc, emailC, cfg.PasswordPepper),
		Smtp:        service.NewSmtpService(smtpRepo, emailC),
		Setup:       setupSvc,
		Mailer:      mailerSvc,
		Users:       users,
		Audit:       auditRepo,
	})
	srv := httptest.NewServer(h)
	setupTokens.Store(srv.URL, tok)

	t.Cleanup(func() {
		srv.Close()
		pool.Close()
		_ = pgc.Terminate(context.Background())
	})
	return srv, pool
}

// registerForCookieTest does an install-ceremony register and returns the
// raw response (not the parsed body) so the test can read Set-Cookie
// headers including their flags (Secure, HttpOnly, ...).
func registerForCookieTest(t *testing.T, base, email, pw, name string) (*http.Response, []byte) {
	t.Helper()
	tokAny, _ := setupTokens.Load(base)
	tok, _ := tokAny.(string)
	body := map[string]any{
		"email": email, "password": pw, "display_name": name, "token": tok,
	}
	resp, raw := rawRequest(t, "POST", base+"/v1/setup/admin", body, nil)
	return resp, raw
}
