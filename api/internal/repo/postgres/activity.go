package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type ActivityRepo struct{ pool *pgxpool.Pool }

// insertActivityEvent writes one feed row inside an existing transaction so the
// event is atomic with the mutation that produced it.
func insertActivityEvent(ctx context.Context, q dbtx, ev repo.ActivityEvent) error {
	meta := ev.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	var createdAt *time.Time
	if !ev.CreatedAt.IsZero() {
		createdAt = &ev.CreatedAt
	}
	_, err = q.Exec(ctx, `
		INSERT INTO activity_events (group_id, actor_id, action, expense_id, settlement_id, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, now()))
	`, ev.GroupID, ev.ActorID, string(ev.Action), ev.ExpenseID, ev.SettlementID, b, createdAt)
	return err
}

// expenseEventMeta builds the metadata for an expense event by reading the
// expense's category (slug + group label) and recurring flag inside the same
// tx. For an update this reflects the new category. Run after the mutation.
func expenseEventMeta(ctx context.Context, q dbtx, expenseID uuid.UUID) (map[string]any, error) {
	var slug, groupLabel string
	var recurring bool
	err := q.QueryRow(ctx, `
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

// insertSettlementEvent writes a settlement activity event inside an existing tx.
// createdAt pins the feed row's timestamp when non-zero (CSV restore); pass the
// zero time for the normal path so the DB default applies.
func insertSettlementEvent(ctx context.Context, q dbtx, groupID, settlementID, actorID uuid.UUID, action repo.ActivityAction, createdAt time.Time) error {
	var actor *uuid.UUID
	if actorID != uuid.Nil {
		actor = &actorID
	}
	return insertActivityEvent(ctx, q, repo.ActivityEvent{
		GroupID:      groupID,
		ActorID:      actor,
		Action:       action,
		SettlementID: &settlementID,
		CreatedAt:    createdAt,
	})
}

// ListByGroup returns up to `limit` hydrated feed rows, newest first, optionally
// continuing strictly after the cursor. The expense/settlement joins omit the
// deleted_at filter so `*.deleted` events still carry their target's description.
func (r *ActivityRepo) ListByGroup(ctx context.Context, groupID uuid.UUID, limit int, after *repo.ActivityRow) ([]repo.ActivityHydrated, error) {
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
	var out []repo.ActivityHydrated
	for rows.Next() {
		var h repo.ActivityHydrated
		var action string
		if err := rows.Scan(&h.ID, &action, &h.OccurredAt, &h.ActorID, &h.ActorName,
			&h.ActorAvatarUpdatedAt, &h.TargetID, &h.Description, &h.AmountCents,
			&h.Currency, &h.CategorySlug, &h.CategoryGroupLabel, &h.Recurring,
			&h.FromUserID, &h.ToUserID); err != nil {
			return nil, err
		}
		h.Action = repo.ActivityAction(action)
		out = append(out, h)
	}
	return out, rows.Err()
}

// SettlementCreators returns settlement_id -> actor_id for every
// settlement.created event in the group with a non-null actor.
func (r *ActivityRepo) SettlementCreators(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT settlement_id, actor_id
		FROM activity_events
		WHERE group_id = $1 AND action = $2 AND settlement_id IS NOT NULL AND actor_id IS NOT NULL
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
