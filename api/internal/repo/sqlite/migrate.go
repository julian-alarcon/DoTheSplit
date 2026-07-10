package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

// migrationFS embeds the SQLite migration files. They are applied in-process at
// boot (there is no separate migrate container for SQLite): the DB is a file
// local to the process, so an external networked migrate step makes no sense.
// Both up and down files are embedded (down is used by the migration
// round-trip test).
//
//go:embed migrations/*.sql
var migrationFS embed.FS

// applyMigrations runs every embedded *.up.sql file, in filename order, inside a
// single transaction. It is idempotent-safe for a fresh database; re-running on
// an already-migrated database would error on CREATE TABLE, so callers gate it
// on a first-boot check (schemaExists).
func applyMigrations(ctx context.Context, db *sql.DB) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, name := range names {
		b, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(b)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return tx.Commit()
}

// schemaExists reports whether the core schema is already present (the users
// table), so boot can skip re-applying migrations on an existing database.
func schemaExists(ctx context.Context, db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
