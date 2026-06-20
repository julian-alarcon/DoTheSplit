package repo_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestMigrationRoundTrip applies every committed *.up.sql, then every
// matching *.down.sql in reverse order, then re-applies the up files.
// Catches malformed down migrations before they bite during a real rollback.
func TestMigrationRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}

	ctx := context.Background()
	pool, _ := startPG(ctx, t)

	up := readMigrationsFiltered(t, ".up.sql", false)
	down := readMigrationsFiltered(t, ".down.sql", true)

	require.NotEmpty(t, up, "no .up.sql files found")
	require.Equal(t, len(up), len(down), "every up migration needs a matching down")

	for _, sql := range up {
		_, err := pool.Exec(ctx, sql)
		require.NoError(t, err, "applying up migration")
	}
	for _, sql := range down {
		_, err := pool.Exec(ctx, sql)
		require.NoError(t, err, "applying down migration")
	}
	// Schema must be empty after down.
	var n int
	err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public'
	`).Scan(&n)
	require.NoError(t, err)
	require.Equal(t, 0, n, "down migrations must drop all tables")

	// Re-apply ups: catches down migrations that don't drop everything they
	// should (causing the second up to fail with "relation already exists").
	for _, sql := range up {
		_, err := pool.Exec(ctx, sql)
		require.NoError(t, err, "re-applying up after down")
	}
}

// TestGroupDeleteCascades drives the FK CASCADE rules on groups(id):
// deleting a group must remove its members, expenses (with splits + revisions),
// settlements, and recurring expense templates. The user record stays so that
// other groups' ledgers (and their FK pointers from expenses) remain intact.
func TestGroupDeleteCascades(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}

	ctx := context.Background()
	pool, _ := startPG(ctx, t)

	for _, sql := range readMigrationsFiltered(t, ".up.sql", false) {
		_, err := pool.Exec(ctx, sql)
		require.NoError(t, err)
	}

	// Seed two users, one group, one expense (with one split + one revision),
	// one settlement, one recurring expense. We use raw SQL on purpose so this
	// test fails loudly if the cascade definition regresses, even when the
	// service layer changes shape.
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}

	var alice, bob string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (display_name, email_hash, email_encrypted, password_hash)
		VALUES ('Alice', $1, $2, 'argon2id$dummy') RETURNING id
	`, []byte("hash-a"), []byte("enc-a")).Scan(&alice))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (display_name, email_hash, email_encrypted, password_hash)
		VALUES ('Bob',   $1, $2, 'argon2id$dummy') RETURNING id
	`, []byte("hash-b"), []byte("enc-b")).Scan(&bob))

	var group string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO groups (name, created_by, default_currency)
		VALUES ('Trip', $1, 'EUR') RETURNING id
	`, alice).Scan(&group))
	exec(`INSERT INTO group_members (group_id, user_id) VALUES ($1, $2)`, group, alice)
	exec(`INSERT INTO group_members (group_id, user_id) VALUES ($1, $2)`, group, bob)

	var category string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT id FROM categories LIMIT 1
	`).Scan(&category))

	var expense string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO expenses (group_id, payer_id, created_by, category_id, amount_cents, currency, description, incurred_at)
		VALUES ($1, $2, $2, $3, 1000, 'EUR', 'dinner', now()) RETURNING id
	`, group, alice, category).Scan(&expense))
	exec(`INSERT INTO splits (expense_id, user_id, share_cents) VALUES ($1, $2, 500), ($1, $3, 500)`, expense, alice, bob)
	exec(`
		INSERT INTO expense_revisions (expense_id, edited_by, field, old_value, new_value)
		VALUES ($1, $2, 'description', 'old', 'new')
	`, expense, alice)
	exec(`
		INSERT INTO settlements (group_id, from_user, to_user, amount_cents, settled_at)
		VALUES ($1, $2, $3, 100, now())
	`, group, bob, alice)
	exec(`
		INSERT INTO recurring_expenses
		  (group_id, payer_id, category_id, amount_cents, currency, description, mode, split_template, cadence, next_run_at)
		VALUES ($1, $2, $3, 100, 'EUR', 'rent', 'equal', '[]'::jsonb, 'monthly', now() + interval '1 month')
	`, group, alice, category)

	// Sanity: rows exist before delete.
	mustCount(t, pool, ctx, 1, `SELECT count(*) FROM groups WHERE id = $1`, group)
	mustCount(t, pool, ctx, 2, `SELECT count(*) FROM group_members WHERE group_id = $1`, group)
	mustCount(t, pool, ctx, 1, `SELECT count(*) FROM expenses WHERE group_id = $1`, group)
	mustCount(t, pool, ctx, 2, `SELECT count(*) FROM splits WHERE expense_id = $1`, expense)
	mustCount(t, pool, ctx, 1, `SELECT count(*) FROM expense_revisions WHERE expense_id = $1`, expense)
	mustCount(t, pool, ctx, 1, `SELECT count(*) FROM settlements WHERE group_id = $1`, group)
	mustCount(t, pool, ctx, 1, `SELECT count(*) FROM recurring_expenses WHERE group_id = $1`, group)

	// Delete the group: every dependent row must vanish.
	exec(`DELETE FROM groups WHERE id = $1`, group)

	mustCount(t, pool, ctx, 0, `SELECT count(*) FROM group_members WHERE group_id = $1`, group)
	mustCount(t, pool, ctx, 0, `SELECT count(*) FROM expenses WHERE group_id = $1`, group)
	mustCount(t, pool, ctx, 0, `SELECT count(*) FROM splits WHERE expense_id = $1`, expense)
	mustCount(t, pool, ctx, 0, `SELECT count(*) FROM expense_revisions WHERE expense_id = $1`, expense)
	mustCount(t, pool, ctx, 0, `SELECT count(*) FROM settlements WHERE group_id = $1`, group)
	mustCount(t, pool, ctx, 0, `SELECT count(*) FROM recurring_expenses WHERE group_id = $1`, group)

	// Users must NOT cascade. Soft-delete invariant: ledger rows survive
	// account deletion via tombstoned users (see AGENTS.md "Account
	// invariants"); group deletion must not punch through to users either.
	mustCount(t, pool, ctx, 2, `SELECT count(*) FROM users WHERE id IN ($1, $2)`, alice, bob)
}

// startPG boots a Postgres testcontainer + pgxpool and registers cleanup.
func startPG(ctx context.Context, t *testing.T) (*pgxpool.Pool, testcontainers.Container) {
	t.Helper()
	pgc, err := tcpg.Run(ctx,
		"postgres:18-alpine",
		tcpg.WithDatabase("dts"),
		tcpg.WithUsername("dts"),
		tcpg.WithPassword("dts"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)

	dsn, err := pgc.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
		_ = pgc.Terminate(context.Background())
	})
	return pool, pgc
}

// readMigrationsFiltered returns each matching migration file's contents,
// sorted by filename. When reverse is true the slice is returned in reverse
// alphabetical order (used for *.down.sql so newer migrations roll back first).
func readMigrationsFiltered(t *testing.T, suffix string, reverse bool) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), suffix) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if reverse {
		for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
			names[i], names[j] = names[j], names[i]
		}
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		require.NoError(t, err)
		out = append(out, string(b))
	}
	return out
}

func mustCount(t *testing.T, pool *pgxpool.Pool, ctx context.Context, want int, sql string, args ...any) {
	t.Helper()
	var got int
	require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&got))
	require.Equal(t, want, got, sql)
}
