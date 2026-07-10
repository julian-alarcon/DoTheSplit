package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type ExpenseRepo struct{ pool *pgxpool.Pool }

const (
	expenseCols            = `id, group_id, payer_id, created_by, category_id, amount_cents, currency, description, notes, incurred_at, created_at, recurring_expense_id`
	expenseColsWithDeleted = `id, group_id, payer_id, created_by, category_id, amount_cents, currency, description, notes, incurred_at, created_at, deleted_at, recurring_expense_id`
)

// CreateWithSplits inserts an expense and its splits atomically in its own tx.
func (r *ExpenseRepo) CreateWithSplits(ctx context.Context, e *repo.Expense) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.createWithSplits(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CreateWithSplitsTx inserts on the caller-owned transaction.
func (r *ExpenseRepo) CreateWithSplitsTx(ctx context.Context, tx repo.Tx, e *repo.Expense) error {
	return r.createWithSplits(ctx, native(tx), e)
}

func (r *ExpenseRepo) createWithSplits(ctx context.Context, q dbtx, e *repo.Expense) error {
	err := q.QueryRow(ctx, `
		INSERT INTO expenses (group_id, payer_id, created_by, category_id, amount_cents, currency, description, notes, incurred_at, recurring_expense_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`, e.GroupID, e.PayerID, e.CreatedBy, e.CategoryID, e.AmountCents, e.Currency, e.Description, e.Notes, e.IncurredAt, e.RecurringExpenseID).
		Scan(&e.ID, &e.CreatedAt)
	if err != nil {
		return err
	}
	for i := range e.Splits {
		e.Splits[i].ExpenseID = e.ID
		if _, err := q.Exec(ctx, `
			INSERT INTO splits (expense_id, user_id, share_cents) VALUES ($1, $2, $3)
		`, e.ID, e.Splits[i].UserID, e.Splits[i].ShareCents); err != nil {
			return err
		}
	}
	meta, err := expenseEventMeta(ctx, q, e.ID)
	if err != nil {
		return err
	}
	if e.RecurringExpenseID != nil {
		meta["recurring_expense_id"] = e.RecurringExpenseID.String()
	}
	var actor *uuid.UUID
	if e.CreatedBy != uuid.Nil {
		actor = &e.CreatedBy
	}
	if e.RecurringExpenseID != nil {
		// Worker-generated: system actor (NULL) so the feed shows "Recurring".
		actor = nil
	}
	return insertActivityEvent(ctx, q, repo.ActivityEvent{
		GroupID:   e.GroupID,
		ActorID:   actor,
		Action:    repo.ActionExpenseCreated,
		ExpenseID: &e.ID,
		Metadata:  meta,
	})
}

// ListByGroup returns non-deleted expenses with their splits, newest first.
func (r *ExpenseRepo) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]repo.Expense, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+expenseCols+`
		FROM expenses
		WHERE group_id = $1 AND deleted_at IS NULL
		ORDER BY incurred_at DESC, created_at DESC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var exps []repo.Expense
	for rows.Next() {
		var e repo.Expense
		if err := rows.Scan(&e.ID, &e.GroupID, &e.PayerID, &e.CreatedBy, &e.CategoryID, &e.AmountCents,
			&e.Currency, &e.Description, &e.Notes, &e.IncurredAt, &e.CreatedAt, &e.RecurringExpenseID); err != nil {
			return nil, err
		}
		exps = append(exps, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(exps) == 0 {
		return exps, nil
	}
	ids := make([]uuid.UUID, len(exps))
	for i, e := range exps {
		ids[i] = e.ID
	}
	splitsByExpense, err := r.splitsFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range exps {
		exps[i].Splits = splitsByExpense[exps[i].ID]
	}
	return exps, nil
}

// FindByIDs returns non-deleted expenses (with splits) keyed by id.
func (r *ExpenseRepo) FindByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*repo.Expense, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]*repo.Expense{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+expenseCols+`
		FROM expenses
		WHERE id = ANY($1) AND deleted_at IS NULL
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID]*repo.Expense, len(ids))
	for rows.Next() {
		var e repo.Expense
		if err := rows.Scan(&e.ID, &e.GroupID, &e.PayerID, &e.CreatedBy, &e.CategoryID, &e.AmountCents,
			&e.Currency, &e.Description, &e.Notes, &e.IncurredAt, &e.CreatedAt, &e.RecurringExpenseID); err != nil {
			return nil, err
		}
		ecopy := e
		out[e.ID] = &ecopy
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	splitsByExpense, err := r.splitsFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	for id, ss := range splitsByExpense {
		if e, ok := out[id]; ok {
			e.Splits = ss
		}
	}
	return out, nil
}

