// Package storefactory constructs the engine-specific repo.Store (Postgres or
// SQLite) from config and wires the realtime fan-out. It is the single place
// that knows which engine is active, so cmd/server and the test harness stay
// engine-agnostic.
package storefactory

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/server/internal/config"
	"github.com/julian-alarcon/dothesplit/server/internal/realtime"
	"github.com/julian-alarcon/dothesplit/server/internal/repo"
	pgstore "github.com/julian-alarcon/dothesplit/server/internal/repo/postgres"
	litestore "github.com/julian-alarcon/dothesplit/server/internal/repo/sqlite"
)

// Result bundles the constructed store with the realtime wiring the caller must
// honor. Exactly one delivery mechanism is active per engine:
//   - Postgres: a DB trigger emits NOTIFY; the caller must run RunListener on
//     ListenerPool to feed the hub. UsesListener is true.
//   - SQLite: the store publishes committed activity events straight to the hub;
//     no listener runs. UsesListener is false and ListenerPool is nil.
type Result struct {
	Store repo.Store
	Hub   *realtime.Hub
	// ListenerPool is the dedicated single-connection Postgres pool for the
	// LISTEN loop. Nil on SQLite. The caller owns closing it.
	ListenerPool *pgxpool.Pool
	// UsesListener reports whether the caller should start realtime.RunListener.
	UsesListener bool
	closeFn      func()
}

// Close releases engine resources (pools / DB handle). Safe to call once.
func (r *Result) Close() {
	if r.closeFn != nil {
		r.closeFn()
	}
}

// hubPublisher adapts the in-process hub to repo.ActivityPublisher for the
// SQLite path, converting the engine-neutral signal to a realtime.Event.
type hubPublisher struct{ hub *realtime.Hub }

func (p hubPublisher) PublishCommitted(ev repo.ActivityEventSignal) {
	p.hub.Publish(realtime.Event{
		ID:           ev.ID,
		GroupID:      ev.GroupID,
		ActorID:      ev.ActorID,
		Action:       ev.Action,
		ExpenseID:    ev.ExpenseID,
		SettlementID: ev.SettlementID,
		CreatedAt:    ev.CreatedAt,
	})
}

// Open builds the store + realtime wiring for the configured engine.
func Open(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*Result, error) {
	hub := realtime.NewHub()
	switch cfg.DatabaseDriver {
	case "sqlite":
		return openSQLite(ctx, cfg, hub)
	default:
		return openPostgres(ctx, cfg, hub, logger)
	}
}

func openPostgres(ctx context.Context, cfg *config.Config, hub *realtime.Hub, logger *slog.Logger) (*Result, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgx pool: %w", err)
	}

	// Dedicated single-connection pool for the realtime LISTEN so the parked
	// connection never shrinks the request pool.
	listenerCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("listener pool config: %w", err)
	}
	listenerCfg.MaxConns = 1
	listenerCfg.MinConns = 1
	listenerPool, err := pgxpool.NewWithConfig(ctx, listenerCfg)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("listener pool: %w", err)
	}

	store := pgstore.NewStore(pool)
	return &Result{
		Store:        store,
		Hub:          hub,
		ListenerPool: listenerPool,
		UsesListener: true,
		closeFn: func() {
			listenerPool.Close()
			store.Close()
		},
	}, nil
}

func openSQLite(ctx context.Context, cfg *config.Config, hub *realtime.Hub) (*Result, error) {
	store, err := litestore.Open(ctx, cfg.DatabaseURL, hubPublisher{hub})
	if err != nil {
		return nil, err
	}
	return &Result{
		Store:        store,
		Hub:          hub,
		UsesListener: false,
		closeFn:      store.Close,
	}, nil
}
