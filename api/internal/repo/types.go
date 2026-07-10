package repo

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// User is the persisted account row. NotificationPrefs is the raw JSON blob;
// the service layer parses it into a typed projection.
type User struct {
	ID                uuid.UUID
	EmailHash         []byte
	EmailEncrypted    []byte
	DisplayName       string
	PasswordHash      string
	CreatedAt         time.Time
	DeletedAt         *time.Time
	Avatar            []byte
	AvatarUpdatedAt   *time.Time
	WeekStart         int16
	Role              string
	EmailVerifiedAt   *time.Time
	NotificationPrefs []byte
}

// RefreshToken is a rotating bearer refresh token (hash stored, never plaintext).
type RefreshToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TokenHash  []byte
	IssuedAt   time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *uuid.UUID
}

// Group is a split group. DefaultSplit pins a 2-member percentage split that
// prefills the create-expense form (nil = none). UnreadCount is populated only
// by member-scoped queries (ListForUser); zero otherwise.
type Group struct {
	ID              uuid.UUID
	Name            string
	DefaultCurrency string
	CreatedBy       uuid.UUID
	CreatedAt       time.Time
	DefaultSplit    []DefaultSplitEntry
	UnreadCount     int
}

type DefaultSplitEntry struct {
	UserID      uuid.UUID `json:"user_id"`
	BasisPoints int64     `json:"basis_points"`
}

