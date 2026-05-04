package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Group struct {
	ID              uuid.UUID
	Name            string
	DefaultCurrency string
	CreatedBy       uuid.UUID
	CreatedAt       time.Time
}

type GroupMember struct {
	GroupID         uuid.UUID
	UserID          uuid.UUID
	DisplayName     string
	JoinedAt        time.Time
	AvatarUpdatedAt *time.Time
	DeletedAt       *time.Time
}

type GroupRepo struct {
	pool *pgxpool.Pool
}

func NewGroupRepo(p *pgxpool.Pool) *GroupRepo { return &GroupRepo{pool: p} }

// Create inserts the group and adds the creator as a member. Done in a transaction.
func (r *GroupRepo) Create(ctx context.Context, name, defaultCurrency string, creatorID uuid.UUID) (*Group, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	g := &Group{Name: name, DefaultCurrency: defaultCurrency, CreatedBy: creatorID}
	err = tx.QueryRow(ctx, `
		INSERT INTO groups (name, default_currency, created_by) VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, name, defaultCurrency, creatorID).Scan(&g.ID, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO group_members (group_id, user_id) VALUES ($1, $2)
	`, g.ID, creatorID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return g, nil
}

func (r *GroupRepo) FindByID(ctx context.Context, id uuid.UUID) (*Group, error) {
	var g Group
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, default_currency, created_by, created_at FROM groups WHERE id = $1
	`, id).Scan(&g.ID, &g.Name, &g.DefaultCurrency, &g.CreatedBy, &g.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// ListForUser returns groups the user belongs to, newest first.
func (r *GroupRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]Group, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT g.id, g.name, g.default_currency, g.created_by, g.created_at
		FROM groups g
		JOIN group_members m ON m.group_id = g.id
		WHERE m.user_id = $1
		ORDER BY g.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.DefaultCurrency, &g.CreatedBy, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Update applies a partial update to the group. nil pointers mean "leave unchanged".
func (r *GroupRepo) Update(ctx context.Context, id uuid.UUID, name, defaultCurrency *string) (*Group, error) {
	if name == nil && defaultCurrency == nil {
		return r.FindByID(ctx, id)
	}
	var g Group
	err := r.pool.QueryRow(ctx, `
		UPDATE groups SET
			name             = COALESCE($2, name),
			default_currency = COALESCE($3, default_currency)
		WHERE id = $1
		RETURNING id, name, default_currency, created_by, created_at
	`, id, name, defaultCurrency).
		Scan(&g.ID, &g.Name, &g.DefaultCurrency, &g.CreatedBy, &g.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// Delete removes the group. Cascades to members, expenses, splits, settlements, recurring.
func (r *GroupRepo) Delete(ctx context.Context, id uuid.UUID) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM groups WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListMembers returns members + their display names for a group.
func (r *GroupRepo) ListMembers(ctx context.Context, groupID uuid.UUID) ([]GroupMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.group_id, m.user_id, u.display_name, m.joined_at, u.avatar_updated_at, u.deleted_at
		FROM group_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.group_id = $1
		ORDER BY m.joined_at
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GroupMember
	for rows.Next() {
		var m GroupMember
		if err := rows.Scan(&m.GroupID, &m.UserID, &m.DisplayName, &m.JoinedAt, &m.AvatarUpdatedAt, &m.DeletedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *GroupRepo) IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)
	`, groupID, userID).Scan(&exists)
	return exists, err
}

// ShareAnyGroup reports whether two users are in at least one group together.
// Callers of /v1/users/{id}/avatar rely on this for authorization.
func (r *GroupRepo) ShareAnyGroup(ctx context.Context, a, b uuid.UUID) (bool, error) {
	if a == b {
		return true, nil
	}
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM group_members ma
			JOIN group_members mb ON mb.group_id = ma.group_id
			WHERE ma.user_id = $1 AND mb.user_id = $2
		)
	`, a, b).Scan(&exists)
	return exists, err
}

// AddMember inserts a membership. Returns the new member's row (with display name).
// If the user is already a member, returns the existing row.
func (r *GroupRepo) AddMember(ctx context.Context, groupID, userID uuid.UUID) (*GroupMember, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO group_members (group_id, user_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, groupID, userID)
	if err != nil {
		return nil, err
	}
	var m GroupMember
	err = r.pool.QueryRow(ctx, `
		SELECT m.group_id, m.user_id, u.display_name, m.joined_at, u.avatar_updated_at, u.deleted_at
		FROM group_members m JOIN users u ON u.id = m.user_id
		WHERE m.group_id = $1 AND m.user_id = $2
	`, groupID, userID).Scan(&m.GroupID, &m.UserID, &m.DisplayName, &m.JoinedAt, &m.AvatarUpdatedAt, &m.DeletedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
