package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type SetupRepo struct{ s *Store }

// Get returns the row, or ErrNotFound when no install ceremony has yet been
// kicked off (the boot path lazily inserts it).
func (r *SetupRepo) Get(ctx context.Context) (*repo.Setup, error) {
	return r.get(ctx, r.s.db)
}

// GetForUpdate reads the single install row. SQLite has no FOR UPDATE and its
// single writer means no lock is needed; it just reads inside tx. Returns
// ErrNotFound when no row exists yet.
func (r *SetupRepo) GetForUpdate(ctx context.Context, tx repo.Tx) (*repo.Setup, error) {
	return r.get(ctx, native(tx).tx)
}

func (r *SetupRepo) get(ctx context.Context, q dbtx) (*repo.Setup, error) {
	var s repo.Setup
	var generated string
	var completed *string
	err := q.QueryRowContext(ctx, `
		SELECT token_hash, token_generated_at, completed_at, completed_by
		FROM app_setup WHERE id = 1
	`).Scan(&s.TokenHash, &generated, &completed, &s.CompletedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.TokenGeneratedAt = scanTS(generated)
	s.CompletedAt = scanTSPtr(completed)
	return &s, nil
}

// Upsert writes the (hash, generatedAt) pair. The UPDATE branch is gated on
// `completed_at IS NULL` so a once-completed row can never have its token
// rotated underneath it - re-opening setup is impossible without manual SQL.
func (r *SetupRepo) Upsert(ctx context.Context, hash []byte, at time.Time) error {
	_, err := r.s.db.ExecContext(ctx, `
		INSERT INTO app_setup (id, token_hash, token_generated_at)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			token_hash = excluded.token_hash,
			token_generated_at = excluded.token_generated_at
		WHERE app_setup.completed_at IS NULL
	`, hash, tsVal(at))
	return err
}

// Complete marks setup as finished. tx may be non-nil so completion can be
// committed atomically with the admin-user creation in SetupService.
func (r *SetupRepo) Complete(ctx context.Context, tx repo.Tx, by uuid.UUID) error {
	q := r.s.resolve(tx)
	_, err := q.ExecContext(ctx, `
		UPDATE app_setup
		   SET completed_at = ?, completed_by = ?
		 WHERE id = 1 AND completed_at IS NULL
	`, tsVal(time.Now().UTC()), by)
	return err
}

// Locked reports whether the install ceremony is finished. Returns false (=>
// "still in setup mode") when no row exists yet OR the row's completed_at is
// null. Returns true when completed_at IS NOT NULL.
func (r *SetupRepo) Locked(ctx context.Context) (bool, error) {
	var locked bool
	err := r.s.db.QueryRowContext(ctx, `
		SELECT COALESCE(
			(SELECT completed_at IS NOT NULL FROM app_setup WHERE id = 1),
			0
		)
	`).Scan(&locked)
	return locked, err
}