// splitsFor batch-loads splits for the given expense ids.
func (r *ExpenseRepo) splitsFor(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]repo.Split, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT expense_id, user_id, share_cents FROM splits WHERE expense_id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID][]repo.Split{}
	for rows.Next() {
		var s repo.Split
		if err := rows.Scan(&s.ExpenseID, &s.UserID, &s.ShareCents); err != nil {
			return nil, err
		}
		out[s.ExpenseID] = append(out[s.ExpenseID], s)
	}
	return out, rows.Err()
}

func (r *ExpenseRepo) FindByID(ctx context.Context, id uuid.UUID) (*repo.Expense, error) {
	var e repo.Expense
	err := r.pool.QueryRow(ctx, `
		SELECT `+expenseColsWithDeleted+`
		FROM expenses WHERE id = $1
	`, id).Scan(&e.ID, &e.GroupID, &e.PayerID, &e.CreatedBy, &e.CategoryID, &e.AmountCents, &e.Currency,
		&e.Description, &e.Notes, &e.IncurredAt, &e.CreatedAt, &e.DeletedAt, &e.RecurringExpenseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	srows, err := r.pool.Query(ctx, `
		SELECT expense_id, user_id, share_cents FROM splits WHERE expense_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var s repo.Split
		if err := srows.Scan(&s.ExpenseID, &s.UserID, &s.ShareCents); err != nil {
			return nil, err
		}
		e.Splits = append(e.Splits, s)
	}
	return &e, srows.Err()
}

func (r *ExpenseRepo) SoftDelete(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	return r.setDeleted(ctx, id, actorID, true)
}

func (r *ExpenseRepo) Restore(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	return r.setDeleted(ctx, id, actorID, false)
}

// setDeleted toggles the soft-delete state and writes the matching activity
// event in one tx. delete=true sets deleted_at (guarding on currently-active);
// delete=false clears it (guarding on currently-deleted).
func (r *ExpenseRepo) setDeleted(ctx context.Context, id, actorID uuid.UUID, del bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		sql    string
		action repo.ActivityAction
	)
	if del {
		sql = `UPDATE expenses SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
		action = repo.ActionExpenseDeleted
	} else {
		sql = `UPDATE expenses SET deleted_at = NULL WHERE id = $1 AND deleted_at IS NOT NULL`
		action = repo.ActionExpenseRestored
	}
	tag, err := tx.Exec(ctx, sql, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	var groupID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT group_id FROM expenses WHERE id = $1`, id).Scan(&groupID); err != nil {
		return err
	}
	meta, err := expenseEventMeta(ctx, tx, id)
	if err != nil {
		return err
	}
	var actor *uuid.UUID
	if actorID != uuid.Nil {
		actor = &actorID
	}
	if err := insertActivityEvent(ctx, tx, repo.ActivityEvent{
		GroupID:   groupID,
		ActorID:   actor,
		Action:    action,
		ExpenseID: &id,
		Metadata:  meta,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Update applies partial changes in one tx, writing an expense_revisions row per
// changed field. See the interface doc for the split-rescale semantics.
func (r *ExpenseRepo) Update(
	ctx context.Context,
	id, editorID uuid.UUID,
	description *string,
	amountCents *int64,
	categoryID *uuid.UUID,
	payerID *uuid.UUID,
	incurredAt *time.Time,
	notes *string,
	newSplits []repo.Split,
) (*repo.Expense, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var e repo.Expense
	err = tx.QueryRow(ctx, `
		SELECT `+expenseColsWithDeleted+`
		FROM expenses WHERE id = $1 FOR UPDATE
	`, id).Scan(&e.ID, &e.GroupID, &e.PayerID, &e.CreatedBy, &e.CategoryID, &e.AmountCents, &e.Currency,
		&e.Description, &e.Notes, &e.IncurredAt, &e.CreatedAt, &e.DeletedAt, &e.RecurringExpenseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if e.DeletedAt != nil {
		return nil, repo.ErrNotFound
	}

	revisions := []struct{ field, oldV, newV string }{}
	if description != nil && *description != e.Description {
		revisions = append(revisions, struct{ field, oldV, newV string }{"description", e.Description, *description})
		e.Description = *description
	}
	if notes != nil && *notes != e.Notes {
		revisions = append(revisions, struct{ field, oldV, newV string }{"notes", e.Notes, *notes})
		e.Notes = *notes
	}
	if categoryID != nil && *categoryID != e.CategoryID {
		revisions = append(revisions, struct{ field, oldV, newV string }{"category_id", e.CategoryID.String(), categoryID.String()})
		e.CategoryID = *categoryID
	}
	if payerID != nil && *payerID != e.PayerID {
		revisions = append(revisions, struct{ field, oldV, newV string }{"payer_id", e.PayerID.String(), payerID.String()})
		e.PayerID = *payerID
	}
	if incurredAt != nil && !incurredAt.Equal(e.IncurredAt) {
		oldStr := e.IncurredAt.UTC().Format(time.RFC3339)
		newStr := incurredAt.UTC().Format(time.RFC3339)
		if oldStr != newStr {
			revisions = append(revisions, struct{ field, oldV, newV string }{"incurred_at", oldStr, newStr})
			e.IncurredAt = *incurredAt
		}
	}

	existingSplits, err := fetchSplitsForUpdate(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	var splitsToWrite []repo.Split
	splitsChanged := false
	if newSplits != nil {
		resolved := make([]repo.Split, len(newSplits))
		copy(resolved, newSplits)
		for i := range resolved {
			resolved[i].ExpenseID = id
		}
		splitsChanged = !splitsEqual(existingSplits, resolved)
		if splitsChanged {
			oldJSON, err := marshalSplitsForRevision(existingSplits)
			if err != nil {
				return nil, err
			}
			newJSON, err := marshalSplitsForRevision(resolved)
			if err != nil {
				return nil, err
			}
			revisions = append(revisions, struct{ field, oldV, newV string }{"splits", oldJSON, newJSON})
			splitsToWrite = resolved
		}
	}

	if amountCents != nil && *amountCents != e.AmountCents {
		oldAmount := e.AmountCents
		revisions = append(revisions, struct{ field, oldV, newV string }{
			"amount_cents",
			strconv.FormatInt(oldAmount, 10),
			strconv.FormatInt(*amountCents, 10),
		})
		if splitsToWrite == nil {
			rescaled := rescaleSplits(existingSplits, oldAmount, *amountCents)
			if !splitsEqual(existingSplits, rescaled) {
				splitsToWrite = rescaled
			}
		}
		e.AmountCents = *amountCents
	}

	if len(revisions) == 0 {
		return &e, tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE expenses SET description = $2, amount_cents = $3, category_id = $4, payer_id = $5, incurred_at = $6, notes = $7
		WHERE id = $1
	`, id, e.Description, e.AmountCents, e.CategoryID, e.PayerID, e.IncurredAt, e.Notes); err != nil {
		return nil, err
	}

	if splitsChanged {
		if _, err := tx.Exec(ctx, `DELETE FROM splits WHERE expense_id = $1`, id); err != nil {
			return nil, err
		}
		for _, s := range splitsToWrite {
			if _, err := tx.Exec(ctx, `
				INSERT INTO splits (expense_id, user_id, share_cents) VALUES ($1, $2, $3)
			`, id, s.UserID, s.ShareCents); err != nil {
				return nil, err
			}
		}
	} else {
		for _, s := range splitsToWrite {
			if _, err := tx.Exec(ctx, `
				UPDATE splits SET share_cents = $3 WHERE expense_id = $1 AND user_id = $2
			`, id, s.UserID, s.ShareCents); err != nil {
				return nil, err
			}
		}
	}

	for _, rv := range revisions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO expense_revisions (expense_id, edited_by, field, old_value, new_value)
			VALUES ($1, $2, $3, $4, $5)
		`, id, editorID, rv.field, rv.oldV, rv.newV); err != nil {
			return nil, err
		}
	}

	meta, err := expenseEventMeta(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	var actor *uuid.UUID
	if editorID != uuid.Nil {
		actor = &editorID
	}
	if err := insertActivityEvent(ctx, tx, repo.ActivityEvent{
		GroupID:   e.GroupID,
		ActorID:   actor,
		Action:    repo.ActionExpenseUpdated,
		ExpenseID: &id,
		Metadata:  meta,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	srows, err := r.pool.Query(ctx, `SELECT expense_id, user_id, share_cents FROM splits WHERE expense_id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var s repo.Split
		if err := srows.Scan(&s.ExpenseID, &s.UserID, &s.ShareCents); err != nil {
			return nil, err
		}
		e.Splits = append(e.Splits, s)
	}
	return &e, srows.Err()
}

