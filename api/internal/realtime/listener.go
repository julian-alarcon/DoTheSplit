package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// notifyChannel must match the channel name used by the activity_events trigger
// (migration 0001).
const notifyChannel = "activity_events"

// RunListener holds one dedicated pool connection parked on LISTEN and forwards
// every notification to the hub. It blocks until ctx is cancelled, reconnecting
// with exponential backoff on any connection error so a Postgres restart or
// dropped connection self-heals. Run it in its own goroutine.
func RunListener(ctx context.Context, pool *pgxpool.Pool, hub *Hub, log *slog.Logger) {
	const (
		baseBackoff = 100 * time.Millisecond
		maxBackoff  = 5 * time.Second
	)
	backoff := baseBackoff
	for {
		if ctx.Err() != nil {
			return
		}
		if err := listenOnce(ctx, pool, hub, log, func() { backoff = baseBackoff }); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Warn("activity listener disconnected; retrying",
				slog.String("err", err.Error()),
				slog.Duration("backoff", backoff))
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff *= 2; backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// listenOnce acquires a connection, issues LISTEN, then loops on
// WaitForNotification until ctx is cancelled or an error occurs. onConnect is
// called once the LISTEN succeeds so the caller can reset its backoff. The
// connection is always released before returning.
func listenOnce(ctx context.Context, pool *pgxpool.Pool, hub *Hub, log *slog.Logger, onConnect func()) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+notifyChannel); err != nil {
		return err
	}
	onConnect()
	log.Info("activity listener connected", slog.String("channel", notifyChannel))

	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var ev Event
		if err := json.Unmarshal([]byte(n.Payload), &ev); err != nil {
			log.Warn("activity listener: bad payload", slog.String("err", err.Error()))
			continue
		}
		hub.Publish(ev)
	}
}
