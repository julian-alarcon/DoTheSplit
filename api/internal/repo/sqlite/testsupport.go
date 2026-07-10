package sqlite

import (
	"context"
	"time"
)

// Engine-correct counterparts to the postgres package's test-support methods,
// so one integration test can run against either engine. See the postgres
// package doc comments for intent.

func (s *Store) SeedSMTPConfig(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO smtp_config (id, host, port, from_address, tls_mode, updated_at)
		VALUES (1, 'localhost', 2525, 'noreply@example.test', 'none', ?)
		ON CONFLICT (id) DO UPDATE SET
			host = excluded.host, port = excluded.port,
			from_address = excluded.from_address, tls_mode = excluded.tls_mode
	`, tsVal(time.Now().UTC()))
	return err
}

func (s *Store) PinLatestCodeHash(ctx context.Context, purpose string, codeHash []byte) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE email_verification_tokens
		SET code_hash = ?, attempts = 0
		WHERE id = (
			SELECT id FROM email_verification_tokens
			WHERE consumed_at IS NULL AND purpose = ?
			ORDER BY created_at DESC LIMIT 1
		)
	`, codeHash, purpose)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) ExpireActiveTokens(ctx context.Context, purpose string) error {
	past := tsVal(time.Now().UTC().Add(-time.Minute))
	_, err := s.db.ExecContext(ctx,
		`UPDATE email_verification_tokens SET expires_at = ? WHERE consumed_at IS NULL AND purpose = ?`, past, purpose)
	return err
}

func (s *Store) CountVerificationTokens(ctx context.Context, purpose, userID string, activeOnly bool) (int, error) {
	q := `SELECT count(*) FROM email_verification_tokens WHERE purpose = ?`
	args := []any{purpose}
	if userID != "" {
		args = append(args, userID)
		q += ` AND user_id = ?`
	}
	if activeOnly {
		q += ` AND consumed_at IS NULL`
	}
	var n int
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

func (s *Store) CountActiveUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

func (s *Store) UserDisplayName(ctx context.Context, userID string) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT display_name FROM users WHERE id = ?`, userID).Scan(&name)
	return name, err
}

func (s *Store) RecurringNextRun(ctx context.Context, id string) (time.Time, error) {
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT next_run_at FROM recurring_expenses WHERE id = ?`, id).Scan(&raw); err != nil {
		return time.Time{}, err
	}
	return scanTS(raw), nil
}
