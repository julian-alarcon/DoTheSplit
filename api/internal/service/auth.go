// Package service holds business logic that sits between HTTP handlers and repositories.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julian-alarcon/dothesplit/api/internal/crypto"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already registered")
)

type AuthService struct {
	users    *repo.UserRepo
	sessions *repo.SessionRepo
	audit    *repo.AuditRepo
	pool     *pgxpool.Pool
	email    *crypto.EmailCipher
	pepper   []byte
	sessTTL  time.Duration

	// stepUpFails counts recent failed step-up password verifications keyed
	// by user ID, so handlers performing destructive admin actions can short-
	// circuit before they even hash a guess. Lazy-initialized.
	stepUpFails sync.Map // map[uuid.UUID]*stepUpCounter
}

type stepUpCounter struct {
	mu       sync.Mutex
	count    int
	windowAt time.Time
}

// stepUpWindow / stepUpMaxFails define the per-user lockout policy. Five bad
// guesses inside a minute lock the account out of step-up for the rest of the
// window. The counter resets on the next successful verify or after the
// window expires.
const (
	stepUpWindow   = time.Minute
	stepUpMaxFails = 5
)

// ErrStepUpRateLimited is returned when too many failed step-up verifications
// have piled up for one user inside the rate-limit window. Handlers map this
// to HTTP 423 Locked.
var ErrStepUpRateLimited = errors.New("step-up rate limited")

func NewAuthService(pool *pgxpool.Pool, users *repo.UserRepo, sessions *repo.SessionRepo, audit *repo.AuditRepo, email *crypto.EmailCipher, pepper []byte, sessTTL time.Duration) *AuthService {
	return &AuthService{
		users:    users,
		sessions: sessions,
		audit:    audit,
		pool:     pool,
		email:    email,
		pepper:   pepper,
		sessTTL:  sessTTL,
	}
}

// User is a service-layer projection of a user with the decrypted email.
type User struct {
	ID                 uuid.UUID
	Email              string
	DisplayName        string
	CreatedAt          time.Time
	HasAvatar          bool
	AvatarUpdatedAt    *time.Time
	DeletedAt          *time.Time
	WeekStart          int16
	Timezone           *string
	IsAdmin            bool
	MustChangePassword bool
}

func (s *AuthService) toUser(u *repo.User) (*User, error) {
	email, err := s.email.Decrypt(u.EmailEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt email: %w", err)
	}
	return &User{
		ID:                 u.ID,
		Email:              email,
		DisplayName:        u.DisplayName,
		CreatedAt:          u.CreatedAt,
		HasAvatar:          u.AvatarUpdatedAt != nil,
		AvatarUpdatedAt:    u.AvatarUpdatedAt,
		DeletedAt:          u.DeletedAt,
		WeekStart:          u.WeekStart,
		Timezone:           u.Timezone,
		IsAdmin:            u.Role == "admin",
		MustChangePassword: u.MustChangePassword,
	}, nil
}

