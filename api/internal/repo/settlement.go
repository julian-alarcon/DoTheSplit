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

func (r *SettlementRepo) Create(ctx context.Context, s *Settlement) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO settlements (group_id, from_user, to_user, amount_cents, note, settled_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`, s.GroupID, s.FromUser, s.ToUser, s.AmountCents, s.Note, s.SettledAt).
		Scan(&s.ID, &s.CreatedAt)
}

func (r *SettlementRepo) FindByID(ctx context.Context, id uuid.UUID) (*Settlement, error) {
	var s Settlement
	err := r.pool.QueryRow(ctx, `
		SELECT id, group_id, from_user, to_user, amount_cents, note, settled_at, created_at, deleted_at
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

func (r *SettlementRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE settlements SET deleted_at = now()
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

func (r *SettlementRepo) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]Settlement, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, group_id, from_user, to_user, amount_cents, note, settled_at, created_at
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
