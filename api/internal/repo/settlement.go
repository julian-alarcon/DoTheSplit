package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

type SettlementRepo struct {
	pool *pgxpool.Pool
}

func NewSettlementRepo(p *pgxpool.Pool) *SettlementRepo { return &SettlementRepo{pool: p} }

const (
	settlementCols            = `id, group_id, from_user, to_user, amount_cents, note, settled_at, created_at`
	settlementColsWithDeleted = settlementCols + `, deleted_at`
)

// Create inserts a settlement and its activity event in its own transaction.
// Callers that already hold a transaction (e.g. the importer) should use
// CreateTx instead.
func (r *SettlementRepo) Create(ctx context.Context, s *Settlement, actorID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.CreateTx(ctx, tx, s, actorID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CreateTx inserts a settlement and its activity event on the supplied
// transaction. The caller owns the tx lifecycle.
func (r *SettlementRepo) CreateTx(ctx context.Context, tx pgx.Tx, s *Settlement, actorID uuid.UUID) error {
	if err := tx.QueryRow(ctx, `
		INSERT INTO settlements (group_id, from_user, to_user, amount_cents, note, settled_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`, s.GroupID, s.FromUser, s.ToUser, s.AmountCents, s.Note, s.SettledAt).
		Scan(&s.ID, &s.CreatedAt); err != nil {
		return err
	}
	if err := insertSettlementEvent(ctx, tx, s.GroupID, s.ID, actorID, ActionSettlementCreated); err != nil {
		return err
	}
	return nil
}

// insertSettlementEvent writes a settlement activity event inside an existing
// tx. The denormalized note/amount are resolved at read time via the join, so
// no metadata is needed here.
func insertSettlementEvent(ctx context.Context, tx pgx.Tx, groupID, settlementID, actorID uuid.UUID, action ActivityAction) error {
	var actor *uuid.UUID
	if actorID != uuid.Nil {
		actor = &actorID
	}
	return insertActivityEvent(ctx, tx, ActivityEvent{
		GroupID:      groupID,
		ActorID:      actor,
		Action:       action,
		SettlementID: &settlementID,
	})
}

func (r *SettlementRepo) FindByID(ctx context.Context, id uuid.UUID) (*Settlement, error) {
	var s Settlement
	err := r.pool.QueryRow(ctx, `
		SELECT `+settlementColsWithDeleted+`
		FROM settlements
		WHERE id = $1
	`, id).Scan(&s.ID, &s.GroupID, &s.FromUser, &s.ToUser,
		&s.AmountCents, &s.Note, &s.SettledAt, &s.CreatedAt, &s.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *SettlementRepo) Update(ctx context.Context, s *Settlement, actorID uuid.UUID) error {
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
		return ErrNotFound
	}
	if err := insertSettlementEvent(ctx, tx, s.GroupID, s.ID, actorID, ActionSettlementUpdated); err != nil {
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
		return ErrNotFound
	}
	if err := tx.QueryRow(ctx, `SELECT group_id FROM settlements WHERE id = $1`, id).Scan(&groupID); err != nil {
		return err
	}
	if err := insertSettlementEvent(ctx, tx, groupID, id, actorID, ActionSettlementDeleted); err != nil {
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
		return ErrNotFound
	}
	if err := tx.QueryRow(ctx, `SELECT group_id FROM settlements WHERE id = $1`, id).Scan(&groupID); err != nil {
		return err
	}
	if err := insertSettlementEvent(ctx, tx, groupID, id, actorID, ActionSettlementRestored); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FindByIDs returns the non-deleted settlements with the given IDs, keyed by id.
// Missing IDs (or soft-deleted ones) are simply absent from the result.
func (r *SettlementRepo) FindByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Settlement, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]Settlement{}, nil
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
	out := make(map[uuid.UUID]Settlement, len(ids))
	for rows.Next() {
		var s Settlement
		if err := rows.Scan(&s.ID, &s.GroupID, &s.FromUser, &s.ToUser,
			&s.AmountCents, &s.Note, &s.SettledAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		out[s.ID] = s
	}
	return out, rows.Err()
}

func (r *SettlementRepo) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]Settlement, error) {
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
	var out []Settlement
	for rows.Next() {
		var s Settlement
		if err := rows.Scan(&s.ID, &s.GroupID, &s.FromUser, &s.ToUser,
			&s.AmountCents, &s.Note, &s.SettledAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
