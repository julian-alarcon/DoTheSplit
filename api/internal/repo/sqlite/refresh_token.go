package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type RefreshTokenRepo struct{ s *Store }

func (r *RefreshTokenRepo) Create(ctx context.Context, t *repo.RefreshToken) error {
	t.ID = uuid.New()
	t.IssuedAt = time.Now().UTC()
	_, err := r.s.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, issued_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, t.ID, t.UserID, t.TokenHash, tsVal(t.IssuedAt), tsVal(t.ExpiresAt))
	return err
}

// FindByTokenHash returns the token row regardless of revocation/expiry so the
// service layer can distinguish "unknown" (ErrNotFound) from "presented again
// after rotation" (revoked_at/replaced_by set), which is a reuse attack.
func (r *RefreshTokenRepo) FindByTokenHash(ctx context.Context, tokenHash []byte) (*repo.RefreshToken, error) {
	var t repo.RefreshToken
	var issued, expires string
	var revoked *string
	err := r.s.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, issued_at, expires_at, revoked_at, replaced_by
		FROM refresh_tokens WHERE token_hash = ?
	`, tokenHash).Scan(&t.ID, &t.UserID, &t.TokenHash, &issued, &expires, &revoked, &t.ReplacedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.IssuedAt = scanTS(issued)
	t.ExpiresAt = scanTS(expires)
	t.RevokedAt = scanTSPtr(revoked)
	return &t, nil
}

// Rotate revokes the presented token and inserts its successor in one tx,
// linking the old row's replaced_by to the new id. Returns the new row.
func (r *RefreshTokenRepo) Rotate(ctx context.Context, oldID uuid.UUID, next *repo.RefreshToken) (*repo.RefreshToken, error) {
	tx, err := r.s.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := native(tx).tx

	next.ID = uuid.New()
	next.IssuedAt = time.Now().UTC()
	if _, err := q.ExecContext(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, issued_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, next.ID, next.UserID, next.TokenHash, tsVal(next.IssuedAt), tsVal(next.ExpiresAt)); err != nil {
		return nil, err
	}
	if _, err := q.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked_at = ?, replaced_by = ?
		WHERE id = ?
	`, tsVal(time.Now().UTC()), next.ID, oldID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return next, nil
}

// RevokeByTokenHash marks a single token revoked. Idempotent.
func (r *RefreshTokenRepo) RevokeByTokenHash(ctx context.Context, tokenHash []byte) error {
	_, err := r.s.db.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked_at = ?
		WHERE token_hash = ? AND revoked_at IS NULL
	`, tsVal(time.Now().UTC()), tokenHash)
	return err
}

// RevokeAllForUser revokes every active refresh token for the user. Called on
// logout-everywhere, password change, account delete, email-change confirm,
// and on refresh-token reuse detection.
func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.s.db.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked_at = ?
		WHERE user_id = ? AND revoked_at IS NULL
	`, tsVal(time.Now().UTC()), userID)
	return err
}
