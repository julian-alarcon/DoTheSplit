package repo

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Expense struct {
	ID          uuid.UUID
	GroupID     uuid.UUID
	PayerID     uuid.UUID
	CategoryID  uuid.UUID
	AmountCents int64
	Currency    string
	Description string
	IncurredAt  time.Time
	CreatedAt   time.Time
	DeletedAt   *time.Time
	Splits      []Split
}

type Split struct {
	ExpenseID  uuid.UUID
	UserID     uuid.UUID
	ShareCents int64
}

type ExpenseRepo struct {
	pool *pgxpool.Pool
}

func NewExpenseRepo(p *pgxpool.Pool) *ExpenseRepo { return &ExpenseRepo{pool: p} }

// CreateWithSplits inserts an expense and its splits atomically.
func (r *ExpenseRepo) CreateWithSplits(ctx context.Context, e *Expense) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO expenses (group_id, payer_id, category_id, amount_cents, currency, description, incurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`, e.GroupID, e.PayerID, e.CategoryID, e.AmountCents, e.Currency, e.Description, e.IncurredAt).
		Scan(&e.ID, &e.CreatedAt)
	if err != nil {
		return err
	}
	for i := range e.Splits {
		e.Splits[i].ExpenseID = e.ID
		if _, err := tx.Exec(ctx, `
			INSERT INTO splits (expense_id, user_id, share_cents) VALUES ($1, $2, $3)
		`, e.ID, e.Splits[i].UserID, e.Splits[i].ShareCents); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ListByGroup returns non-deleted expenses with their splits, newest first.
func (r *ExpenseRepo) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]Expense, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, group_id, payer_id, category_id, amount_cents, currency, description, incurred_at, created_at
		FROM expenses
		WHERE group_id = $1 AND deleted_at IS NULL
		ORDER BY incurred_at DESC, created_at DESC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var exps []Expense
	for rows.Next() {
		var e Expense
		if err := rows.Scan(&e.ID, &e.GroupID, &e.PayerID, &e.CategoryID, &e.AmountCents,
			&e.Currency, &e.Description, &e.IncurredAt, &e.CreatedAt); err != nil {
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
	// Fetch splits in a single query.
	ids := make([]uuid.UUID, len(exps))
	for i, e := range exps {
		ids[i] = e.ID
	}
	srows, err := r.pool.Query(ctx, `
		SELECT expense_id, user_id, share_cents FROM splits WHERE expense_id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	splitsByExpense := map[uuid.UUID][]Split{}
	for srows.Next() {
		var s Split
		if err := srows.Scan(&s.ExpenseID, &s.UserID, &s.ShareCents); err != nil {
			return nil, err
		}
		splitsByExpense[s.ExpenseID] = append(splitsByExpense[s.ExpenseID], s)
	}
	for i := range exps {
		exps[i].Splits = splitsByExpense[exps[i].ID]
	}
	return exps, srows.Err()
}

func (r *ExpenseRepo) FindByID(ctx context.Context, id uuid.UUID) (*Expense, error) {
	var e Expense
	err := r.pool.QueryRow(ctx, `
		SELECT id, group_id, payer_id, category_id, amount_cents, currency, description, incurred_at, created_at, deleted_at
		FROM expenses WHERE id = $1
	`, id).Scan(&e.ID, &e.GroupID, &e.PayerID, &e.CategoryID, &e.AmountCents, &e.Currency,
		&e.Description, &e.IncurredAt, &e.CreatedAt, &e.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
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
		var s Split
		if err := srows.Scan(&s.ExpenseID, &s.UserID, &s.ShareCents); err != nil {
			return nil, err
		}
		e.Splits = append(e.Splits, s)
	}
	return &e, srows.Err()
}