// ScanDefaultSplit unmarshals the JSON default_split column. NULL/empty → nil.
func ScanDefaultSplit(raw []byte) ([]DefaultSplitEntry, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out []DefaultSplitEntry
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type GroupMember struct {
	GroupID         uuid.UUID
	UserID          uuid.UUID
	DisplayName     string
	JoinedAt        time.Time
	AvatarUpdatedAt *time.Time
	DeletedAt       *time.Time
}

// UpdateInput captures partial-update fields for a group. nil pointer = leave
// unchanged. For DefaultSplit: nil = unchanged, non-nil pointer to empty slice
// = clear, non-nil pointer to non-empty slice = replace.
type UpdateInput struct {
	Name            *string
	DefaultCurrency *string
	DefaultSplit    *[]DefaultSplitEntry
	CreatedBy       *uuid.UUID
}

// AdminGroupRow extends Group with cross-instance counts used by the admin listing.
type AdminGroupRow struct {
	Group
	MemberCount  int
	ExpenseCount int
}

type Category struct {
	ID         uuid.UUID
	Slug       string
	Label      string
	Sort       int
	GroupLabel string
}

// Expense is a ledger entry with its splits.
type Expense struct {
	ID                 uuid.UUID
	GroupID            uuid.UUID
	PayerID            uuid.UUID
	CreatedBy          uuid.UUID
	CategoryID         uuid.UUID
	AmountCents        int64
	Currency           string
	Description        string
	Notes              string
	IncurredAt         time.Time
	CreatedAt          time.Time
	DeletedAt          *time.Time
	RecurringExpenseID *uuid.UUID
	Splits             []Split
}

type Split struct {
	ExpenseID  uuid.UUID
	UserID     uuid.UUID
	ShareCents int64
}

type ExpenseRevision struct {
	ID        uuid.UUID
	ExpenseID uuid.UUID
	EditedBy  uuid.UUID
	EditedAt  time.Time
	Field     string
	OldValue  string
	NewValue  string
}

type Settlement struct {
	ID          uuid.UUID
	GroupID     uuid.UUID
	FromUser    uuid.UUID
	ToUser      uuid.UUID
	AmountCents int64
	Note        string
	SettledAt   time.Time
	CreatedAt   time.Time
	DeletedAt   *time.Time
}

// NetBalance is a per-user net position in a group. Positive = owed to them.
type NetBalance struct {
	UserID      uuid.UUID
	DisplayName string
	NetCents    int64
}

// SplitTemplateEntry is the JSON shape stored in recurring_expenses.split_template.
type SplitTemplateEntry struct {
	UserID uuid.UUID `json:"user_id"`
	Value  int64     `json:"value,omitempty"`
}

type RecurringExpense struct {
	ID            uuid.UUID
	GroupID       uuid.UUID
	PayerID       uuid.UUID
	CategoryID    uuid.UUID
	AmountCents   int64
	Currency      string
	Description   string
	Mode          string
	SplitTemplate []SplitTemplateEntry
	Cadence       string
	NextRunAt     time.Time
	CreatedAt     time.Time
	DeletedAt     *time.Time
}

// ActivityAction enumerates the append-only feed actions.
type ActivityAction string

const (
	ActionExpenseCreated     ActivityAction = "expense.created"
	ActionExpenseUpdated     ActivityAction = "expense.updated"
	ActionExpenseDeleted     ActivityAction = "expense.deleted"
	ActionExpenseRestored    ActivityAction = "expense.restored"
	ActionSettlementCreated  ActivityAction = "settlement.created"
	ActionSettlementUpdated  ActivityAction = "settlement.updated"
	ActionSettlementDeleted  ActivityAction = "settlement.deleted"
	ActionSettlementRestored ActivityAction = "settlement.restored"
)

// ActivityEvent is the write model for one row in the append-only feed. ActorID
// is nil for worker/system actions. Exactly one of ExpenseID/SettlementID is set.
type ActivityEvent struct {
	GroupID      uuid.UUID
	ActorID      *uuid.UUID
	Action       ActivityAction
	ExpenseID    *uuid.UUID
	SettlementID *uuid.UUID
	Metadata     map[string]any
}

// ActivityRow is the keyset cursor tuple: (created_at, id).
type ActivityRow struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// ActivityHydrated is one fully-denormalized feed row.
type ActivityHydrated struct {
	ID                   uuid.UUID
	Action               ActivityAction
	OccurredAt           time.Time
	ActorID              *uuid.UUID
	ActorName            *string
	ActorAvatarUpdatedAt *time.Time
	TargetID             uuid.UUID
	Description          string
	AmountCents          int64
	Currency             string
	CategorySlug         *string
	CategoryGroupLabel   *string
	Recurring            bool
	FromUserID           *uuid.UUID
	ToUserID             *uuid.UUID
}

// TransactionKind discriminates rows in the merged expense+settlement feed.
type TransactionKind string

const (
	TransactionExpense    TransactionKind = "expense"
	TransactionSettlement TransactionKind = "settlement"
)

// TransactionRow is a (kind, occurred_at, created_at, id) tuple for keyset
// pagination; the service hydrates full payloads in batch.
type TransactionRow struct {
	Kind       TransactionKind
	OccurredAt time.Time
	CreatedAt  time.Time
	ID         uuid.UUID
}

// SearchRow is a hit in the merged feed restricted to a query substring.
type SearchRow struct {
	Kind       TransactionKind
	GroupID    uuid.UUID
	OccurredAt time.Time
	CreatedAt  time.Time
	ID         uuid.UUID
}

// VerificationPurpose discriminates the email-verification token flows.
type VerificationPurpose string

const (
	PurposeRegister      VerificationPurpose = "register"
	PurposeChangeEmail   VerificationPurpose = "change_email"
	PurposePasswordReset VerificationPurpose = "password_reset"
)

// VerificationToken stores a hashed 6-digit code (never the code itself).
type VerificationToken struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Purpose      VerificationPurpose
	CodeHash     []byte
	NewEmailHash []byte
	NewEmailEnc  []byte
	Attempts     int16
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	CreatedAt    time.Time
}

type AuditEntry struct {
	ID            uuid.UUID
	ActorUserID   uuid.UUID
	TargetUserID  *uuid.UUID
	TargetGroupID *uuid.UUID
	Action        string
	IP            *string
	UserAgent     *string
	Success       bool
	Metadata      json.RawMessage
	CreatedAt     time.Time
}

// AuditFilter narrows a List query.
type AuditFilter struct {
	Action string
}

type SmtpConfig struct {
	Host              string
	Port              int
	Username          *string
	PasswordEncrypted []byte
	FromAddress       string
	TLSMode           string
	UpdatedAt         time.Time
	UpdatedBy         *uuid.UUID
}

// Setup is the single-row first-run install state.
type Setup struct {
	TokenHash        []byte
	TokenGeneratedAt time.Time
	CompletedAt      *time.Time
	CompletedBy      *uuid.UUID
}

// OutboxRow is a single queued outbound email.
type OutboxRow struct {
	ID            uuid.UUID
	ToEmailEnc    []byte
	Subject       string
	Body          string
	Template      string
	Attempts      int16
	LastError     *string
	SentAt        *time.Time
	NextAttemptAt time.Time
	CreatedAt     time.Time
}
