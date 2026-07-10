package postgres

import (
	"context"
	"time"
)

// The methods below exist solely for the integration test harness, which needs
// to seed/inspect rows with engine-specific SQL that has no place in the
// production repo interface. They are engine-correct counterparts to the sqlite
// package's equivalents so a single test can run against either engine.

// SeedSMTPConfig writes the single smtp_config row so the API treats the
// instance as SMTP-configured.
func (s *Store) SeedSMTPConfig(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO smtp_config (id, host, port, from_address, tls_mode)
		VALUES (true, 'localhost', 2525, 'noreply@example.test', 'none')
		ON CONFLICT (id) DO UPDATE SET
			host = EXCLUDED.host, port = EXCLUDED.port,
			from_address = EXCLUDED.from_address, tls_mode = EXCLUDED.tls_mode
	`)
	return err
}

// PinLatestCodeHash overwrites the newest unconsumed token of the given purpose
// with codeHash (attempts reset to 0). Returns the number of rows affected.
func (s *Store) PinLatestCodeHash(ctx context.Context, purpose string, codeHash []byte) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE email_verification_tokens
		SET code_hash = $1, attempts = 0
		WHERE id = (
			SELECT id FROM email_verification_tokens
			WHERE consumed_at IS NULL AND purpose = $2
			ORDER BY created_at DESC LIMIT 1
		)
	`, codeHash, purpose)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ExpireActiveTokens force-expires every unconsumed token of the given purpose.
func (s *Store) ExpireActiveTokens(ctx context.Context, purpose string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE email_verification_tokens SET expires_at = now() - interval '1 minute' WHERE consumed_at IS NULL AND purpose = $1`, purpose)
	return err
}

// CountVerificationTokens counts tokens of a purpose, optionally scoped to a
// user (empty userID = all users) and only unconsumed rows when activeOnly.
func (s *Store) CountVerificationTokens(ctx context.Context, purpose, userID string, activeOnly bool) (int, error) {
	q := `SELECT count(*) FROM email_verification_tokens WHERE purpose = $1`
	args := []any{purpose}
	if userID != "" {
		args = append(args, userID)
		q += ` AND user_id = $2`
	}
	if activeOnly {
		q += ` AND consumed_at IS NULL`
	}
	var n int
	err := s.pool.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

// CountActiveUsers counts non-deleted users.
func (s *Store) CountActiveUsers(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

// UserDisplayName returns the display_name for a user id (any delete state).
func (s *Store) UserDisplayName(ctx context.Context, userID string) (string, error) {
	var name string
	err := s.pool.QueryRow(ctx, `SELECT display_name FROM users WHERE id = $1`, userID).Scan(&name)
	return name, err
}

// RecurringNextRun returns the next_run_at of a recurring template.
func (s *Store) RecurringNextRun(ctx context.Context, id string) (time.Time, error) {
	var t time.Time
	err := s.pool.QueryRow(ctx, `SELECT next_run_at FROM recurring_expenses WHERE id = $1`, id).Scan(&t)
	return t.UTC(), err
}
