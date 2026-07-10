package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type VerificationRepo struct{ pool *pgxpool.Pool }

// Insert creates a fresh token row. May participate in an outer tx.
func (r *VerificationRepo) Insert(ctx context.Context, tx repo.Tx, t *repo.VerificationToken) error {
	q := resolve(r.pool, tx)
	return q.QueryRow(ctx, `
		INSERT INTO email_verification_tokens
			(user_id, purpose, code_hash, new_email_hash, new_email_enc, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, attempts, created_at
	`, t.UserID, string(t.Purpose), t.CodeHash, t.NewEmailHash, t.NewEmailEnc, t.ExpiresAt).
		Scan(&t.ID, &t.Attempts, &t.CreatedAt)
}

// FindActive returns the most recent unconsumed, unexpired token for the
// (user, purpose) pair, or ErrNotFound. Used by verify/confirm flows.
func (r *VerificationRepo) FindActive(ctx context.Context, userID uuid.UUID, purpose repo.VerificationPurpose) (*repo.VerificationToken, error) {
	var t repo.VerificationToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, purpose, code_hash, new_email_hash, new_email_enc, attempts, expires_at, consumed_at, created_at
		FROM email_verification_tokens
		WHERE user_id = $1 AND purpose = $2 AND consumed_at IS NULL AND expires_at > now()
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, string(purpose)).Scan(
		&t.ID, &t.UserID, &t.Purpose, &t.CodeHash, &t.NewEmailHash, &t.NewEmailEnc,
		&t.Attempts, &t.ExpiresAt, &t.ConsumedAt, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Consume stamps consumed_at = now() so the token can't be reused.
func (r *VerificationRepo) Consume(ctx context.Context, tx repo.Tx, id uuid.UUID) error {
	q := resolve(r.pool, tx)
	ct, err := q.Exec(ctx,
		`UPDATE email_verification_tokens SET consumed_at = now() WHERE id = $1 AND consumed_at IS NULL`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// IncrementAttempts records a failed code submission.
func (r *VerificationRepo) IncrementAttempts(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE email_verification_tokens SET attempts = attempts + 1 WHERE id = $1`, id)
	return err
}

// InvalidateAll soft-cancels every active token for (user, purpose). Called
// before issuing a fresh code on resend so the previous one stops working
// immediately.
func (r *VerificationRepo) InvalidateAll(ctx context.Context, tx repo.Tx, userID uuid.UUID, purpose repo.VerificationPurpose) error {
	q := resolve(r.pool, tx)
	_, err := q.Exec(ctx, `
		UPDATE email_verification_tokens SET consumed_at = now()
		WHERE user_id = $1 AND purpose = $2 AND consumed_at IS NULL
	`, userID, string(purpose))
	return err
}