func fetchSplitsForUpdate(ctx context.Context, q dbtx, expenseID uuid.UUID) ([]repo.Split, error) {
	rows, err := q.Query(ctx, `SELECT expense_id, user_id, share_cents FROM splits WHERE expense_id = $1 ORDER BY user_id`, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.Split
	for rows.Next() {
		var s repo.Split
		if err := rows.Scan(&s.ExpenseID, &s.UserID, &s.ShareCents); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// marshalSplitsForRevision emits a compact JSON array of {user_id, share_cents},
// sorted by user_id, for stable before/after diffs in expense_revisions.
func marshalSplitsForRevision(splits []repo.Split) (string, error) {
	type row struct {
		UserID     string `json:"user_id"`
		ShareCents int64  `json:"share_cents"`
	}
	rows := make([]row, len(splits))
	for i, s := range splits {
		rows[i] = row{UserID: s.UserID.String(), ShareCents: s.ShareCents}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].UserID < rows[j].UserID })
	b, err := json.Marshal(rows)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func splitsEqual(a, b []repo.Split) bool {
	if len(a) != len(b) {
		return false
	}
	index := make(map[uuid.UUID]int64, len(a))
	for _, s := range a {
		index[s.UserID] = s.ShareCents
	}
	for _, s := range b {
		if v, ok := index[s.UserID]; !ok || v != s.ShareCents {
			return false
		}
	}
	return true
}

// ListRevisions returns the full edit history for an expense (oldest first).
func (r *ExpenseRepo) ListRevisions(ctx context.Context, expenseID uuid.UUID) ([]repo.ExpenseRevision, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, expense_id, edited_by, edited_at, field, old_value, new_value
		FROM expense_revisions WHERE expense_id = $1 ORDER BY edited_at ASC
	`, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.ExpenseRevision
	for rows.Next() {
		var rv repo.ExpenseRevision
		if err := rows.Scan(&rv.ID, &rv.ExpenseID, &rv.EditedBy, &rv.EditedAt,
			&rv.Field, &rv.OldValue, &rv.NewValue); err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, rows.Err()
}

// rescaleSplits turns existing share_cents into new shares proportional to the
// new total. Rounding leftovers go to the first splits in user_id order.
func rescaleSplits(existing []repo.Split, oldTotal, newTotal int64) []repo.Split {
	out := make([]repo.Split, len(existing))
	copy(out, existing)
	if oldTotal == 0 || len(out) == 0 {
		return out
	}
	var assigned int64
	for i := range out {
		share := out[i].ShareCents * newTotal / oldTotal
		out[i].ShareCents = share
		assigned += share
	}
	for i := int64(0); i < newTotal-assigned; i++ {
		out[int(i)%len(out)].ShareCents++
	}
	return out
}
