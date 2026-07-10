package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// openTestDB opens a fresh in-memory SQLite DB with the required pragmas and the
// embedded schema applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", withPragmas("file:"+t.Name()+"?mode=memory&cache=shared"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.PingContext(context.Background()))
	require.NoError(t, applyMigrations(context.Background(), db))
	return db
}

// TestSQLiteMigrationRoundTrip applies the up migration, then the down, then up
// again, catching a malformed down migration before it bites a real rollback.
func TestSQLiteMigrationRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", withPragmas("file:roundtrip?mode=memory&cache=shared"))
	require.NoError(t, err)
	defer db.Close()

	up, err := migrationFS.ReadFile("migrations/0001_init.up.sql")
	require.NoError(t, err)
	down, err := migrationFS.ReadFile("migrations/0001_init.down.sql")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, string(up))
	require.NoError(t, err)

	var tables int
	countTables := `SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`
	require.NoError(t, db.QueryRowContext(ctx, countTables).Scan(&tables))
	require.Greater(t, tables, 10, "expected the full schema after up")

	_, err = db.ExecContext(ctx, string(down))
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(ctx, countTables).Scan(&tables))
	require.Equal(t, 0, tables, "down migration must drop every table")

	_, err = db.ExecContext(ctx, string(up))
	require.NoError(t, err, "re-applying up after down must succeed")
}

// TestSQLiteCategorySeed asserts the seed data landed with stable ids.
func TestSQLiteCategorySeed(t *testing.T) {
	db := openTestDB(t)
	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM categories`).Scan(&n))
	require.Equal(t, 50, n)
	var slug string
	require.NoError(t, db.QueryRow(`SELECT slug FROM categories WHERE label = 'Other'`).Scan(&slug))
	require.Equal(t, "other", slug)
}

// TestSQLiteGroupDeleteCascades verifies PRAGMA foreign_keys=ON is honored and
// deleting a group cascades to its group-scoped children (members, expenses ->
// splits, settlements, recurring, activity_events) while users survive.
func TestSQLiteGroupDeleteCascades(t *testing.T) {
	db := openTestDB(t)

	now := tsVal(time.Now().UTC())
	uid := uuid.New()
	uid2 := uuid.New()
	gid := uuid.New()
	eid := uuid.New()
	sid := uuid.New()
	rid := uuid.New()
	catID := mustCategoryID(t, db)

	mustExec(t, db, `INSERT INTO users (id, email_hash, email_encrypted, display_name, password_hash, created_at) VALUES (?, ?, ?, 'U', 'h', ?)`,
		uid, []byte("h"), []byte("e"), now)
	mustExec(t, db, `INSERT INTO users (id, email_hash, email_encrypted, display_name, password_hash, created_at) VALUES (?, ?, ?, 'U2', 'h', ?)`,
		uid2, []byte("h2"), []byte("e2"), now)
	mustExec(t, db, `INSERT INTO groups (id, name, created_by, created_at) VALUES (?, 'G', ?, ?)`, gid, uid, now)
	mustExec(t, db, `INSERT INTO group_members (group_id, user_id, joined_at) VALUES (?, ?, ?)`, gid, uid, now)
	mustExec(t, db, `INSERT INTO expenses (id, group_id, payer_id, created_by, amount_cents, description, incurred_at, category_id, created_at) VALUES (?, ?, ?, ?, 100, 'e', ?, ?, ?)`,
		eid, gid, uid, uid, now, catID, now)
	mustExec(t, db, `INSERT INTO splits (expense_id, user_id, share_cents) VALUES (?, ?, 100)`, eid, uid)
	mustExec(t, db, `INSERT INTO settlements (id, group_id, from_user, to_user, amount_cents, settled_at, created_at) VALUES (?, ?, ?, ?, 50, ?, ?)`,
		sid, gid, uid, uid2, now, now)
	mustExec(t, db, `INSERT INTO recurring_expenses (id, group_id, payer_id, amount_cents, description, mode, split_template, cadence, category_id, next_run_at, created_at) VALUES (?, ?, ?, 100, 'r', 'equal', '[]', 'monthly', ?, ?, ?)`,
		rid, gid, uid, catID, now, now)
	mustExec(t, db, `INSERT INTO activity_events (id, group_id, action, expense_id, metadata, created_at) VALUES (?, ?, 'expense.created', ?, '{}', ?)`,
		uuid.New(), gid, eid, now)

	mustExec(t, db, `DELETE FROM groups WHERE id = ?`, gid)

	for _, tbl := range []string{"group_members", "expenses", "splits", "settlements", "recurring_expenses", "activity_events"} {
		require.Equal(t, 0, count(t, db, `SELECT count(*) FROM `+tbl), "%s should cascade-delete with the group", tbl)
	}
	require.Equal(t, 2, count(t, db, `SELECT count(*) FROM users`), "users must NOT cascade on group delete")
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), q, args...)
	require.NoError(t, err)
}

func count(t *testing.T, db *sql.DB, q string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(q).Scan(&n))
	return n
}

func mustCategoryID(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, db.QueryRow(`SELECT id FROM categories LIMIT 1`).Scan(&id))
	return id
}
