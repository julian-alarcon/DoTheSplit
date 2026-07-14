package sqlite

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

type EmailOutboxRepo struct{ s *Store }

// Enqueue writes a row, optionally inside a caller-supplied transaction so the
// outbox insert commits atomically with the action that triggered the email
// (e.g. settlement creation). Sets the assigned id, attempts, next_attempt_at,
// and created_at on the row.
func (r *EmailOutboxRepo) Enqueue(ctx context.Context, tx repo.Tx, row *repo.OutboxRow) error {
	q := r.s.resolve(tx)
	now := time.Now().UTC()
	row.ID = uuid.New()
	row.Attempts = 0
	row.CreatedAt = now
	row.NextAttemptAt = now
	_, err := q.ExecContext(ctx, `
		INSERT INTO email_outbox (id, to_email_enc, subject, body, template, attempts, next_attempt_at, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?)
	`, row.ID, row.ToEmailEnc, row.Subject, row.Body, row.Template, tsVal(row.NextAttemptAt), tsVal(row.CreatedAt))
	return err
}

// ClaimDue returns up to limit pending rows whose next_attempt_at has elapsed.
// The caller (the worker) is expected to hold the outbox advisory lock so two
// workers can't race on the same rows.
func (r *EmailOutboxRepo) ClaimDue(ctx context.Context, limit int) ([]repo.OutboxRow, error) {
	rows, err := r.s.db.QueryContext(ctx, `
		SELECT id, to_email_enc, subject, body, template, attempts, last_error, sent_at, next_attempt_at, created_at
		FROM email_outbox
		WHERE sent_at IS NULL AND attempts < 5 AND next_attempt_at <= ?
		ORDER BY next_attempt_at ASC
		LIMIT ?
	`, tsVal(time.Now().UTC()), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.OutboxRow
	for rows.Next() {
		var o repo.OutboxRow
		var next, created string
		var sent *string
		if err := rows.Scan(&o.ID, &o.ToEmailEnc, &o.Subject, &o.Body, &o.Template,
			&o.Attempts, &o.LastError, &sent, &next, &created); err != nil {
			return nil, err
		}
		o.SentAt = scanTSPtr(sent)
		o.NextAttemptAt = scanTS(next)
		o.CreatedAt = scanTS(created)
		out = append(out, o)
	}
	return out, rows.Err()
}

// MarkSent records a successful delivery.
func (r *EmailOutboxRepo) MarkSent(ctx context.Context, id uuid.UUID) error {
	_, err := r.s.db.ExecContext(ctx,
		`UPDATE email_outbox SET sent_at = ?, last_error = NULL WHERE id = ?`,
		tsVal(time.Now().UTC()), id)
	return err
}

// MarkFailed bumps the attempt counter, records the last error, and pushes
// next_attempt_at forward. retryAfter is added to now() so the caller controls
// the backoff schedule (1m, 5m, 15m, 1h, 6h are typical).
func (r *EmailOutboxRepo) MarkFailed(ctx context.Context, id uuid.UUID, lastErr string, retryAfter time.Duration) error {
	next := time.Now().UTC().Add(retryAfter)
	_, err := r.s.db.ExecContext(ctx, `
		UPDATE email_outbox
		SET attempts = attempts + 1,
		    last_error = ?,
		    next_attempt_at = ?
		WHERE id = ?
	`, lastErr, tsVal(next), id)
	return err
}
