package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

type SettlementRepo struct{ pool *pgxpool.Pool }

const (
	settlementCols            = `id, group_id, from_user, to_user, amount_cents, note, settled_at, created_at`
	settlementColsWithDeleted = settlementCols + `, deleted_at`
)

// Create inserts a settlement and its activity event in its own transaction.
// Callers that already hold a transaction (e.g. the importer) should use
// CreateTx instead.
func (r *SettlementRepo) Create(ctx context.Context, s *repo.Settlement, actorID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
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
	return r.createTx(ctx, native(tx), s, actorID)
}

func (r *SettlementRepo) createTx(ctx context.Context, q dbtx, s *repo.Settlement, actorID uuid.UUID) error {
	// A CSV restore may supply the original creation time; otherwise let the DB
	// default (now()) apply and read it back via RETURNING.
	var createdAt *time.Time
	if !s.CreatedAt.IsZero() {
		createdAt = &s.CreatedAt
	}
	if err := q.QueryRow(ctx, `
		INSERT INTO settlements (group_id, from_user, to_user, amount_cents, note, settled_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, now()))
		RETURNING id, created_at
	`, s.GroupID, s.FromUser, s.ToUser, s.AmountCents, s.Note, s.SettledAt, createdAt).
		Scan(&s.ID, &s.CreatedAt); err != nil {
		return err
	}
	if err := insertSettlementEvent(ctx, q, s.GroupID, s.ID, actorID, repo.ActionSettlementCreated, s.CreatedAt); err != nil {
		return err
	}
	return nil
}

func (r *SettlementRepo) FindByID(ctx context.Context, id uuid.UUID) (*repo.Settlement, error) {
	var s repo.Settlement
	err := r.pool.QueryRow(ctx, `
		SELECT `+settlementColsWithDeleted+`
		FROM settlements
		WHERE id = $1
	`, id).Scan(&s.ID, &s.GroupID, &s.FromUser, &s.ToUser,
		&s.AmountCents, &s.Note, &s.SettledAt, &s.CreatedAt, &s.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repo.ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *SettlementRepo) Update(ctx context.Context, s *repo.Settlement, actorID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE settlements
		SET from_user = $2, to_user = $3, amount_cents = $4, note = $5, settled_at = $6
		WHERE id = $1 AND deleted_at IS NULL
	`, s.ID, s.FromUser, s.ToUser, s.AmountCents, s.Note, s.SettledAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	if err := insertSettlementEvent(ctx, tx, s.GroupID, s.ID, actorID, repo.ActionSettlementUpdated, time.Time{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *SettlementRepo) SoftDelete(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var groupID uuid.UUID
	tag, err := tx.Exec(ctx, `
		UPDATE settlements SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	if err := tx.QueryRow(ctx, `SELECT group_id FROM settlements WHERE id = $1`, id).Scan(&groupID); err != nil {
		return err
	}
	if err := insertSettlementEvent(ctx, tx, groupID, id, actorID, repo.ActionSettlementDeleted, time.Time{}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *SettlementRepo) Restore(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var groupID uuid.UUID
	tag, err := tx.Exec(ctx, `
		UPDATE settlements SET deleted_at = NULL
		WHERE id = $1 AND deleted_at IS NOT NULL
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	if err := tx.QueryRow(ctx, `SELECT group_id FROM settlements WHERE id = $1`, id).Scan(&groupID); err != nil {
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
	rows, err := r.pool.Query(ctx, `
		SELECT `+settlementCols+`
		FROM settlements
		WHERE id = ANY($1) AND deleted_at IS NULL
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID]repo.Settlement, len(ids))
	for rows.Next() {
		var s repo.Settlement
		if err := rows.Scan(&s.ID, &s.GroupID, &s.FromUser, &s.ToUser,
			&s.AmountCents, &s.Note, &s.SettledAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		out[s.ID] = s
	}
	return out, rows.Err()
}

func (r *SettlementRepo) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]repo.Settlement, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+settlementCols+`
		FROM settlements
		WHERE group_id = $1 AND deleted_at IS NULL
		ORDER BY settled_at DESC, created_at DESC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.Settlement
	for rows.Next() {
		var s repo.Settlement
		if err := rows.Scan(&s.ID, &s.GroupID, &s.FromUser, &s.ToUser,
			&s.AmountCents, &s.Note, &s.SettledAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
