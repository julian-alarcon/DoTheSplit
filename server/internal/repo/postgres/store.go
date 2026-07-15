// Package postgres is the pgx/v5 implementation of the repo.Store contract.
// SQL text is currently hand-written pgx; Phase 0b swaps the internals to
// sqlc-generated code behind these same types without touching the boundary.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

// dbtx is the pgx execution surface shared by *pgxpool.Pool and pgx.Tx, so a
// repo method can run either on the pool or inside a caller-owned transaction.
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// pgTx wraps a pgx.Tx as an engine-neutral repo.Tx.
type pgTx struct{ tx pgx.Tx }

func (t *pgTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *pgTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

// native unwraps a repo.Tx back to its pgx.Tx. A nil or wrong-engine Tx panics
// with a clear message rather than a bare type-assertion panic.
func native(tx repo.Tx) pgx.Tx {
	if tx == nil {
		panic("postgres: nil repo.Tx passed to a transaction-only method")
	}
	pt, ok := tx.(*pgTx)
	if !ok {
		panic("postgres: repo.Tx from a different engine passed to the postgres store")
	}
	return pt.tx
}

// resolve returns the querier for an optional tx: the pool when tx is nil,
// otherwise the underlying pgx.Tx. Mirrors the old "nil Querier = use pool".
func resolve(pool *pgxpool.Pool, tx repo.Tx) dbtx {
	if tx == nil {
		return pool
	}
	return native(tx)
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505), which the repos map to repo.ErrConflict.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Store is the pgx-backed repo.Store. It holds the request pool and one
// long-lived repo instance per table.
type Store struct {
	pool *pgxpool.Pool

	users        *UserRepo
	refresh      *RefreshTokenRepo
	groups       *GroupRepo
	expenses     *ExpenseRepo
	settlements  *SettlementRepo
	balances     *BalanceRepo
	recurring    *RecurringRepo
	categories   *CategoryRepo
	transactions *TransactionRepo
	activity     *ActivityRepo
	search       *SearchRepo
	audit        *AuditRepo
	smtp         *SmtpRepo
	setup        *SetupRepo
	verification *VerificationRepo
	outbox       *EmailOutboxRepo
}

// NewStore builds a Store over an existing pgx pool.
func NewStore(pool *pgxpool.Pool) *Store {
	s := &Store{pool: pool}
	s.users = &UserRepo{pool: pool}
	s.refresh = &RefreshTokenRepo{pool: pool}
	s.groups = &GroupRepo{pool: pool}
	s.expenses = &ExpenseRepo{pool: pool}
	s.settlements = &SettlementRepo{pool: pool}
	s.balances = &BalanceRepo{pool: pool}
	s.recurring = &RecurringRepo{pool: pool}
	s.categories = &CategoryRepo{pool: pool}
	s.transactions = &TransactionRepo{pool: pool}
	s.activity = &ActivityRepo{pool: pool}
	s.search = &SearchRepo{pool: pool}
	s.audit = &AuditRepo{pool: pool}
	s.smtp = &SmtpRepo{pool: pool}
	s.setup = &SetupRepo{pool: pool}
	s.verification = &VerificationRepo{pool: pool}
	s.outbox = &EmailOutboxRepo{pool: pool}
	return s
}

func (s *Store) Users() repo.UserRepo                 { return s.users }
func (s *Store) RefreshTokens() repo.RefreshTokenRepo { return s.refresh }
func (s *Store) Groups() repo.GroupRepo               { return s.groups }
func (s *Store) Expenses() repo.ExpenseRepo           { return s.expenses }
func (s *Store) Settlements() repo.SettlementRepo     { return s.settlements }
func (s *Store) Balances() repo.BalanceRepo           { return s.balances }
func (s *Store) Recurring() repo.RecurringRepo        { return s.recurring }
func (s *Store) Categories() repo.CategoryRepo        { return s.categories }
func (s *Store) Transactions() repo.TransactionRepo   { return s.transactions }
func (s *Store) Activity() repo.ActivityRepo          { return s.activity }
func (s *Store) Search() repo.SearchRepo              { return s.search }
func (s *Store) Audit() repo.AuditRepo                { return s.audit }
func (s *Store) Smtp() repo.SmtpRepo                  { return s.smtp }
func (s *Store) Setup() repo.SetupRepo                { return s.setup }
func (s *Store) Verification() repo.VerificationRepo  { return s.verification }
func (s *Store) EmailOutbox() repo.EmailOutboxRepo    { return s.outbox }

// Begin opens a pgx transaction wrapped as a repo.Tx.
func (s *Store) Begin(ctx context.Context) (repo.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &pgTx{tx: tx}, nil
}

// LockBootstrap takes the advisory lock that serializes first-user registration.
func (s *Store) LockBootstrap(ctx context.Context, tx repo.Tx) error {
	_, err := native(tx).Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('admin_bootstrap'))`)
	return err
}

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) Close() { s.pool.Close() }

// Compile-time assurance the postgres Store satisfies the repo.Store contract.
var _ repo.Store = (*Store)(nil)
