package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SmtpConfig struct {
	Host              string
	Port              int
	Username          *string
	PasswordEncrypted []byte
	FromAddress       string
	TLSMode           string
	UpdatedAt         time.Time
	UpdatedBy         *uuid.UUID
}

type SmtpRepo struct {
	pool *pgxpool.Pool
}

func NewSmtpRepo(p *pgxpool.Pool) *SmtpRepo { return &SmtpRepo{pool: p} }

// Get returns the single config row, or ErrNotFound if none has been written.
func (r *SmtpRepo) Get(ctx context.Context) (*SmtpConfig, error) {
	var c SmtpConfig
	err := r.pool.QueryRow(ctx, `
		SELECT host, port, username, password_encrypted, from_address, tls_mode, updated_at, updated_by
		FROM smtp_config WHERE id = true
	`).Scan(&c.Host, &c.Port, &c.Username, &c.PasswordEncrypted, &c.FromAddress, &c.TLSMode, &c.UpdatedAt, &c.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Upsert writes the single-row config. passwordEncrypted is written verbatim;
// pass nil to leave the existing ciphertext untouched, or an empty slice to
// clear it.
func (r *SmtpRepo) Upsert(ctx context.Context, c *SmtpConfig, leavePassword bool) error {
	// Two upsert variants because COALESCE on bytea($) needs a typed NULL.
	if leavePassword {
		_, err := r.pool.Exec(ctx, `
			INSERT INTO smtp_config (id, host, port, username, from_address, tls_mode, updated_at, updated_by)
			VALUES (true, $1, $2, $3, $4, $5, now(), $6)
			ON CONFLICT (id) DO UPDATE SET
				host = EXCLUDED.host,
				port = EXCLUDED.port,
				username = EXCLUDED.username,
				from_address = EXCLUDED.from_address,
				tls_mode = EXCLUDED.tls_mode,
				updated_at = now(),
				updated_by = EXCLUDED.updated_by
		`, c.Host, c.Port, c.Username, c.FromAddress, c.TLSMode, c.UpdatedBy)
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO smtp_config (id, host, port, username, password_encrypted, from_address, tls_mode, updated_at, updated_by)
		VALUES (true, $1, $2, $3, $4, $5, $6, now(), $7)
		ON CONFLICT (id) DO UPDATE SET
			host = EXCLUDED.host,
			port = EXCLUDED.port,
			username = EXCLUDED.username,
			password_encrypted = EXCLUDED.password_encrypted,
			from_address = EXCLUDED.from_address,
			tls_mode = EXCLUDED.tls_mode,
			updated_at = now(),
			updated_by = EXCLUDED.updated_by
	`, c.Host, c.Port, c.Username, c.PasswordEncrypted, c.FromAddress, c.TLSMode, c.UpdatedBy)
	return err
}
