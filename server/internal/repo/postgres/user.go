package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

type UserRepo struct{ pool *pgxpool.Pool }

const userCols = `id, email_hash, email_encrypted, display_name, password_hash, created_at, deleted_at, avatar_updated_at, week_start, role, email_verified_at, notification_prefs`

func scanUser(row pgx.Row, u *repo.User) error {
	return row.Scan(&u.ID, &u.EmailHash, &u.EmailEncrypted, &u.DisplayName,
		&u.PasswordHash, &u.CreatedAt, &u.DeletedAt, &u.AvatarUpdatedAt, &u.WeekStart,
		&u.Role, &u.EmailVerifiedAt, &u.NotificationPrefs)
}

func (r *UserRepo) Create(ctx context.Context, u *repo.User) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO users (email_hash, email_encrypted, display_name, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`, u.EmailHash, u.EmailEncrypted, u.DisplayName, u.PasswordHash).Scan(&u.ID, &u.CreatedAt)
}

// FindByEmailHash returns only non-deleted users.
func (r *UserRepo) FindByEmailHash(ctx context.Context, emailHash []byte) (*repo.User, error) {
	var u repo.User
	err := scanUser(r.pool.QueryRow(ctx, `
		SELECT `+userCols+`
		FROM users WHERE email_hash = $1 AND deleted_at IS NULL
	`, emailHash), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByID returns the user regardless of soft-delete state. Callers that want
// only active users should check DeletedAt themselves.
func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*repo.User, error) {
	var u repo.User
	err := scanUser(r.pool.QueryRow(ctx, `
		SELECT `+userCols+`
		FROM users WHERE id = $1
	`, id), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateDisplayName renames the user.
func (r *UserRepo) UpdateDisplayName(ctx context.Context, id uuid.UUID, name string) error {
	ct, err := r.pool.Exec(ctx, `
		UPDATE users SET display_name = $2
		WHERE id = $1 AND deleted_at IS NULL
	`, id, name)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// UpdateWeekStart sets the user's preferred first day of the week.
// Caller must validate that v is in the allowed set (currently 0 or 1).
func (r *UserRepo) UpdateWeekStart(ctx context.Context, id uuid.UUID, v int16) error {
	ct, err := r.pool.Exec(ctx, `
		UPDATE users SET week_start = $2
		WHERE id = $1 AND deleted_at IS NULL
	`, id, v)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// UpdatePasswordHash rotates the encoded Argon2id hash.
func (r *UserRepo) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	return r.updatePasswordHash(ctx, r.pool, id, hash)
}

// UpdatePasswordHashTx rotates the hash inside a caller-owned transaction.
func (r *UserRepo) UpdatePasswordHashTx(ctx context.Context, tx repo.Tx, id uuid.UUID, hash string) error {
	return r.updatePasswordHash(ctx, native(tx), id, hash)
}

func (r *UserRepo) updatePasswordHash(ctx context.Context, q dbtx, id uuid.UUID, hash string) error {
	ct, err := q.Exec(ctx, `
		UPDATE users SET password_hash = $2
		WHERE id = $1 AND deleted_at IS NULL
	`, id, hash)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// SetAvatar stores the normalized 8x8 PNG bytes and stamps avatar_updated_at.
// Pass nil png to clear the avatar.
func (r *UserRepo) SetAvatar(ctx context.Context, id uuid.UUID, png []byte) error {
	var tag pgconn.CommandTag
	var err error
	if png == nil {
		tag, err = r.pool.Exec(ctx, `
			UPDATE users SET avatar = NULL, avatar_updated_at = NULL
			WHERE id = $1 AND deleted_at IS NULL
		`, id)
	} else {
		tag, err = r.pool.Exec(ctx, `
			UPDATE users SET avatar = $2, avatar_updated_at = now()
			WHERE id = $1 AND deleted_at IS NULL
		`, id, png)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// GetAvatar returns the raw PNG bytes or ErrNotFound when the user has no avatar.
func (r *UserRepo) GetAvatar(ctx context.Context, id uuid.UUID) ([]byte, error) {
	var png []byte
	err := r.pool.QueryRow(ctx, `
		SELECT avatar FROM users WHERE id = $1
	`, id).Scan(&png)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if png == nil {
		return nil, repo.ErrNotFound
	}
	return png, nil
}

// CountActive returns the number of non-deleted users. Used by the bootstrap
// path inside a transaction with an advisory lock to detect "first user".
func (r *UserRepo) CountActive(ctx context.Context, tx repo.Tx) (int, error) {
	q := resolve(r.pool, tx)
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM users WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

// CountActiveAdmins returns the number of non-deleted admins. Used by the
// last-admin guard before delete/demote.
func (r *UserRepo) CountActiveAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE role='admin' AND deleted_at IS NULL`).Scan(&n)
	return n, err
}

