// Command api starts the DoTheSplit HTTP API server.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/julian-alarcon/dothesplit/api/internal/config"
	"github.com/julian-alarcon/dothesplit/api/internal/crypto"
	"github.com/julian-alarcon/dothesplit/api/internal/handlers"
	"github.com/julian-alarcon/dothesplit/api/internal/realtime"
	"github.com/julian-alarcon/dothesplit/api/internal/server"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
	"github.com/julian-alarcon/dothesplit/api/internal/storefactory"
	"github.com/julian-alarcon/dothesplit/api/internal/worker"
)

// Stamped at build time via go build -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "dev"
)

func main() {
	// Self-probe used by the Docker HEALTHCHECK; runs before any DB or
	// config setup so a sick instance can still answer the probe path.
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		runHealthcheck()
		return
	}

	// Config decides the log level, so parse it before building the real
	// logger; a throwaway boot logger covers a failed config load.
	bootLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		bootLogger.Error("load config", slog.String("err", err.Error()))
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.SlogLevel()}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Build the engine-specific store (Postgres or SQLite) and realtime wiring.
	sf, err := storefactory.Open(ctx, cfg, logger)
	if err != nil {
		logger.Error("open store", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer sf.Close()
	store := sf.Store
	hub := sf.Hub

	email, err := crypto.NewEmailCipher(cfg.EmailEncKey, cfg.EmailHMACKey)
	if err != nil {
		logger.Error("email cipher", slog.String("err", err.Error()))
		os.Exit(1)
	}

	users := store.Users()
	refreshTokens := store.RefreshTokens()
	groups := store.Groups()
	expenses := store.Expenses()
	settlements := store.Settlements()
	balances := store.Balances()
	recurring := store.Recurring()
	categories := store.Categories()
	transactionRepo := store.Transactions()
	activityRepo := store.Activity()
	searchRepo := store.Search()
	auditRepo := store.Audit()
	smtpRepo := store.Smtp()
	setupRepo := store.Setup()
	verificationRepo := store.Verification()
	outboxRepo := store.EmailOutbox()
	mailerSvc := service.NewMailerService(smtpRepo, outboxRepo, email, cfg.WebOrigin, logger)
	auth := service.NewAuthService(store, users, auditRepo, verificationRepo, mailerSvc, setupRepo, email, cfg.PasswordPepper)
	auth.SetTokenAuth(refreshTokens, cfg.JWTSigningKey,
		time.Duration(cfg.AccessTokenTTLMin)*time.Minute,
		time.Duration(cfg.RefreshTokenTTLDay)*24*time.Hour)
	notificationSvc := service.NewNotificationService(users, mailerSvc, email)

	meSvc := service.NewMeService(users, email, cfg.PasswordPepper)
	meSvc.SetAuth(auth)
	categorySvc := service.NewCategoryService(categories)
	groupSvc := service.NewGroupService(groups, users, balances, email)
	expenseSvc := service.NewExpenseService(expenses, groups, categorySvc)
	balanceSvc := service.NewBalanceService(balances, groups)
	settlementSvc := service.NewSettlementService(settlements, groups)
	recurringSvc := service.NewRecurringService(recurring, expenses, groups, categorySvc)
	transactionSvc := service.NewTransactionService(groupSvc, transactionRepo, expenses, settlements, recurring)
	activitySvc := service.NewActivityService(groupSvc, activityRepo)
	searchSvc := service.NewSearchService(groupSvc, groups, searchRepo, expenses, settlements)
	importSvc := service.NewSplitwiseImporter(store, users, groups, expenseSvc, categorySvc, settlements, auth, email)
	groupExpenseImporterSvc := service.NewGroupExpenseImporter(groups, groupSvc, expenseSvc, categorySvc)
	exporterSvc := service.NewGroupCSVExporter(groupSvc, groups, expenseSvc, settlements, categorySvc, users)
	adminSvc := service.NewAdminService(store, users, groups, auditRepo, auth, email, cfg.PasswordPepper)
	smtpSvc := service.NewSmtpService(smtpRepo, email)
	setupSvc := service.NewSetupService(store, setupRepo, auth, auditRepo)

	// Wire notifications into the services that produce them. The hook is
	// optional so tests can construct services without a real mailer.
	groupSvc.SetNotifications(notificationSvc)
	settlementSvc.SetNotifications(users, notificationSvc)
	recurringSvc.SetNotifications(users, notificationSvc)

	// First-run setup: rotate the install token on every boot until consumed.
	// The cleartext is logged once as a warning so the operator can grab it
	// from `docker compose logs api`. Once setup is completed the banner is
	// suppressed and the token cleartext is gone - only its SHA-256 lives in
	// app_setup, and even that is unreachable from any post-setup code path.
	if ct, _, completed, err := setupSvc.EnsureToken(ctx); err != nil {
		logger.Error("setup ensure token", slog.String("err", err.Error()))
		os.Exit(1)
	} else if !completed {
		logger.Warn("first-run setup required",
			slog.String("url", cfg.WebOrigin+"/setup"),
			slog.String("token", ct),
			slog.String("note", "Visit the URL and paste the token. This banner stops once setup is consumed."),
		)
	}

	srv := &handlers.Server{
		Cfg: cfg, Store: store,
		Auth:             auth,
		MeSvc:            meSvc,
		Groups:           groupSvc,
		Categories:       categorySvc,
		Expenses:         expenseSvc,
		Balances:         balanceSvc,
		Settlements:      settlementSvc,
		Recurring:        recurringSvc,
		Transactions:     transactionSvc,
		Activity:         activitySvc,
		SearchSvc:        searchSvc,
		Imports:          importSvc,
		GroupExpenseImps: groupExpenseImporterSvc,
		Exporter:         exporterSvc,
		Admin:            adminSvc,
		Smtp:             smtpSvc,
		Setup:            setupSvc,
		Mailer:           mailerSvc,
		Notifications:    notificationSvc,
		Users:            users,
		Audit:            auditRepo,
		Hub:              hub,
		Version:          version,
		Commit:           commit,
	}
	h := server.New(srv)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Real-time delivery. Postgres: park a LISTEN connection that fans DB
	// notifications out to SSE subscribers (its own single-conn pool, so it
	// never competes with request queries). SQLite: the store publishes
	// committed events straight to the hub, so no listener runs.
	if sf.UsesListener {
		go realtime.RunListener(ctx, sf.ListenerPool, hub, logger)
	}

	// Embedded worker: when WORKER_MODE=embedded (always on SQLite) run the
	// recurring/outbox tick in-process instead of a separate container. On
	// Postgres the tick's advisory lock still coordinates with any external
	// worker, so running both is safe.
	if cfg.WorkerMode == "embedded" {
		go worker.Run(ctx, store, recurringSvc, mailerSvc, 60*time.Second, logger)
		logger.Info("embedded worker started")
	}

	go func() {
		logger.Info("listening", slog.String("addr", cfg.HTTPAddr))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http serve", slog.String("err", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", slog.String("err", err.Error()))
	}
}
