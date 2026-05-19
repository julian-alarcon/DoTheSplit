// Package repo holds Postgres repositories and shared DB types.
package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned by repo methods when no row matches.
var ErrNotFound = errors.New("not found")

// Querier is the subset of pgx execution methods used by repository functions
// that need to optionally participate in an external transaction. A nil
// Querier means "use the pool directly". Both *pgxpool.Pool and pgx.Tx
// satisfy it.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type poolQuerier struct{ pool *pgxpool.Pool }

func (p poolQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return p.pool.Exec(ctx, sql, args...)
}
func (p poolQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.pool.Query(ctx, sql, args...)
}
func (p poolQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}
