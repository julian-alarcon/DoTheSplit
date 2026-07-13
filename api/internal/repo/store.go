package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Tx is an engine-neutral handle to an open transaction. Services that span
// multiple tables obtain one from Store.Begin and pass it back into the ...Tx
// repo methods. The concrete type (pgx.Tx or *sql.Tx wrapper) is hidden inside
// the engine package; repo methods type-assert it back to their native handle.
type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// ActivityEventSignal is the minimal, committed-only change signal the SQLite
// store forwards to the in-process realtime hub after a transaction commits.
// It mirrors the fields the Postgres NOTIFY trigger emits. Defined here (rather
// than importing internal/realtime) so the repo layer stays free of a realtime
// dependency; the sqlite store adapts it to realtime.Event at the seam.
type ActivityEventSignal struct {
	ID           uuid.UUID
	GroupID      uuid.UUID
	ActorID      *uuid.UUID
	Action       string
	ExpenseID    *uuid.UUID
	SettlementID *uuid.UUID
	CreatedAt    time.Time
}

// ActivityPublisher receives activity-event signals once they are durably
// committed. The Postgres store uses a no-op (its DB trigger + LISTEN handle
// fan-out); the SQLite store forwards to the in-process hub.
type ActivityPublisher interface {
	PublishCommitted(ev ActivityEventSignal)
}

// Store is the top-level persistence dependency injected into cmd/api and
// cmd/worker. It owns the connection(s), exposes every repository, and begins
// transactions. One concrete Store exists per engine (postgres, sqlite).
type Store interface {
	Users() UserRepo
	RefreshTokens() RefreshTokenRepo
	Groups() GroupRepo
	Expenses() ExpenseRepo
	Settlements() SettlementRepo
	Balances() BalanceRepo
	Recurring() RecurringRepo
	Categories() CategoryRepo
	Transactions() TransactionRepo
	Activity() ActivityRepo
	Search() SearchRepo
	Audit() AuditRepo
	Smtp() SmtpRepo
	Setup() SetupRepo
	Verification() VerificationRepo
	EmailOutbox() EmailOutboxRepo

	// Begin opens a transaction for multi-table service operations.
	Begin(ctx context.Context) (Tx, error)

	// LockBootstrap serializes concurrent first-user registration so exactly one
	// caller observes count==0 and becomes the bootstrap admin. On Postgres this
	// is pg_advisory_xact_lock; on SQLite (single writer) it is a no-op. Must be
	// called inside the supplied transaction.
	LockBootstrap(ctx context.Context, tx Tx) error

	// Ping verifies connectivity for the health probe.
	Ping(ctx context.Context) error

	// Close releases all connections.
	Close()
}

// UserRepo persists accounts. The q Querier-style params of the original repos
// are replaced by an optional Tx: pass nil to run on the pool, or a Tx to
// participate in a caller-owned transaction.
type UserRepo interface {
	Create(ctx context.Context, u *User) error
	FindByEmailHash(ctx context.Context, emailHash []byte) (*User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdateDisplayName(ctx context.Context, id uuid.UUID, name string) error
	UpdateWeekStart(ctx context.Context, id uuid.UUID, v int16) error
	UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error
	// UpdatePasswordHashTx rotates the hash inside a caller-owned transaction
	// (used by admin reset, which must scramble atomically with the reset email).
	UpdatePasswordHashTx(ctx context.Context, tx Tx, id uuid.UUID, hash string) error
	SetAvatar(ctx context.Context, id uuid.UUID, png []byte) error
	GetAvatar(ctx context.Context, id uuid.UUID) ([]byte, error)
	CountActive(ctx context.Context, tx Tx) (int, error)
	CountActiveAdmins(ctx context.Context) (int, error)
	SetRole(ctx context.Context, id uuid.UUID, role string) error
	ListPaginated(ctx context.Context, limit, offset int, includeDeleted bool) ([]User, int, error)
	CreateWithRole(ctx context.Context, tx Tx, u *User, role string) error
	MarkEmailVerified(ctx context.Context, tx Tx, id uuid.UUID) error
	// UpdateEmail returns ErrConflict on an active-email uniqueness violation.
	UpdateEmail(ctx context.Context, tx Tx, id uuid.UUID, emailHash, emailEnc []byte) error
	UpdateNotificationPrefs(ctx context.Context, id uuid.UUID, prefs []byte) error
	FindOrCreateStub(ctx context.Context, tx Tx, emailHash, emailEnc []byte, displayName, scrambledPwHash string) (*User, error)
	SoftDelete(ctx context.Context, id uuid.UUID, tombstone string, scrambledHash, scrambledEnc []byte, scrambledPwHash string) error
}

type RefreshTokenRepo interface {
	Create(ctx context.Context, t *RefreshToken) error
	FindByTokenHash(ctx context.Context, tokenHash []byte) (*RefreshToken, error)
	Rotate(ctx context.Context, oldID uuid.UUID, next *RefreshToken) (*RefreshToken, error)
	RevokeByTokenHash(ctx context.Context, tokenHash []byte) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}

type GroupRepo interface {
	Create(ctx context.Context, name, defaultCurrency string, creatorID uuid.UUID) (*Group, error)
	CreateTx(ctx context.Context, tx Tx, name, defaultCurrency string, creatorID uuid.UUID) (*Group, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Group, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]Group, error)
	MarkActivityRead(ctx context.Context, groupID, userID uuid.UUID) error
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Group, error)
	ClearDefaultSplit(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListAll(ctx context.Context, limit, offset int) ([]AdminGroupRow, int, error)
	ListMembers(ctx context.Context, groupID uuid.UUID) ([]GroupMember, error)
	IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error)
	HasTransactions(ctx context.Context, groupID uuid.UUID) (bool, error)
	ShareAnyGroup(ctx context.Context, a, b uuid.UUID) (bool, error)
	RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error
	AddMemberTx(ctx context.Context, tx Tx, groupID, userID uuid.UUID) error
	AddMember(ctx context.Context, groupID, userID uuid.UUID) (*GroupMember, error)
}

