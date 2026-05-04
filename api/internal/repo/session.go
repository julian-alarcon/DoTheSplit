package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	CreatedAt time.Time
}

type SessionRepo struct {
	pool *pgxpool.Pool
}

func NewSessionRepo(p *pgxpool.Pool) *SessionRepo { return &SessionRepo{pool: p} }

func (r *SessionRepo) Create(ctx context.Context, s *Session) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, s.UserID, s.TokenHash, s.ExpiresAt).Scan(&s.ID, &s.CreatedAt)
}

func (r *SessionRepo) FindByTokenHash(ctx context.Context, tokenHash []byte) (*Session, error) {
	var s Session
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM sessions WHERE token_hash = $1 AND expires_at > now()
	`, tokenHash).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SessionRepo) DeleteByTokenHash(ctx context.Context, tokenHash []byte) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// DeleteAllForUser removes every session belonging to the user; used during
// account soft-delete so the current cookie stops working immediately.
func (r *SessionRepo) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}
