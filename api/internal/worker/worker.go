// Package worker runs the periodic background tick (materialize due recurring
// expenses, drain the email outbox). It is shared by the standalone worker
// binary (cmd/worker) and the embedded in-process worker the api starts when
// WORKER_MODE=embedded (always the case on SQLite).
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// TickLocker is an optional capability a Store may implement to serialize ticks
// across multiple worker processes. Postgres implements it with a session
// advisory lock; SQLite does not (single node, single writer) and the worker
// simply ticks unguarded.
type TickLocker interface {
	// TryLockTick attempts to acquire the singleton tick lock. It returns
	// ok=false (without error) when another process holds it. When ok=true the
	// returned release func must be called after the tick.
	TryLockTick(ctx context.Context) (ok bool, release func(), err error)
}

// Run drives the tick loop until ctx is cancelled. interval is the tick period
// (60s in production). Recurring is required; mailer may be nil (no outbox
// drain). If store implements TickLocker, each tick is guarded by it.
func Run(ctx context.Context, store repo.Store, recurring *service.RecurringService, mailer *service.MailerService, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	logger.Info("worker started")
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopping")
			return
		case <-ticker.C:
			if err := RunOnce(ctx, store, recurring, mailer, logger); err != nil {
				logger.Error("tick", slog.String("err", err.Error()))
			}
		}
	}
}

// RunOnce performs a single tick: acquire the optional lock, materialize due
// recurring expenses, then drain the outbox.
func RunOnce(ctx context.Context, store repo.Store, recurring *service.RecurringService, mailer *service.MailerService, logger *slog.Logger) error {
	if locker, ok := store.(TickLocker); ok {
		got, release, err := locker.TryLockTick(ctx)
		if err != nil {
			return err
		}
		if !got {
			logger.Debug("another worker holds the tick lock; skipping")
			return nil
		}
		defer release()
	}

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
