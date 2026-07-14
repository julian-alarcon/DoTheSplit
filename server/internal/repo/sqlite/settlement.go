package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

type SettlementRepo struct{ s *Store }

const (
	settlementCols            = `id, group_id, from_user, to_user, amount_cents, note, settled_at, created_at`
	settlementColsWithDeleted = settlementCols + `, deleted_at`
)

// Create inserts a settlement and its activity event in its own transaction.
// Callers that already hold a transaction (e.g. the importer) should use
// CreateTx instead.
func (r *SettlementRepo) Create(ctx context.Context, s *repo.Settlement, actorID uuid.UUID) error {
	tx, err := r.s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.createTx(ctx, tx, s, actorID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CreateTx inserts a settlement and its activity event on the caller-owned
// transaction. The caller owns the tx lifecycle.
func (r *SettlementRepo) CreateTx(ctx context.Context, tx repo.Tx, s *repo.Settlement, actorID uuid.UUID) error {
	return r.createTx(ctx, tx, s, actorID)
}

func (r *SettlementRepo) createTx(ctx context.Context, tx repo.Tx, s *repo.Settlement, actorID uuid.UUID) error {
	q := native(tx).tx
	s.ID = uuid.New()
	// A CSV restore may supply the original creation time; otherwise stamp now.
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO settlements (id, group_id, from_user, to_user, amount_cents, note, settled_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.GroupID, s.FromUser, s.ToUser, s.AmountCents, s.Note, tsVal(s.SettledAt), tsVal(s.CreatedAt)); err != nil {
		return err
	}
	return insertSettlementEvent(ctx, tx, s.GroupID, s.ID, actorID, repo.ActionSettlementCreated, s.CreatedAt)
}

func (r *SettlementRepo) FindByID(ctx context.Context, id uuid.UUID) (*repo.Settlement, error) {
	var s repo.Settlement
	var settled, created string
	var deleted *string
	err := r.s.db.QueryRowContext(ctx, `
		SELECT `+settlementColsWithDeleted+`
		FROM settlements
		WHERE id = ?
	`, id).Scan(&s.ID, &s.GroupID, &s.FromUser, &s.ToUser,
		&s.AmountCents, &s.Note, &settled, &created, &deleted)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repo.ErrNotFound
		}
		return nil, err
	}
	s.SettledAt = scanTS(settled)
	s.CreatedAt = scanTS(created)
	s.DeletedAt = scanTSPtr(deleted)
	return &s, nil
}

func (r *SettlementRepo) Update(ctx context.Context, s *repo.Settlement, actorID uuid.UUID) error {
	tx, err := r.s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := native(tx).tx

	res, err := q.ExecContext(ctx, `
		UPDATE settlements
		SET from_user = ?, to_user = ?, amount_cents = ?, note = ?, settled_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, s.FromUser, s.ToUser, s.AmountCents, s.Note, tsVal(s.SettledAt), s.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	if err := insertSettlementEvent(ctx, tx, s.GroupID, s.ID, actorID, repo.ActionSettlementUpdated, time.Time{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *SettlementRepo) SoftDelete(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	tx, err := r.s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := native(tx).tx

	res, err := q.ExecContext(ctx, `
		UPDATE settlements SET deleted_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, tsVal(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	var groupID uuid.UUID
	if err := q.QueryRowContext(ctx, `SELECT group_id FROM settlements WHERE id = ?`, id).Scan(&groupID); err != nil {
		return err
	}
	if err := insertSettlementEvent(ctx, tx, groupID, id, actorID, repo.ActionSettlementDeleted, time.Time{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *SettlementRepo) Restore(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	tx, err := r.s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := native(tx).tx

	res, err := q.ExecContext(ctx, `
		UPDATE settlements SET deleted_at = NULL
		WHERE id = ? AND deleted_at IS NOT NULL
	`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	var groupID uuid.UUID
	if err := q.QueryRowContext(ctx, `SELECT group_id FROM settlements WHERE id = ?`, id).Scan(&groupID); err != nil {
		return err
	}
	if err := insertSettlementEvent(ctx, tx, groupID, id, actorID, repo.ActionSettlementRestored, time.Time{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FindByIDs returns the non-deleted settlements with the given IDs, keyed by id.
// Missing IDs (or soft-deleted ones) are simply absent from the result.
func (r *SettlementRepo) FindByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]repo.Settlement, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]repo.Settlement{}, nil
	}
	ph, args := inPlaceholders(ids)
	rows, err := r.s.db.QueryContext(ctx, `
		SELECT `+settlementCols+`
		FROM settlements
		WHERE id IN (`+ph+`) AND deleted_at IS NULL
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID]repo.Settlement, len(ids))
	for rows.Next() {
		var s repo.Settlement
		var settled, created string
		if err := rows.Scan(&s.ID, &s.GroupID, &s.FromUser, &s.ToUser,
			&s.AmountCents, &s.Note, &settled, &created); err != nil {
			return nil, err
		}
		s.SettledAt = scanTS(settled)
		s.CreatedAt = scanTS(created)
		out[s.ID] = s
	}
	return out, rows.Err()
}

func (r *SettlementRepo) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]repo.Settlement, error) {
	rows, err := r.s.db.QueryContext(ctx, `
		SELECT `+settlementCols+`
		FROM settlements
		WHERE group_id = ? AND deleted_at IS NULL
		ORDER BY settled_at DESC, created_at DESC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.Settlement
	for rows.Next() {
		var s repo.Settlement
		var settled, created string
		if err := rows.Scan(&s.ID, &s.GroupID, &s.FromUser, &s.ToUser,
			&s.AmountCents, &s.Note, &settled, &created); err != nil {
			return nil, err
		}
		s.SettledAt = scanTS(settled)
		s.CreatedAt = scanTS(created)
		out = append(out, s)
	}
	return out, rows.Err()
}
