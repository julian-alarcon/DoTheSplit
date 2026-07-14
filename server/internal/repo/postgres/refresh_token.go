package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

type RefreshTokenRepo struct{ pool *pgxpool.Pool }

func (r *RefreshTokenRepo) Create(ctx context.Context, t *repo.RefreshToken) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, issued_at
	`, t.UserID, t.TokenHash, t.ExpiresAt).Scan(&t.ID, &t.IssuedAt)
}

// FindByTokenHash returns the token row regardless of revocation/expiry so the
// service layer can distinguish "unknown" (ErrNotFound) from "presented again
// after rotation" (revoked_at/replaced_by set), which is a reuse attack.
func (r *RefreshTokenRepo) FindByTokenHash(ctx context.Context, tokenHash []byte) (*repo.RefreshToken, error) {
	var t repo.RefreshToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, issued_at, expires_at, revoked_at, replaced_by
		FROM refresh_tokens WHERE token_hash = $1
	`, tokenHash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.IssuedAt, &t.ExpiresAt, &t.RevokedAt, &t.ReplacedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Rotate revokes the presented token and inserts its successor in one tx,
// linking the old row's replaced_by to the new id. Returns the new row.
func (r *RefreshTokenRepo) Rotate(ctx context.Context, oldID uuid.UUID, next *repo.RefreshToken) (*repo.RefreshToken, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, issued_at
	`, next.UserID, next.TokenHash, next.ExpiresAt).Scan(&next.ID, &next.IssuedAt); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $2
		WHERE id = $1
	`, oldID, next.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return next, nil
}

// RevokeByTokenHash marks a single token revoked. Idempotent.
func (r *RefreshTokenRepo) RevokeByTokenHash(ctx context.Context, tokenHash []byte) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, tokenHash)
	return err
}

// RevokeAllForUser revokes every active refresh token for the user. Called on
// logout-everywhere, password change, account delete, email-change confirm,
// and on refresh-token reuse detection.
func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	return err
}
