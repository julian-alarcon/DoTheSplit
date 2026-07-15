package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

type UserRepo struct{ s *Store }

const userCols = `id, email_hash, email_encrypted, display_name, password_hash, created_at, deleted_at, avatar_updated_at, week_start, role, email_verified_at, notification_prefs`

// scanUser scans userCols into u, converting the TEXT timestamp columns.
func scanUser(sc interface{ Scan(...any) error }, u *repo.User) error {
	var created string
	var deleted, avatarUpdated, emailVerified *string
	if err := sc.Scan(&u.ID, &u.EmailHash, &u.EmailEncrypted, &u.DisplayName,
		&u.PasswordHash, &created, &deleted, &avatarUpdated, &u.WeekStart,
		&u.Role, &emailVerified, &u.NotificationPrefs); err != nil {
		return err
	}
	u.CreatedAt = scanTS(created)
	u.DeletedAt = scanTSPtr(deleted)
	u.AvatarUpdatedAt = scanTSPtr(avatarUpdated)
	u.EmailVerifiedAt = scanTSPtr(emailVerified)
	return nil
}

func (r *UserRepo) Create(ctx context.Context, u *repo.User) error {
	u.ID = uuid.New()
	u.CreatedAt = time.Now().UTC()
	_, err := r.s.db.ExecContext(ctx, `
		INSERT INTO users (id, email_hash, email_encrypted, display_name, password_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, u.ID, u.EmailHash, u.EmailEncrypted, u.DisplayName, u.PasswordHash, tsVal(u.CreatedAt))
	return err
}

// FindByEmailHash returns only non-deleted users.
func (r *UserRepo) FindByEmailHash(ctx context.Context, emailHash []byte) (*repo.User, error) {
	var u repo.User
	err := scanUser(r.s.db.QueryRowContext(ctx, `
		SELECT `+userCols+`
		FROM users WHERE email_hash = ? AND deleted_at IS NULL
	`, emailHash), &u)
	if errors.Is(err, sql.ErrNoRows) {
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
	err := scanUser(r.s.db.QueryRowContext(ctx, `
		SELECT `+userCols+`
		FROM users WHERE id = ?
	`, id), &u)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateDisplayName renames the user.
func (r *UserRepo) UpdateDisplayName(ctx context.Context, id uuid.UUID, name string) error {
	res, err := r.s.db.ExecContext(ctx, `
		UPDATE users SET display_name = ?
		WHERE id = ? AND deleted_at IS NULL
	`, name, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// UpdateWeekStart sets the user's preferred first day of the week.
// Caller must validate that v is in the allowed set (currently 0 or 1).
func (r *UserRepo) UpdateWeekStart(ctx context.Context, id uuid.UUID, v int16) error {
	res, err := r.s.db.ExecContext(ctx, `
		UPDATE users SET week_start = ?
		WHERE id = ? AND deleted_at IS NULL
	`, v, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// UpdatePasswordHash rotates the encoded Argon2id hash.
func (r *UserRepo) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	return r.updatePasswordHash(ctx, r.s.db, id, hash)
}

// UpdatePasswordHashTx rotates the hash inside a caller-owned transaction.
func (r *UserRepo) UpdatePasswordHashTx(ctx context.Context, tx repo.Tx, id uuid.UUID, hash string) error {
	return r.updatePasswordHash(ctx, native(tx).tx, id, hash)
}

func (r *UserRepo) updatePasswordHash(ctx context.Context, q dbtx, id uuid.UUID, hash string) error {
	res, err := q.ExecContext(ctx, `
		UPDATE users SET password_hash = ?
		WHERE id = ? AND deleted_at IS NULL
	`, hash, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// SetAvatar stores the normalized 8x8 PNG bytes and stamps avatar_updated_at.
// Pass nil png to clear the avatar.
func (r *UserRepo) SetAvatar(ctx context.Context, id uuid.UUID, png []byte) error {
	var res sql.Result
	var err error
	if png == nil {
		res, err = r.s.db.ExecContext(ctx, `
			UPDATE users SET avatar = NULL, avatar_updated_at = NULL
			WHERE id = ? AND deleted_at IS NULL
		`, id)
	} else {
		res, err = r.s.db.ExecContext(ctx, `
			UPDATE users SET avatar = ?, avatar_updated_at = ?
			WHERE id = ? AND deleted_at IS NULL
		`, png, tsVal(time.Now().UTC()), id)
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// GetAvatar returns the raw PNG bytes or ErrNotFound when the user has no avatar.
func (r *UserRepo) GetAvatar(ctx context.Context, id uuid.UUID) ([]byte, error) {
	var png []byte
	err := r.s.db.QueryRowContext(ctx, `
		SELECT avatar FROM users WHERE id = ?
	`, id).Scan(&png)
	if errors.Is(err, sql.ErrNoRows) {
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
// path inside a transaction to detect "first user".
func (r *UserRepo) CountActive(ctx context.Context, tx repo.Tx) (int, error) {
	q := r.s.resolve(tx)
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE deleted_at IS NULL`).Scan(&n)
	return n, err
}

