// Package repo defines the engine-neutral persistence contract: domain types,
// repository interfaces, and the Store/Tx abstraction. Concrete implementations
// live in repo/postgres (pgx/v5) and repo/sqlite (database/sql + modernc). No
// SQL lives in this package.
package repo

import "errors"

// ErrNotFound is returned by repo methods when no row matches.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a write violates a uniqueness constraint (e.g.
// re-using an active email). Each engine translates its native unique-violation
// error into this sentinel at the repo boundary so services stay engine-neutral
// (Postgres surfaces pgconn.PgError 23505; SQLite surfaces a constraint error).
var ErrConflict = errors.New("conflict")
