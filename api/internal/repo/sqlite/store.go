// Package sqlite is the database/sql + modernc.org/sqlite implementation of the
// repo.Store contract. It mirrors repo/postgres but for a single-writer, file-
// backed engine: no row locking, no LISTEN/NOTIFY (activity events are published
// to the in-process hub after commit via an ActivityPublisher), timestamps are
// stored as RFC3339Nano UTC text, and UUIDs/JSON are stored as TEXT.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

// dbtx is the database/sql execution surface shared by *sql.DB and *sql.Tx, so a
// repo method can run either on the pool or inside a caller-owned transaction.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// sqliteTx wraps a *sql.Tx as an engine-neutral repo.Tx. It also buffers the
// activity-event signals produced during the transaction and flushes them to
// the store's publisher only after the underlying Commit succeeds - mirroring
// Postgres's commit-gated NOTIFY delivery so subscribers never see a rolled-back
// row.
type sqliteTx struct {
	tx      *sql.Tx
	store   *Store
	pending []repo.ActivityEventSignal
}

func (t *sqliteTx) Commit(ctx context.Context) error {
	if err := t.tx.Commit(); err != nil {
		return err
	}
	if t.store.publisher != nil {
		for _, ev := range t.pending {
			t.store.publisher.PublishCommitted(ev)
		}
	}
	t.pending = nil
	return nil
}

func (t *sqliteTx) Rollback(ctx context.Context) error {
	err := t.tx.Rollback()
	// A rollback (or a rollback after commit, which is the errcheck-satisfying
	// deferred call) discards any buffered signals.
	t.pending = nil
	if errors.Is(err, sql.ErrTxDone) {
		return nil
	}
	return err
}

// native unwraps a repo.Tx to its *sqliteTx. A nil or wrong-engine Tx panics
// with a clear message rather than a bare type-assertion failure.
func native(tx repo.Tx) *sqliteTx {
	if tx == nil {
		panic("sqlite: nil repo.Tx passed to a transaction-only method")
	}
	st, ok := tx.(*sqliteTx)
	if !ok {
		panic("sqlite: repo.Tx from a different engine passed to the sqlite store")
	}
	return st
}

// resolve returns the querier for an optional tx: the pool when tx is nil,
// otherwise the underlying *sql.Tx.
func (s *Store) resolve(tx repo.Tx) dbtx {
	if tx == nil {
		return s.db
	}
	return native(tx).tx
}

// isUniqueViolation reports whether err is a SQLite UNIQUE/PRIMARY KEY constraint
// violation, which the repos map to repo.ErrConflict. modernc surfaces these as
// error strings containing "UNIQUE constraint failed" / "constraint failed".
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed: UNIQUE")
}

// --- timestamp helpers -------------------------------------------------------
//
// SQLite has no native timestamp type and modernc does not scan a TEXT column
// into time.Time. We store every timestamp as RFC3339Nano in UTC so that lexical
// text ordering matches chronological ordering (required by the keyset-pagination
// tuple comparisons in transactions/activity/search).

const tsLayout = "2006-01-02T15:04:05.999999999Z07:00" // == time.RFC3339Nano

// tsVal formats a time for storage.
func tsVal(t time.Time) string { return t.UTC().Format(tsLayout) }

// scanTS parses a NOT NULL timestamp column value.
func scanTS(s string) time.Time {
	t, _ := time.Parse(tsLayout, s)
	return t.UTC()
}

// scanTSPtr converts a nullable timestamp string into *time.Time.
func scanTSPtr(s *string) *time.Time {
	if s == nil {
		return nil
	}
	t := scanTS(*s)
	return &t
}

// Store is the database/sql-backed repo.Store.
type Store struct {
	db        *sql.DB
	publisher repo.ActivityPublisher

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

// NewStore builds a Store over an open *sql.DB. publisher may be nil (activity
// events are then not fanned out, e.g. in a repo-only unit test).
func NewStore(db *sql.DB, publisher repo.ActivityPublisher) *Store {
	s := &Store{db: db, publisher: publisher}
	s.users = &UserRepo{s: s}
	s.refresh = &RefreshTokenRepo{s: s}
	s.groups = &GroupRepo{s: s}
	s.expenses = &ExpenseRepo{s: s}
	s.settlements = &SettlementRepo{s: s}
	s.balances = &BalanceRepo{s: s}
	s.recurring = &RecurringRepo{s: s}
	s.categories = &CategoryRepo{s: s}
	s.transactions = &TransactionRepo{s: s}
	s.activity = &ActivityRepo{s: s}
	s.search = &SearchRepo{s: s}
	s.audit = &AuditRepo{s: s}
	s.smtp = &SmtpRepo{s: s}
	s.setup = &SetupRepo{s: s}
	s.verification = &VerificationRepo{s: s}
	s.outbox = &EmailOutboxRepo{s: s}
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

// Begin opens a transaction wrapped as a repo.Tx.
func (s *Store) Begin(ctx context.Context) (repo.Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &sqliteTx{tx: tx, store: s}, nil
}

// LockBootstrap is a no-op on SQLite: the single writer already serializes the
// first-user registration, so no advisory lock is needed.
func (s *Store) LockBootstrap(ctx context.Context, tx repo.Tx) error {
	native(tx) // validate the caller passed a live sqlite tx
	return nil
}

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) Close() { _ = s.db.Close() }

// publish buffers an activity signal on the given tx so it is delivered to the
// hub only after commit. Shared by the expense/settlement repos.
func recordSignal(tx repo.Tx, ev repo.ActivityEventSignal) {
	native(tx).pending = append(native(tx).pending, ev)
}

// Compile-time assurance the sqlite Store satisfies the repo.Store contract.
var _ repo.Store = (*Store)(nil)
