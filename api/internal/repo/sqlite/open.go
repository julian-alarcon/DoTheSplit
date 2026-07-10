package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

// requiredPragmas are appended to the DSN so every pooled connection gets them.
//   - foreign_keys(ON): SQLite enforces FKs only when enabled per-connection;
//     the group-delete cascade depends on it.
//   - journal_mode(WAL): readers don't block the single writer.
//   - busy_timeout(5000): block-and-retry briefly on lock contention instead of
//     failing immediately with SQLITE_BUSY.
//   - synchronous(NORMAL): safe with WAL, faster than FULL.
const requiredPragmas = "_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"

// Open builds a SQLite-backed repo.Store from a DSN (a file path, a
// "file:..." URI, or ":memory:"), applying the required pragmas, running the
// embedded migrations on first boot, and wiring the activity publisher.
//
// The connection pool is NOT capped to a single connection: several service
// flows issue a pool read while already holding a Begin transaction (e.g.
// auth.Register's pre-check), which would deadlock a one-connection pool. WAL
// mode lets readers run concurrently with the single writer, and busy_timeout
// serializes the rare writer-vs-writer overlap without erroring, so an
// uncapped small pool is both correct and deadlock-free for a single node.
func Open(ctx context.Context, dsn string, publisher repo.ActivityPublisher) (*Store, error) {
	db, err := sql.Open("sqlite", withPragmas(dsn))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	exists, err := schemaExists(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("check sqlite schema: %w", err)
	}
	if !exists {
		if err := applyMigrations(ctx, db); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("migrate sqlite: %w", err)
		}
	}

	return NewStore(db, publisher), nil
}

// withPragmas merges requiredPragmas into the DSN's query string. Works for
// bare paths ("dts.db"), file URIs ("file:/data/dts.db"), and in-memory DSNs.
func withPragmas(dsn string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + requiredPragmas
}
