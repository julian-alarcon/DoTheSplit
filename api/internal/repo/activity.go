package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

// ActivityEvent is the write model for one row in the append-only feed.
// ActorID is nil for worker/system actions. Exactly one of ExpenseID /
// SettlementID is set.
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

// ActivityHydrated is one fully-denormalized feed row, ready for the API
// without a second query. Expense/settlement payloads are joined in even when
// soft-deleted so `*.deleted` rows still resolve their description.
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
	// Settlement rows only: who paid whom. Nil for expense rows.
	FromUserID *uuid.UUID
	ToUserID   *uuid.UUID
}

type ActivityRepo struct {
	pool *pgxpool.Pool
}

func NewActivityRepo(p *pgxpool.Pool) *ActivityRepo { return &ActivityRepo{pool: p} }

// insertActivityEvent writes one event inside an existing transaction, so the
// event is atomic with the mutation that produced it. Package-local: callers
// are the expense/settlement repos.
func insertActivityEvent(ctx context.Context, tx pgx.Tx, ev ActivityEvent) error {
	meta := ev.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO activity_events (group_id, actor_id, action, expense_id, settlement_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, ev.GroupID, ev.ActorID, ev.Action, ev.ExpenseID, ev.SettlementID, b)
	return err
}

// expenseEventMeta builds the metadata for an expense event by reading the
// expense's category (slug + group label) and recurring flag inside the same
// tx. For an update this reflects the *new* category, which is what the feed
// should show. Caller must run this after applying the mutation.
func expenseEventMeta(ctx context.Context, tx pgx.Tx, expenseID uuid.UUID) (map[string]any, error) {
	var slug, groupLabel string
	var recurring bool
	err := tx.QueryRow(ctx, `
		SELECT c.slug, c.group_label, e.recurring_expense_id IS NOT NULL
		FROM expenses e JOIN categories c ON c.id = e.category_id
		WHERE e.id = $1
	`, expenseID).Scan(&slug, &groupLabel, &recurring)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"category_slug":        slug,
		"category_group_label": groupLabel,
		"recurring":            recurring,
	}, nil
}

// ListByGroup returns up to `limit` hydrated feed rows for the group, newest
// first (created_at DESC, id DESC), optionally continuing strictly after the
// cursor. Caller passes `limit + 1` to detect a next page. The expense /
// settlement joins deliberately omit the `deleted_at IS NULL` filter so
// `*.deleted` events still carry their target's description.
func (r *ActivityRepo) ListByGroup(ctx context.Context, groupID uuid.UUID, limit int, after *ActivityRow) ([]ActivityHydrated, error) {
	args := []any{groupID}
	cursorPredicate := ""
	if after != nil {
		args = append(args, after.CreatedAt, after.ID)
		cursorPredicate = "AND (ae.created_at, ae.id) < ($2, $3)"
	}
	limitArg := len(args) + 1
	args = append(args, limit)
	query := fmt.Sprintf(`
		SELECT
			ae.id,
			ae.action,
			ae.created_at,
			ae.actor_id,
			u.display_name,
			u.avatar_updated_at,
			COALESCE(ae.expense_id, ae.settlement_id) AS target_id,
			COALESCE(e.description, s.note, '') AS description,
			COALESCE(e.amount_cents, s.amount_cents, 0) AS amount_cents,
			COALESCE(e.currency, g.default_currency) AS currency,
			ae.metadata->>'category_slug' AS category_slug,
			ae.metadata->>'category_group_label' AS category_group_label,
			COALESCE((ae.metadata->>'recurring')::boolean, false) AS recurring,
			s.from_user AS from_user_id,
			s.to_user AS to_user_id
		FROM activity_events ae
		JOIN groups g ON g.id = ae.group_id
		LEFT JOIN users u ON u.id = ae.actor_id
		LEFT JOIN expenses e ON e.id = ae.expense_id
		LEFT JOIN settlements s ON s.id = ae.settlement_id
		WHERE ae.group_id = $1 %s
		ORDER BY ae.created_at DESC, ae.id DESC
		LIMIT $%d
	`, cursorPredicate, limitArg)
	query = strings.TrimSpace(query)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActivityHydrated
	for rows.Next() {
		var h ActivityHydrated
		var action string
		if err := rows.Scan(&h.ID, &action, &h.OccurredAt, &h.ActorID, &h.ActorName,
			&h.ActorAvatarUpdatedAt, &h.TargetID, &h.Description, &h.AmountCents,
			&h.Currency, &h.CategorySlug, &h.CategoryGroupLabel, &h.Recurring,
			&h.FromUserID, &h.ToUserID); err != nil {
			return nil, err
		}
		h.Action = ActivityAction(action)
		out = append(out, h)
	}
	return out, rows.Err()
}
