package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type SmtpRepo struct{ s *Store }

// Get returns the single config row, or ErrNotFound if none has been written.
func (r *SmtpRepo) Get(ctx context.Context) (*repo.SmtpConfig, error) {
	var c repo.SmtpConfig
	var updated string
	err := r.s.db.QueryRowContext(ctx, `
		SELECT host, port, username, password_encrypted, from_address, tls_mode, updated_at, updated_by
		FROM smtp_config WHERE id = 1
	`).Scan(&c.Host, &c.Port, &c.Username, &c.PasswordEncrypted, &c.FromAddress, &c.TLSMode, &updated, &c.UpdatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.UpdatedAt = scanTS(updated)
	return &c, nil
}

// Upsert writes the single-row config. passwordEncrypted is written verbatim;
// pass leavePassword=true to keep the existing ciphertext untouched.
func (r *SmtpRepo) Upsert(ctx context.Context, c *repo.SmtpConfig, leavePassword bool) error {
	now := tsVal(time.Now().UTC())
	// Two upsert variants so leaving the password alone doesn't overwrite the
	// stored ciphertext with NULL.
	if leavePassword {
		_, err := r.s.db.ExecContext(ctx, `
			INSERT INTO smtp_config (id, host, port, username, from_address, tls_mode, updated_at, updated_by)
			VALUES (1, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				host = excluded.host,
				port = excluded.port,
				username = excluded.username,
				from_address = excluded.from_address,
				tls_mode = excluded.tls_mode,
				updated_at = excluded.updated_at,
				updated_by = excluded.updated_by
		`, c.Host, c.Port, c.Username, c.FromAddress, c.TLSMode, now, c.UpdatedBy)
		return err
	}
	_, err := r.s.db.ExecContext(ctx, `
		INSERT INTO smtp_config (id, host, port, username, password_encrypted, from_address, tls_mode, updated_at, updated_by)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			host = excluded.host,
			port = excluded.port,
			username = excluded.username,
			password_encrypted = excluded.password_encrypted,
			from_address = excluded.from_address,
			tls_mode = excluded.tls_mode,
			updated_at = excluded.updated_at,
			updated_by = excluded.updated_by
	`, c.Host, c.Port, c.Username, c.PasswordEncrypted, c.FromAddress, c.TLSMode, now, c.UpdatedBy)
	return err
}
