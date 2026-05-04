package repo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

type RecurringRepo struct {
	pool *pgxpool.Pool
}

func NewRecurringRepo(p *pgxpool.Pool) *RecurringRepo { return &RecurringRepo{pool: p} }

func (r *RecurringRepo) Create(ctx context.Context, e *RecurringExpense) error {
	tmpl, err := json.Marshal(e.SplitTemplate)
	if err != nil {
		return err
	}
	return r.pool.QueryRow(ctx, `
		INSERT INTO recurring_expenses
			(group_id, payer_id, category_id, amount_cents, currency, description, mode, split_template, cadence, next_run_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`, e.GroupID, e.PayerID, e.CategoryID, e.AmountCents, e.Currency, e.Description, e.Mode, tmpl, e.Cadence, e.NextRunAt).
		Scan(&e.ID, &e.CreatedAt)
}

func (r *RecurringRepo) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]RecurringExpense, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, group_id, payer_id, category_id, amount_cents, currency, description, mode, split_template, cadence, next_run_at, created_at
		FROM recurring_expenses
		WHERE group_id = $1 AND deleted_at IS NULL
		ORDER BY next_run_at
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecurringExpense
	for rows.Next() {
		var e RecurringExpense
		var tmpl []byte
		if err := rows.Scan(&e.ID, &e.GroupID, &e.PayerID, &e.CategoryID, &e.AmountCents, &e.Currency,
			&e.Description, &e.Mode, &tmpl, &e.Cadence, &e.NextRunAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(tmpl, &e.SplitTemplate); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *RecurringRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE recurring_expenses SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *RecurringRepo) FindByID(ctx context.Context, id uuid.UUID) (*RecurringExpense, error) {
	var e RecurringExpense
	var tmpl []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, group_id, payer_id, category_id, amount_cents, currency, description, mode, split_template, cadence, next_run_at, created_at
		FROM recurring_expenses WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&e.ID, &e.GroupID, &e.PayerID, &e.CategoryID, &e.AmountCents, &e.Currency,
		&e.Description, &e.Mode, &tmpl, &e.Cadence, &e.NextRunAt, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(tmpl, &e.SplitTemplate); err != nil {
		return nil, err
	}
	return &e, nil
}

// ClaimDue returns up to limit recurring expenses whose next_run_at is now or
// earlier, using FOR UPDATE SKIP LOCKED so concurrent workers don't collide.
// The returned rows are inside a transaction that the caller MUST commit or
// rollback via CommitClaim / RollbackClaim.
func (r *RecurringRepo) ClaimDue(ctx context.Context, limit int) (pgx.Tx, []RecurringExpense, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT id, group_id, payer_id, category_id, amount_cents, currency, description, mode, split_template, cadence, next_run_at, created_at
		FROM recurring_expenses
		WHERE deleted_at IS NULL AND next_run_at <= now()
		ORDER BY next_run_at
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`, limit)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, nil, err
	}
	defer rows.Close()
	var out []RecurringExpense
	for rows.Next() {
		var e RecurringExpense
		var tmpl []byte
		if err := rows.Scan(&e.ID, &e.GroupID, &e.PayerID, &e.CategoryID, &e.AmountCents, &e.Currency,
			&e.Description, &e.Mode, &tmpl, &e.Cadence, &e.NextRunAt, &e.CreatedAt); err != nil {
			_ = tx.Rollback(ctx)
			return nil, nil, err
		}
		if err := json.Unmarshal(tmpl, &e.SplitTemplate); err != nil {
			_ = tx.Rollback(ctx)
			return nil, nil, err
		}
		out = append(out, e)
	}
	return tx, out, rows.Err()
}

// UpdateNextRunTx advances next_run_at within an open transaction.
func (r *RecurringRepo) UpdateNextRunTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, nextRunAt time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE recurring_expenses SET next_run_at = $2 WHERE id = $1
	`, id, nextRunAt)
	return err
}