// Register creates a user and opens a session. Returns the user and the plaintext
// session token (to be set as a cookie by the handler).
//
// First-user bootstrap: if there are zero non-deleted users at the moment of
// registration, the new account is created with role='admin'. The "first user
// wins" check uses a session-level pg_advisory_xact_lock so two concurrent
// registrations cannot both observe `count = 0` and both promote.
func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*User, string, error) {
	email = strings.TrimSpace(email)
	displayName = strings.TrimSpace(displayName)
	if email == "" || password == "" || displayName == "" {
		return nil, "", errors.New("email, password, and display_name are required")
	}
	if len(password) < 10 {
		return nil, "", errors.New("password must be at least 10 characters")
	}

	emailHash := s.email.HashEmail(email)
	if _, err := s.users.FindByEmailHash(ctx, emailHash); err == nil {
		return nil, "", ErrEmailTaken
	} else if !errors.Is(err, repo.ErrNotFound) {
		return nil, "", err
	}

	emailEnc, err := s.email.Encrypt(email)
	if err != nil {
		return nil, "", err
	}
	pwdHash, err := crypto.HashPassword(password, s.pepper)
	if err != nil {
		return nil, "", err
	}

	u := &repo.User{
		EmailHash:      emailHash,
		EmailEncrypted: emailEnc,
		DisplayName:    displayName,
		PasswordHash:   pwdHash,
	}

	// Bootstrap-admin path: serialize concurrent first registrations on a
	// dedicated advisory key, count active users inside the same tx, and
	// promote only if the count is zero. The advisory lock is released on
	// commit/rollback automatically because it's transaction-scoped.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('admin_bootstrap'))`); err != nil {
		return nil, "", err
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE deleted_at IS NULL`).Scan(&n); err != nil {
		return nil, "", err
	}
	role := "user"
	if n == 0 {
		role = "admin"
	}
	u.Role = role
	if err := s.users.CreateWithRole(ctx, tx, u, role, false); err != nil {
		return nil, "", err
	}
	if role == "admin" {
		meta, _ := json.Marshal(map[string]any{"reason": "first_user"})
		if err := s.audit.Insert(ctx, tx, &repo.AuditEntry{
			ActorUserID: u.ID,
			Action:      "bootstrap_admin",
			Success:     true,
			Metadata:    meta,
		}); err != nil {
			return nil, "", err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}

	token, err := s.issueSession(ctx, u.ID)
	if err != nil {
		return nil, "", err
	}
	out, err := s.toUser(u)
	if err != nil {
		return nil, "", err
	}
	return out, token, nil
}

// Login verifies credentials and issues a session. Returns (user, token).
// Authentication-failure errors intentionally share a single sentinel to avoid
// enumeration of which users exist.
func (s *AuthService) Login(ctx context.Context, email, password string) (*User, string, error) {
	emailHash := s.email.HashEmail(email)
	u, err := s.users.FindByEmailHash(ctx, emailHash)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, "", ErrInvalidCredentials
	}
	if err != nil {
		return nil, "", err
	}
	ok, err := crypto.VerifyPassword(u.PasswordHash, password, s.pepper)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", ErrInvalidCredentials
	}
	token, err := s.issueSession(ctx, u.ID)
	if err != nil {
		return nil, "", err
	}
	out, err := s.toUser(u)
	if err != nil {
		return nil, "", err
	}
	return out, token, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.sessions.DeleteByTokenHash(ctx, hashToken(token))
}

// Resolve returns the user for a raw session token, or ErrInvalidCredentials.
func (s *AuthService) Resolve(ctx context.Context, token string) (*User, error) {
	if token == "" {
		return nil, ErrInvalidCredentials
	}
	sess, err := s.sessions.FindByTokenHash(ctx, hashToken(token))
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	u, err := s.users.FindByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	if u.DeletedAt != nil {
		return nil, ErrInvalidCredentials
	}
	return s.toUser(u)
}

// IssueSession creates a fresh session token for the given user. Exposed for
// handlers that need to refresh the cookie after wiping all sessions (e.g. on
// password change).
func (s *AuthService) IssueSession(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.issueSession(ctx, userID)
}

func (s *AuthService) issueSession(ctx context.Context, userID uuid.UUID) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sess := &repo.Session{
		UserID:    userID,
		TokenHash: hashToken(token),
		ExpiresAt: time.Now().Add(s.sessTTL),
	}
	if err := s.sessions.Create(ctx, sess); err != nil {
		return "", err
	}
	return token, nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// VerifyPassword re-checks a user's password for step-up authorization.
// Returns ErrStepUpRateLimited when too many recent failures pile up;
// ErrInvalidCredentials on a bad password; nil on success. Failures and the
// user/admin-not-found case are intentionally indistinguishable to callers.
func (s *AuthService) VerifyPassword(ctx context.Context, userID uuid.UUID, password string) error {
	if s.lockedOut(userID) {
		return ErrStepUpRateLimited
	}
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		s.recordStepUpFailure(userID)
		return ErrInvalidCredentials
	}
	if u.DeletedAt != nil {
		s.recordStepUpFailure(userID)
		return ErrInvalidCredentials
	}
	ok, err := crypto.VerifyPassword(u.PasswordHash, password, s.pepper)
	if err != nil {
		return err
	}
	if !ok {
		s.recordStepUpFailure(userID)
		return ErrInvalidCredentials
	}
	s.clearStepUpFailures(userID)
	return nil
}

// AdminChangeUserPassword rotates the target user's password hash, sets the
// must_change_password flag in the same UPDATE, and revokes all of the
// target's sessions. The target's *next* login will succeed; everything they
// do until they POST /v1/me/password is gated by EnforcePasswordChange.
func (s *AuthService) AdminChangeUserPassword(ctx context.Context, targetID uuid.UUID, newPassword string) error {
	if len(newPassword) < 10 {
		return errors.New("password must be at least 10 characters")
	}
	hash, err := crypto.HashPassword(newPassword, s.pepper)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePasswordHashWithFlag(ctx, targetID, hash, true); err != nil {
		return err
	}
	return s.sessions.DeleteAllForUser(ctx, targetID)
}

func (s *AuthService) lockedOut(userID uuid.UUID) bool {
	v, ok := s.stepUpFails.Load(userID)
	if !ok {
		return false
	}
	c := v.(*stepUpCounter)
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.windowAt) > stepUpWindow {
		return false
	}
	return c.count >= stepUpMaxFails
}

func (s *AuthService) recordStepUpFailure(userID uuid.UUID) {
	v, _ := s.stepUpFails.LoadOrStore(userID, &stepUpCounter{})
	c := v.(*stepUpCounter)
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.windowAt) > stepUpWindow {
		c.count = 0
		c.windowAt = time.Now()
	}
	c.count++
}

func (s *AuthService) clearStepUpFailures(userID uuid.UUID) {
	s.stepUpFails.Delete(userID)
}
