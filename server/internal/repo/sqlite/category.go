package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

type CategoryRepo struct{ s *Store }

func (r *CategoryRepo) List(ctx context.Context) ([]repo.Category, error) {
	rows, err := r.s.db.QueryContext(ctx, `
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
	err := r.s.db.QueryRowContext(ctx, `
		SELECT id, slug, label, sort, group_label FROM categories WHERE id = ?
	`, id).Scan(&c.ID, &c.Slug, &c.Label, &c.Sort, &c.GroupLabel)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CategoryRepo) FindBySlug(ctx context.Context, slug string) (*repo.Category, error) {
	var c repo.Category
	err := r.s.db.QueryRowContext(ctx, `
		SELECT id, slug, label, sort, group_label FROM categories WHERE slug = ?
	`, slug).Scan(&c.ID, &c.Slug, &c.Label, &c.Sort, &c.GroupLabel)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}