type ExpenseRepo interface {
	CreateWithSplits(ctx context.Context, e *Expense) error
	CreateWithSplitsTx(ctx context.Context, tx Tx, e *Expense) error
	ListByGroup(ctx context.Context, groupID uuid.UUID) ([]Expense, error)
	FindByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*Expense, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Expense, error)
	SoftDelete(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error
	Restore(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error
	Update(ctx context.Context, id, editorID uuid.UUID, description *string, amountCents *int64,
		categoryID *uuid.UUID, payerID *uuid.UUID, incurredAt *time.Time, notes *string, newSplits []Split) (*Expense, error)
	ListRevisions(ctx context.Context, expenseID uuid.UUID) ([]ExpenseRevision, error)
}

type SettlementRepo interface {
	Create(ctx context.Context, s *Settlement, actorID uuid.UUID) error
	CreateTx(ctx context.Context, tx Tx, s *Settlement, actorID uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (*Settlement, error)
	Update(ctx context.Context, s *Settlement, actorID uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error
	Restore(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error
	FindByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Settlement, error)
	ListByGroup(ctx context.Context, groupID uuid.UUID) ([]Settlement, error)
}

type BalanceRepo interface {
	NetBalances(ctx context.Context, groupID uuid.UUID) ([]NetBalance, error)
	NetForUser(ctx context.Context, groupID, userID uuid.UUID) (int64, error)
}

type RecurringRepo interface {
	Create(ctx context.Context, e *RecurringExpense) error
	ListByGroup(ctx context.Context, groupID uuid.UUID) ([]RecurringExpense, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (*RecurringExpense, error)
	CadenceByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
	// ClaimDue returns due templates inside an open transaction the caller MUST
	// commit or roll back. On Postgres the rows are locked FOR NO KEY UPDATE
	// SKIP LOCKED; on SQLite the single writer needs no lock clause.
	ClaimDue(ctx context.Context, limit int) (Tx, []RecurringExpense, error)
	UpdateNextRunTx(ctx context.Context, tx Tx, id uuid.UUID, nextRunAt time.Time) error
}

type CategoryRepo interface {
	List(ctx context.Context) ([]Category, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Category, error)
	FindBySlug(ctx context.Context, slug string) (*Category, error)
}

type TransactionRepo interface {
	ListByGroup(ctx context.Context, groupID uuid.UUID, limit int, after *TransactionRow) ([]TransactionRow, error)
}

type ActivityRepo interface {
	ListByGroup(ctx context.Context, groupID uuid.UUID, limit int, after *ActivityRow) ([]ActivityHydrated, error)
	// SettlementCreators returns settlement_id -> actor_id for every
	// settlement.created event in the group that has a non-null actor. Used by
	// the CSV exporter to fill the settlement CreatedBy column (settlements have
	// no creator of their own; it lives only on the create event).
	SettlementCreators(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]uuid.UUID, error)
}

type SearchRepo interface {
	SearchTransactions(ctx context.Context, groupIDs []uuid.UUID, q string, categoryID *uuid.UUID, limit int) ([]SearchRow, error)
	AvailableCategories(ctx context.Context, groupIDs []uuid.UUID, q string) ([]uuid.UUID, error)
}

type AuditRepo interface {
	Insert(ctx context.Context, tx Tx, e *AuditEntry) error
	List(ctx context.Context, f AuditFilter, limit, offset int) ([]AuditEntry, int, error)
}

type SmtpRepo interface {
	Get(ctx context.Context) (*SmtpConfig, error)
	Upsert(ctx context.Context, c *SmtpConfig, leavePassword bool) error
}

type SetupRepo interface {
	Get(ctx context.Context) (*Setup, error)
	// GetForUpdate reads the single install row inside tx, locking it against a
	// racing completer (FOR UPDATE on Postgres; single-writer no-op on SQLite).
	// Returns ErrNotFound when no row exists yet.
	GetForUpdate(ctx context.Context, tx Tx) (*Setup, error)
	Upsert(ctx context.Context, hash []byte, at time.Time) error
	Complete(ctx context.Context, tx Tx, by uuid.UUID) error
	Locked(ctx context.Context) (bool, error)
}

type VerificationRepo interface {
	Insert(ctx context.Context, tx Tx, t *VerificationToken) error
	FindActive(ctx context.Context, userID uuid.UUID, purpose VerificationPurpose) (*VerificationToken, error)
	Consume(ctx context.Context, tx Tx, id uuid.UUID) error
	IncrementAttempts(ctx context.Context, id uuid.UUID) error
	InvalidateAll(ctx context.Context, tx Tx, userID uuid.UUID, purpose VerificationPurpose) error
}

type EmailOutboxRepo interface {
	Enqueue(ctx context.Context, tx Tx, row *OutboxRow) error
	ClaimDue(ctx context.Context, limit int) ([]OutboxRow, error)
	MarkSent(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, lastErr string, retryAfter time.Duration) error
}
