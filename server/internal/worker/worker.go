// Package worker runs the periodic background tick (materialize due recurring
// expenses, drain the email outbox). The api starts it as an in-process
// goroutine at boot; there is no separate worker process. The app is
// single-node by design, so the tick runs unguarded.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/julian-alarcon/dothesplit/server/internal/service"
)

// Run drives the tick loop until ctx is cancelled. interval is the tick period
// (60s in production). Recurring is required; mailer may be nil (no outbox
// drain).
func Run(ctx context.Context, recurring *service.RecurringService, mailer *service.MailerService, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	logger.Info("worker started")
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopping")
			return
		case <-ticker.C:
			if err := RunOnce(ctx, recurring, mailer, logger); err != nil {
				logger.Error("tick", slog.String("err", err.Error()))
			}
		}
	}
}

// RunOnce performs a single tick: materialize due recurring expenses, then
// drain the outbox.
func RunOnce(ctx context.Context, recurring *service.RecurringService, mailer *service.MailerService, logger *slog.Logger) error {
	n, err := recurring.Tick(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		logger.Info("materialized recurring expenses", slog.Int("count", n))
	}

	if mailer != nil {
		sent, err := mailer.DispatchOutbox(ctx, 50)
		if err != nil {
			logger.Warn("outbox dispatch", slog.String("err", err.Error()))
		} else if sent > 0 {
			logger.Info("sent emails", slog.Int("count", sent))
		}
	}
	return nil
}