func (r *ExpenseRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE expenses SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateWithRescale applies description / amount / category / payer changes in one tx.
// If amountCents changed, existing splits are rescaled proportionally so each
// user's relative share is preserved; any rounding remainder is distributed to
// the first few splits (matching resolveSplits behavior).
// Every non-nil change writes an expense_revisions row.
func (r *ExpenseRepo) UpdateWithRescale(
	ctx context.Context,
	id, editorID uuid.UUID,
	description *string,
	amountCents *int64,
	categoryID *uuid.UUID,
	payerID *uuid.UUID,
) (*Expense, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var e Expense
	err = tx.QueryRow(ctx, `
		SELECT id, group_id, payer_id, category_id, amount_cents, currency, description, incurred_at, created_at, deleted_at
		FROM expenses WHERE id = $1 FOR UPDATE
	`, id).Scan(&e.ID, &e.GroupID, &e.PayerID, &e.CategoryID, &e.AmountCents, &e.Currency,
		&e.Description, &e.IncurredAt, &e.CreatedAt, &e.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if e.DeletedAt != nil {
		return nil, ErrNotFound
	}

	revisions := []struct{ field, oldV, newV string }{}
	if description != nil && *description != e.Description {
		revisions = append(revisions, struct{ field, oldV, newV string }{"description", e.Description, *description})
		e.Description = *description
	}
	if categoryID != nil && *categoryID != e.CategoryID {
		revisions = append(revisions, struct{ field, oldV, newV string }{"category_id", e.CategoryID.String(), categoryID.String()})
		e.CategoryID = *categoryID
	}
	if payerID != nil && *payerID != e.PayerID {
		revisions = append(revisions, struct{ field, oldV, newV string }{"payer_id", e.PayerID.String(), payerID.String()})
		e.PayerID = *payerID
	}
	rescaledSplits := []Split{}
	if amountCents != nil && *amountCents != e.AmountCents {
		oldAmount := e.AmountCents
		revisions = append(revisions, struct{ field, oldV, newV string }{
			"amount_cents",
			strconv.FormatInt(oldAmount, 10),
			strconv.FormatInt(*amountCents, 10),
		})

		srows, qerr := tx.Query(ctx, `SELECT expense_id, user_id, share_cents FROM splits WHERE expense_id = $1 ORDER BY user_id`, id)
		if qerr != nil {
			return nil, qerr
		}
		var existing []Split
		for srows.Next() {
			var s Split
			if err := srows.Scan(&s.ExpenseID, &s.UserID, &s.ShareCents); err != nil {
				srows.Close()
				return nil, err
			}
			existing = append(existing, s)
		}
		srows.Close()
		if err := srows.Err(); err != nil {
			return nil, err
		}
		rescaledSplits = rescaleSplits(existing, oldAmount, *amountCents)
		e.AmountCents = *amountCents
	}

	if len(revisions) == 0 {
		return &e, tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE expenses SET description = $2, amount_cents = $3, category_id = $4, payer_id = $5
		WHERE id = $1
	`, id, e.Description, e.AmountCents, e.CategoryID, e.PayerID); err != nil {
		return nil, err
	}

	for _, s := range rescaledSplits {
		if _, err := tx.Exec(ctx, `
			UPDATE splits SET share_cents = $3 WHERE expense_id = $1 AND user_id = $2
		`, id, s.UserID, s.ShareCents); err != nil {
			return nil, err
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
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Reload splits for the returned expense.
	srows, err := r.pool.Query(ctx, `SELECT expense_id, user_id, share_cents FROM splits WHERE expense_id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var s Split
		if err := srows.Scan(&s.ExpenseID, &s.UserID, &s.ShareCents); err != nil {
			return nil, err
		}
		e.Splits = append(e.Splits, s)
	}
	return &e, srows.Err()
}

// ListRevisions returns the full edit history for an expense (oldest first).
func (r *ExpenseRepo) ListRevisions(ctx context.Context, expenseID uuid.UUID) ([]ExpenseRevision, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, expense_id, edited_by, edited_at, field, old_value, new_value
		FROM expense_revisions WHERE expense_id = $1 ORDER BY edited_at ASC
	`, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExpenseRevision
	for rows.Next() {
		var rv ExpenseRevision
		if err := rows.Scan(&rv.ID, &rv.ExpenseID, &rv.EditedBy, &rv.EditedAt,
			&rv.Field, &rv.OldValue, &rv.NewValue); err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, rows.Err()
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

// rescaleSplits turns existing share_cents into new shares proportional to
// the new total. Rounding leftovers go to the first splits in user_id order
// (matching the order enforced at read time).
func rescaleSplits(existing []Split, oldTotal, newTotal int64) []Split {
	out := make([]Split, len(existing))
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

