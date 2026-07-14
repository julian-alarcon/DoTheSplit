package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

type RecurringRepo struct{ s *Store }

const recurringCols = `id, group_id, payer_id, category_id, amount_cents, currency, description, mode, split_template, cadence, next_run_at, created_at`

// scanRecurring scans recurringCols into e, unmarshaling the JSON split template
// and parsing the TEXT timestamps.
func scanRecurring(sc interface{ Scan(...any) error }, e *repo.RecurringExpense) error {
	var tmpl string
	var nextRun, created string
	if err := sc.Scan(&e.ID, &e.GroupID, &e.PayerID, &e.CategoryID, &e.AmountCents, &e.Currency,
		&e.Description, &e.Mode, &tmpl, &e.Cadence, &nextRun, &created); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(tmpl), &e.SplitTemplate); err != nil {
		return err
	}
	e.NextRunAt = scanTS(nextRun)
	e.CreatedAt = scanTS(created)
	return nil
}

func (r *RecurringRepo) Create(ctx context.Context, e *repo.RecurringExpense) error {
	tmpl, err := json.Marshal(e.SplitTemplate)
	if err != nil {
		return err
	}
	e.ID = uuid.New()
	e.CreatedAt = time.Now().UTC()
	_, err = r.s.db.ExecContext(ctx, `
		INSERT INTO recurring_expenses
			(id, group_id, payer_id, category_id, amount_cents, currency, description, mode, split_template, cadence, next_run_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.ID, e.GroupID, e.PayerID, e.CategoryID, e.AmountCents, e.Currency, e.Description, e.Mode,
		string(tmpl), e.Cadence, tsVal(e.NextRunAt), tsVal(e.CreatedAt))
	return err
}

func (r *RecurringRepo) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]repo.RecurringExpense, error) {
	rows, err := r.s.db.QueryContext(ctx, `
		SELECT `+recurringCols+`
		FROM recurring_expenses
		WHERE group_id = ? AND deleted_at IS NULL
		ORDER BY next_run_at
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.RecurringExpense
	for rows.Next() {
		var e repo.RecurringExpense
		if err := scanRecurring(rows, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *RecurringRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	res, err := r.s.db.ExecContext(ctx, `
		UPDATE recurring_expenses SET deleted_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, tsVal(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

func (r *RecurringRepo) FindByID(ctx context.Context, id uuid.UUID) (*repo.RecurringExpense, error) {
	var e repo.RecurringExpense
	err := scanRecurring(r.s.db.QueryRowContext(ctx, `
		SELECT `+recurringCols+`
		FROM recurring_expenses WHERE id = ? AND deleted_at IS NULL
	`, id), &e)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// CadenceByIDs maps each given recurring-template id to its cadence, including
// soft-deleted templates (a materialized expense outlives its template). Used
// by the transaction feed to tag expenses that came from a recurring template.
func (r *RecurringRepo) CadenceByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	ph, args := inPlaceholders(ids)
	rows, err := r.s.db.QueryContext(ctx, `
		SELECT id, cadence FROM recurring_expenses WHERE id IN (`+ph+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var cadence string
		if err := rows.Scan(&id, &cadence); err != nil {
			return nil, err
		}
		out[id] = cadence
	}
	return out, rows.Err()
}

// ClaimDue returns up to limit recurring expenses whose next_run_at is now or
// earlier, inside an open transaction the caller MUST commit or roll back.
// SQLite has a single writer, so no FOR UPDATE / SKIP LOCKED clause is needed:
// concurrent ticks are serialized by the write lock, not by row locks.
func (r *RecurringRepo) ClaimDue(ctx context.Context, limit int) (repo.Tx, []repo.RecurringExpense, error) {
	tx, err := r.s.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	q := native(tx).tx
	rows, err := q.QueryContext(ctx, `
		SELECT `+recurringCols+`
		FROM recurring_expenses
		WHERE deleted_at IS NULL AND next_run_at <= ?
		ORDER BY next_run_at
		LIMIT ?
	`, tsVal(time.Now().UTC()), limit)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, nil, err
	}
	defer rows.Close()
	var out []repo.RecurringExpense
	for rows.Next() {
		var e repo.RecurringExpense
		if err := scanRecurring(rows, &e); err != nil {
			_ = tx.Rollback(ctx)
			return nil, nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback(ctx)
		return nil, nil, err
	}
	return tx, out, nil
}

// UpdateNextRunTx advances next_run_at within an open transaction.
func (r *RecurringRepo) UpdateNextRunTx(ctx context.Context, tx repo.Tx, id uuid.UUID, nextRunAt time.Time) error {
	_, err := native(tx).tx.ExecContext(ctx, `
		UPDATE recurring_expenses SET next_run_at = ? WHERE id = ?
	`, tsVal(nextRunAt), id)
	return err
}
