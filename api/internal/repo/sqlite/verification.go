package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type VerificationRepo struct{ s *Store }

// Insert creates a fresh token row. May participate in an outer tx.
func (r *VerificationRepo) Insert(ctx context.Context, tx repo.Tx, t *repo.VerificationToken) error {
	q := r.s.resolve(tx)
	t.ID = uuid.New()
	t.CreatedAt = time.Now().UTC()
	t.Attempts = 0
	_, err := q.ExecContext(ctx, `
		INSERT INTO email_verification_tokens
			(id, user_id, purpose, code_hash, new_email_hash, new_email_enc, attempts, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)
	`, t.ID, t.UserID, string(t.Purpose), t.CodeHash, t.NewEmailHash, t.NewEmailEnc, tsVal(t.ExpiresAt), tsVal(t.CreatedAt))
	return err
}

// FindActive returns the most recent unconsumed, unexpired token for the
// (user, purpose) pair, or ErrNotFound. Used by verify/confirm flows.
func (r *VerificationRepo) FindActive(ctx context.Context, userID uuid.UUID, purpose repo.VerificationPurpose) (*repo.VerificationToken, error) {
	var t repo.VerificationToken
	var purposeStr, expires, created string
	var consumed *string
	err := r.s.db.QueryRowContext(ctx, `
		SELECT id, user_id, purpose, code_hash, new_email_hash, new_email_enc, attempts, expires_at, consumed_at, created_at
		FROM email_verification_tokens
		WHERE user_id = ? AND purpose = ? AND consumed_at IS NULL AND expires_at > ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, string(purpose), tsVal(time.Now().UTC())).Scan(
		&t.ID, &t.UserID, &purposeStr, &t.CodeHash, &t.NewEmailHash, &t.NewEmailEnc,
		&t.Attempts, &expires, &consumed, &created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Purpose = repo.VerificationPurpose(purposeStr)
	t.ExpiresAt = scanTS(expires)
	t.ConsumedAt = scanTSPtr(consumed)
	t.CreatedAt = scanTS(created)
	return &t, nil
}

// Consume stamps consumed_at = now() so the token can't be reused.
func (r *VerificationRepo) Consume(ctx context.Context, tx repo.Tx, id uuid.UUID) error {
	q := r.s.resolve(tx)
	res, err := q.ExecContext(ctx,
		`UPDATE email_verification_tokens SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL`,
		tsVal(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// IncrementAttempts records a failed code submission.
func (r *VerificationRepo) IncrementAttempts(ctx context.Context, id uuid.UUID) error {
	_, err := r.s.db.ExecContext(ctx,
		`UPDATE email_verification_tokens SET attempts = attempts + 1 WHERE id = ?`, id)
	return err
}

// InvalidateAll soft-cancels every active token for (user, purpose). Called
// before issuing a fresh code on resend so the previous one stops working
// immediately.
func (r *VerificationRepo) InvalidateAll(ctx context.Context, tx repo.Tx, userID uuid.UUID, purpose repo.VerificationPurpose) error {
	q := r.s.resolve(tx)
	_, err := q.ExecContext(ctx, `
		UPDATE email_verification_tokens SET consumed_at = ?
		WHERE user_id = ? AND purpose = ? AND consumed_at IS NULL
	`, tsVal(time.Now().UTC()), userID, string(purpose))
	return err
}
