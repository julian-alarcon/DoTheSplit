package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type ActivityRepo struct{ s *Store }

// insertActivityEvent writes one feed row on the given tx AND buffers a signal
// for post-commit delivery to the in-process hub (the SQLite analogue of the
// Postgres NOTIFY trigger). It must run inside a transaction so the buffered
// signal is only flushed if the surrounding mutation commits.
func insertActivityEvent(ctx context.Context, tx repo.Tx, ev repo.ActivityEvent) error {
	meta := ev.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	id := uuid.New()
	now := ev.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	st := native(tx)
	if _, err := st.tx.ExecContext(ctx, `
		INSERT INTO activity_events (id, group_id, actor_id, action, expense_id, settlement_id, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, ev.GroupID, ev.ActorID, string(ev.Action), ev.ExpenseID, ev.SettlementID, string(b), tsVal(now)); err != nil {
		return err
	}
	recordSignal(tx, repo.ActivityEventSignal{
		ID:           id,
		GroupID:      ev.GroupID,
		ActorID:      ev.ActorID,
		Action:       string(ev.Action),
		ExpenseID:    ev.ExpenseID,
		SettlementID: ev.SettlementID,
		CreatedAt:    now,
	})
	return nil
}

// expenseEventMeta builds the metadata for an expense event (new category slug +
// group label + recurring flag), read on the same tx after the mutation.
func expenseEventMeta(ctx context.Context, q dbtx, expenseID uuid.UUID) (map[string]any, error) {
	var slug, groupLabel string
	var recurring bool
	err := q.QueryRowContext(ctx, `
		SELECT c.slug, c.group_label, e.recurring_expense_id IS NOT NULL
		FROM expenses e JOIN categories c ON c.id = e.category_id
		WHERE e.id = ?
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

// insertSettlementEvent writes a settlement activity event inside an existing tx.
// createdAt pins the feed row's timestamp when non-zero (CSV restore); pass the
// zero time for the normal path so the current time applies.
func insertSettlementEvent(ctx context.Context, tx repo.Tx, groupID, settlementID, actorID uuid.UUID, action repo.ActivityAction, createdAt time.Time) error {
	var actor *uuid.UUID
	if actorID != uuid.Nil {
		actor = &actorID
	}
	return insertActivityEvent(ctx, tx, repo.ActivityEvent{
		GroupID:      groupID,
		ActorID:      actor,
		Action:       action,
		SettlementID: &settlementID,
		CreatedAt:    createdAt,
	})
}

// ListByGroup returns up to `limit` hydrated feed rows, newest first, optionally
// continuing strictly after the cursor. Mirrors the Postgres query but with
// json_extract() for the metadata reads and a row-value keyset comparison.
func (r *ActivityRepo) ListByGroup(ctx context.Context, groupID uuid.UUID, limit int, after *repo.ActivityRow) ([]repo.ActivityHydrated, error) {
	args := []any{groupID}
	cursorPredicate := ""
	if after != nil {
		args = append(args, tsVal(after.CreatedAt), after.ID)
		cursorPredicate = "AND (ae.created_at, ae.id) < (?, ?)"
	}
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
			json_extract(ae.metadata, '$.category_slug') AS category_slug,
			json_extract(ae.metadata, '$.category_group_label') AS category_group_label,
			COALESCE(json_extract(ae.metadata, '$.recurring'), 0) AS recurring,
			s.from_user AS from_user_id,
			s.to_user AS to_user_id
		FROM activity_events ae
		JOIN groups g ON g.id = ae.group_id
		LEFT JOIN users u ON u.id = ae.actor_id
		LEFT JOIN expenses e ON e.id = ae.expense_id
		LEFT JOIN settlements s ON s.id = ae.settlement_id
		WHERE ae.group_id = ? %s
		ORDER BY ae.created_at DESC, ae.id DESC
		LIMIT ?
	`, cursorPredicate)
	query = strings.TrimSpace(query)
	rows, err := r.s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.ActivityHydrated
	for rows.Next() {
		var h repo.ActivityHydrated
		var action string
		var occurred string
		var actorAvatar *string
		var recurringInt int64
		if err := rows.Scan(&h.ID, &action, &occurred, &h.ActorID, &h.ActorName,
			&actorAvatar, &h.TargetID, &h.Description, &h.AmountCents,
			&h.Currency, &h.CategorySlug, &h.CategoryGroupLabel, &recurringInt,
			&h.FromUserID, &h.ToUserID); err != nil {
			return nil, err
		}
		h.Action = repo.ActivityAction(action)
		h.OccurredAt = scanTS(occurred)
		h.ActorAvatarUpdatedAt = scanTSPtr(actorAvatar)
		h.Recurring = recurringInt != 0
		out = append(out, h)
	}
	return out, rows.Err()
}

// inPlaceholders returns "?,?,..." for building IN clauses from a slice, plus the
// args as []any. Callers guard against an empty slice (SQLite rejects "IN ()").
func inPlaceholders[T any](items []T) (string, []any) {
	ph := make([]string, len(items))
	args := make([]any, len(items))
	for i, v := range items {
		ph[i] = "?"
		args[i] = v
	}
	return strings.Join(ph, ","), args
}

// SettlementCreators returns settlement_id -> actor_id for every
// settlement.created event in the group with a non-null actor.
func (r *ActivityRepo) SettlementCreators(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	rows, err := r.s.db.QueryContext(ctx, `
		SELECT settlement_id, actor_id
		FROM activity_events
		WHERE group_id = ? AND action = ? AND settlement_id IS NOT NULL AND actor_id IS NOT NULL
	`, groupID, string(repo.ActionSettlementCreated))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID]uuid.UUID)
	for rows.Next() {
		var sid, aid uuid.UUID
		if err := rows.Scan(&sid, &aid); err != nil {
			return nil, err
		}
		out[sid] = aid
	}
	return out, rows.Err()
}
