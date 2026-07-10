package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type EmailOutboxRepo struct{ pool *pgxpool.Pool }

// Enqueue writes a row, optionally inside a caller-supplied transaction so the
// outbox insert commits atomically with the action that triggered the email
// (e.g. settlement creation). Returns the assigned id and created_at.
func (r *EmailOutboxRepo) Enqueue(ctx context.Context, tx repo.Tx, row *repo.OutboxRow) error {
	q := resolve(r.pool, tx)
	return q.QueryRow(ctx, `
		INSERT INTO email_outbox (to_email_enc, subject, body, template)
		VALUES ($1, $2, $3, $4)
		RETURNING id, attempts, next_attempt_at, created_at
	`, row.ToEmailEnc, row.Subject, row.Body, row.Template).
		Scan(&row.ID, &row.Attempts, &row.NextAttemptAt, &row.CreatedAt)
}

// ClaimDue returns up to limit pending rows whose next_attempt_at has elapsed.
// The caller (the worker) is expected to hold the outbox advisory lock so two
// workers can't race on the same rows.
func (r *EmailOutboxRepo) ClaimDue(ctx context.Context, limit int) ([]repo.OutboxRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, to_email_enc, subject, body, template, attempts, last_error, sent_at, next_attempt_at, created_at
		FROM email_outbox
		WHERE sent_at IS NULL AND attempts < 5 AND next_attempt_at <= now()
		ORDER BY next_attempt_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.OutboxRow
	for rows.Next() {
		var o repo.OutboxRow
		if err := rows.Scan(&o.ID, &o.ToEmailEnc, &o.Subject, &o.Body, &o.Template,
			&o.Attempts, &o.LastError, &o.SentAt, &o.NextAttemptAt, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// MarkSent records a successful delivery.
func (r *EmailOutboxRepo) MarkSent(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE email_outbox SET sent_at = now(), last_error = NULL WHERE id = $1`, id)
	return err
}

// MarkFailed bumps the attempt counter, records the last error, and pushes
// next_attempt_at forward. retryAfter is added to now() so the caller controls
// the backoff schedule (1m, 5m, 15m, 1h, 6h are typical).
func (r *EmailOutboxRepo) MarkFailed(ctx context.Context, id uuid.UUID, lastErr string, retryAfter time.Duration) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE email_outbox
		SET attempts = attempts + 1,
		    last_error = $2,
		    next_attempt_at = now() + ($3::bigint || ' milliseconds')::interval
		WHERE id = $1
	`, id, lastErr, retryAfter.Milliseconds())
	return err
}
