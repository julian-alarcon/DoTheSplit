// Command worker runs background jobs (recurring expenses + email outbox).
// On Postgres a session advisory lock (via the store's TickLocker) ensures only
// one instance materializes at a time. SQLite deployments don't run this binary
// - the api process runs the worker in-process (WORKER_MODE=embedded).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/julian-alarcon/dothesplit/api/internal/config"
	"github.com/julian-alarcon/dothesplit/api/internal/crypto"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
	"github.com/julian-alarcon/dothesplit/api/internal/storefactory"
	"github.com/julian-alarcon/dothesplit/api/internal/worker"
)

// Stamped at build time via -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "dev"
)

func main() {
	bootLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		bootLogger.Error("load config", slog.String("err", err.Error()))
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.SlogLevel()}))
	slog.SetDefault(logger)
	logger.Info("worker starting", slog.String("version", version), slog.String("commit", commit))

	// A standalone worker against SQLite would open a second connection to the
	// same file and contend with the api for the single writer, and it could not
	// reach the api's in-process realtime hub. SQLite must use the embedded
	// worker instead.
	if cfg.DatabaseDriver == "sqlite" {
		logger.Error("the standalone worker does not support DATABASE_DRIVER=sqlite; the api runs the worker in-process (WORKER_MODE=embedded)")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sf, err := storefactory.Open(ctx, cfg, logger)
	if err != nil {
		logger.Error("open store", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer sf.Close()
	store := sf.Store

	emailCipher, err := crypto.NewEmailCipher(cfg.EmailEncKey, cfg.EmailHMACKey)
	if err != nil {
		logger.Error("email cipher", slog.String("err", err.Error()))
		os.Exit(1)
	}

	categorySvc := service.NewCategoryService(store.Categories())
	mailerSvc := service.NewMailerService(store.Smtp(), store.EmailOutbox(), emailCipher, cfg.WebOrigin, logger)
	notificationSvc := service.NewNotificationService(store.Users(), mailerSvc, emailCipher)
	recurringSvc := service.NewRecurringService(store.Recurring(), store.Expenses(), store.Groups(), categorySvc)
	recurringSvc.SetNotifications(store.Users(), notificationSvc)

	worker.Run(ctx, store, recurringSvc, mailerSvc, 60*time.Second, logger)
}