// CountActiveAdmins returns the number of non-deleted admins. Used by the
// last-admin guard before delete/demote.
func (r *UserRepo) CountActiveAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM users WHERE role='admin' AND deleted_at IS NULL`).Scan(&n)
	return n, err
}

// SetRole sets the user's role. Caller is responsible for the last-admin guard
// when demoting.
func (r *UserRepo) SetRole(ctx context.Context, id uuid.UUID, role string) error {
	res, err := r.s.db.ExecContext(ctx,
		`UPDATE users SET role = ? WHERE id = ? AND deleted_at IS NULL`, role, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
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
	if err := r.s.db.QueryRowContext(ctx, `SELECT count(*) FROM users `+where).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.s.db.QueryContext(ctx, `
		SELECT `+userCols+` FROM users
		`+where+`
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
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

// CreateWithRole inserts a user with an explicit role. Intended for admin-driven
// user creation and the bootstrap-admin path. The caller is responsible for
// hashing the password. If tx is non-nil the insert participates in it;
// otherwise a new pool query is used.
func (r *UserRepo) CreateWithRole(ctx context.Context, tx repo.Tx, u *repo.User, role string) error {
	q := r.s.resolve(tx)
	u.ID = uuid.New()
	u.CreatedAt = time.Now().UTC()
	_, err := q.ExecContext(ctx, `
		INSERT INTO users (id, email_hash, email_encrypted, display_name, password_hash, role, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, u.ID, u.EmailHash, u.EmailEncrypted, u.DisplayName, u.PasswordHash, role, tsVal(u.CreatedAt))
	return err
}

// MarkEmailVerified stamps email_verified_at = now() and is idempotent: a
// second call is a no-op rather than an error so the verify flow can be
// safely retried by the user without producing a 404.
func (r *UserRepo) MarkEmailVerified(ctx context.Context, tx repo.Tx, id uuid.UUID) error {
	q := r.s.resolve(tx)
	_, err := q.ExecContext(ctx, `
		UPDATE users SET email_verified_at = ?
		WHERE id = ? AND deleted_at IS NULL AND email_verified_at IS NULL
	`, tsVal(time.Now().UTC()), id)
	return err
}

// UpdateEmail replaces the user's email_hash and email_encrypted atomically.
// An active-email uniqueness violation (the partial unique index
// `users_email_hash_active_key`) is mapped to repo.ErrConflict.
func (r *UserRepo) UpdateEmail(ctx context.Context, tx repo.Tx, id uuid.UUID, emailHash, emailEnc []byte) error {
	q := r.s.resolve(tx)
	res, err := q.ExecContext(ctx, `
		UPDATE users SET email_hash = ?, email_encrypted = ?
		WHERE id = ? AND deleted_at IS NULL
	`, emailHash, emailEnc, id)
	if err != nil {
		if isUniqueViolation(err) {
			return repo.ErrConflict
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// UpdateNotificationPrefs writes the JSON blob verbatim. The caller is
// responsible for validating keys (the service layer rejects unknown ones).
func (r *UserRepo) UpdateNotificationPrefs(ctx context.Context, id uuid.UUID, prefs []byte) error {
	res, err := r.s.db.ExecContext(ctx, `
		UPDATE users SET notification_prefs = ?
		WHERE id = ? AND deleted_at IS NULL
	`, prefs, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
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
	q := r.s.resolve(tx)
	var u repo.User
	err := scanUser(q.QueryRowContext(ctx, `
		SELECT `+userCols+`
		FROM users WHERE email_hash = ? AND deleted_at IS NULL
	`, emailHash), &u)
	if err == nil {
		return &u, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	id := uuid.New()
	created := time.Now().UTC()
	if _, err := q.ExecContext(ctx, `
		INSERT INTO users (id, email_hash, email_encrypted, display_name, password_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, emailHash, emailEnc, displayName, scrambledPwHash, tsVal(created)); err != nil {
		return nil, err
	}
	if err := scanUser(q.QueryRowContext(ctx, `
		SELECT `+userCols+`
		FROM users WHERE id = ?
	`, id), &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// SoftDelete marks the account as deleted, scrubs identifying fields, and
// renames the user to a tombstone referencing their own UUID so historical
// ledger entries stay traceable. Refresh tokens are revoked separately.
func (r *UserRepo) SoftDelete(ctx context.Context, id uuid.UUID, tombstone string, scrambledHash, scrambledEnc []byte, scrambledPwHash string) error {
	res, err := r.s.db.ExecContext(ctx, `
		UPDATE users SET
			deleted_at        = ?,
			display_name      = ?,
			email_hash        = ?,
			email_encrypted   = ?,
			password_hash     = ?,
			avatar            = NULL,
			avatar_updated_at = NULL
		WHERE id = ? AND deleted_at IS NULL
	`, tsVal(time.Now().UTC()), tombstone, scrambledHash, scrambledEnc, scrambledPwHash, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	return nil
}