// SetRole sets the user's role. Caller is responsible for the last-admin guard
// when demoting.
func (r *UserRepo) SetRole(ctx context.Context, id uuid.UUID, role string) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE users SET role = $2 WHERE id = $1 AND deleted_at IS NULL`, id, role)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// ListPaginated returns a page of users ordered by created_at DESC.
// includeDeleted controls whether soft-deleted rows are returned.
// Returns the rows plus the total count for paginator UIs.
func (r *UserRepo) ListPaginated(ctx context.Context, limit, offset int, includeDeleted bool) ([]repo.User, int, error) {
	where := "WHERE deleted_at IS NULL"
	if includeDeleted {
		where = ""
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users `+where).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+userCols+` FROM users
		`+where+`
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []repo.User
	for rows.Next() {
		var u repo.User
		if err := scanUser(rows, &u); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

// CreateWithRole inserts a user with an explicit role. Returns the created
// user's id and created_at. Intended for admin-driven user creation and the
// bootstrap-admin path. The caller is responsible for hashing the password.
// If tx is non-nil the insert participates in it; otherwise a new pool query
// is used.
func (r *UserRepo) CreateWithRole(ctx context.Context, tx repo.Tx, u *repo.User, role string) error {
	q := resolve(r.pool, tx)
	return q.QueryRow(ctx, `
		INSERT INTO users (email_hash, email_encrypted, display_name, password_hash, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, u.EmailHash, u.EmailEncrypted, u.DisplayName, u.PasswordHash, role).
		Scan(&u.ID, &u.CreatedAt)
}

// MarkEmailVerified stamps email_verified_at = now() and is idempotent: a
// second call is a no-op rather than an error so the verify flow can be
// safely retried by the user without producing a 404.
func (r *UserRepo) MarkEmailVerified(ctx context.Context, tx repo.Tx, id uuid.UUID) error {
	q := resolve(r.pool, tx)
	_, err := q.Exec(ctx, `
		UPDATE users SET email_verified_at = now()
		WHERE id = $1 AND deleted_at IS NULL AND email_verified_at IS NULL
	`, id)
	return err
}

// UpdateEmail replaces the user's email_hash and email_encrypted atomically.
// An active-email uniqueness violation (the partial unique index
// `users_email_hash_active_key`) is mapped to repo.ErrConflict.
func (r *UserRepo) UpdateEmail(ctx context.Context, tx repo.Tx, id uuid.UUID, emailHash, emailEnc []byte) error {
	q := resolve(r.pool, tx)
	ct, err := q.Exec(ctx, `
		UPDATE users SET email_hash = $2, email_encrypted = $3
		WHERE id = $1 AND deleted_at IS NULL
	`, id, emailHash, emailEnc)
	if err != nil {
		if isUniqueViolation(err) {
			return repo.ErrConflict
		}
		return err
	}
	if ct.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// UpdateNotificationPrefs writes the JSONB blob verbatim. The caller is
// responsible for validating keys (the service layer rejects unknown ones).
func (r *UserRepo) UpdateNotificationPrefs(ctx context.Context, id uuid.UUID, prefs []byte) error {
	ct, err := r.pool.Exec(ctx, `
		UPDATE users SET notification_prefs = $2
		WHERE id = $1 AND deleted_at IS NULL
	`, id, prefs)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// FindOrCreateStub returns an existing non-deleted user matching emailHash,
// or inserts a new placeholder row when none exists. Stubs are non-loginable
// (the caller passes an unguessable scrambled password hash) and exist solely
// so importer flows can attach foreign keys for users who haven't registered
// yet. The same return shape is used in both branches so callers can't
// distinguish "real" from "stub" via the response, which preserves the
// no-enumeration property of the import endpoint.
func (r *UserRepo) FindOrCreateStub(ctx context.Context, tx repo.Tx, emailHash, emailEnc []byte, displayName, scrambledPwHash string) (*repo.User, error) {
	q := resolve(r.pool, tx)
	var u repo.User
	err := scanUser(q.QueryRow(ctx, `
		SELECT `+userCols+`
		FROM users WHERE email_hash = $1 AND deleted_at IS NULL
	`, emailHash), &u)
	if err == nil {
		return &u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	row := q.QueryRow(ctx, `
		INSERT INTO users (email_hash, email_encrypted, display_name, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING `+userCols+`
	`, emailHash, emailEnc, displayName, scrambledPwHash)
	if err := scanUser(row, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// SoftDelete marks the account as deleted, scrubs identifying fields, and
// renames the user to a tombstone referencing their own UUID so historical
// ledger entries stay traceable. Refresh tokens are revoked separately.
func (r *UserRepo) SoftDelete(ctx context.Context, id uuid.UUID, tombstone string, scrambledHash, scrambledEnc []byte, scrambledPwHash string) error {
	ct, err := r.pool.Exec(ctx, `
		UPDATE users SET
			deleted_at        = now(),
			display_name      = $2,
			email_hash        = $3,
			email_encrypted   = $4,
			password_hash     = $5,
			avatar            = NULL,
			avatar_updated_at = NULL
		WHERE id = $1 AND deleted_at IS NULL
	`, id, tombstone, scrambledHash, scrambledEnc, scrambledPwHash)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	return nil
}
