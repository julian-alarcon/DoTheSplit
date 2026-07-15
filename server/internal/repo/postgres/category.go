package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

type CategoryRepo struct{ pool *pgxpool.Pool }

// List returns every category in presentation order.
func (r *CategoryRepo) List(ctx context.Context) ([]repo.Category, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, slug, label, sort, group_label FROM categories ORDER BY sort, label
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.Category
	for rows.Next() {
		var c repo.Category
		if err := rows.Scan(&c.ID, &c.Slug, &c.Label, &c.Sort, &c.GroupLabel); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CategoryRepo) FindByID(ctx context.Context, id uuid.UUID) (*repo.Category, error) {
	var c repo.Category
	err := r.pool.QueryRow(ctx, `
		SELECT id, slug, label, sort, group_label FROM categories WHERE id = $1
	`, id).Scan(&c.ID, &c.Slug, &c.Label, &c.Sort, &c.GroupLabel)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CategoryRepo) FindBySlug(ctx context.Context, slug string) (*repo.Category, error) {
	var c repo.Category
	err := r.pool.QueryRow(ctx, `
		SELECT id, slug, label, sort, group_label FROM categories WHERE slug = $1
	`, slug).Scan(&c.ID, &c.Slug, &c.Label, &c.Sort, &c.GroupLabel)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}
