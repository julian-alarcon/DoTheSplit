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

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julian-alarcon/dothesplit/api/internal/crypto"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already registered")
	// ErrSetupRequired is returned by Register when the instance is still in
	// first-run setup mode. Handlers map it to 403 with code='setup_required'.
	ErrSetupRequired = errors.New("setup required")
	// ErrEmailUnverified is returned by Login when the account exists but the
	// user has not yet confirmed their email address. Handlers map it to
	// 403 with code='email_unverified' so the frontend can route to /verify.
	ErrEmailUnverified = errors.New("email not verified")
	// ErrInvalidCode and friends serve the verify/confirm flows.
	ErrInvalidCode       = errors.New("invalid code")
	ErrCodeExpired       = errors.New("code expired or already used")
	ErrVerifyRateLimited = errors.New("verification rate limited")
)

// SetupLocker is the minimal interface AuthService needs from the setup
// repo. Defined here (and not in the setup file) to keep the dep graph
// acyclic: AuthService → SetupLocker, and SetupService → AuthService.
type SetupLocker interface {
	Locked(ctx context.Context) (bool, error)
}

type AuthService struct {
	users        *repo.UserRepo
	refresh      *repo.RefreshTokenRepo
	audit        *repo.AuditRepo
	verification *repo.VerificationRepo
	mailer       *MailerService
	setupLock    SetupLocker
	pool         *pgxpool.Pool
	email        *crypto.EmailCipher
	pepper       []byte
	// jwtKey signs/verifies bearer access tokens; accessTTL/refreshTTL govern
	// the bearer-token flow. Set via SetTokenAuth; zero values disable the
	// token endpoints (tests that don't exercise auth can skip it).
	jwtKey     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration

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

func NewAuthService(pool *pgxpool.Pool, users *repo.UserRepo, audit *repo.AuditRepo, verification *repo.VerificationRepo, mailer *MailerService, setupLock SetupLocker, email *crypto.EmailCipher, pepper []byte) *AuthService {
	return &AuthService{
		users:        users,
		audit:        audit,
		verification: verification,
		mailer:       mailer,
		setupLock:    setupLock,
		pool:         pool,
		email:        email,
		pepper:       pepper,
	}
}

// SetTokenAuth enables the bearer-token flow (SPA / Capacitor clients). It also
// threads the refresh repo into the token-revocation paths so that password
// change, account delete, and email-change confirm revoke refresh tokens.
// Wired in cmd/api; tests that don't exercise auth may leave it unset.
func (s *AuthService) SetTokenAuth(refresh *repo.RefreshTokenRepo, jwtKey []byte, accessTTL, refreshTTL time.Duration) {
	s.refresh = refresh
	s.jwtKey = jwtKey
	s.accessTTL = accessTTL
	s.refreshTTL = refreshTTL
}

// RevokeRefreshForUser revokes all refresh tokens for a user. Safe to call
// when token auth is disabled (no-op). Exposed so the Me / Admin services can
// end every other logged-in session on password change / account delete.
func (s *AuthService) RevokeRefreshForUser(ctx context.Context, userID uuid.UUID) error {
	if s.refresh == nil {
		return nil
	}
	return s.refresh.RevokeAllForUser(ctx, userID)
}

// User is a service-layer projection of a user with the decrypted email.
type User struct {
	ID              uuid.UUID
	Email           string
	DisplayName     string
	CreatedAt       time.Time
	HasAvatar       bool
	AvatarUpdatedAt *time.Time
	DeletedAt       *time.Time
	WeekStart       int16
	IsAdmin         bool
	EmailVerifiedAt *time.Time
}

func (s *AuthService) toUser(u *repo.User) (*User, error) {
	email, err := s.email.Decrypt(u.EmailEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt email: %w", err)
	}
	return &User{
		ID:              u.ID,
		Email:           email,
		DisplayName:     u.DisplayName,
		CreatedAt:       u.CreatedAt,
		HasAvatar:       u.AvatarUpdatedAt != nil,
		AvatarUpdatedAt: u.AvatarUpdatedAt,
		DeletedAt:       u.DeletedAt,
		WeekStart:       u.WeekStart,
		IsAdmin:         u.Role == "admin",
		EmailVerifiedAt: u.EmailVerifiedAt,
	}, nil
}

// RegisterResult is what /v1/auth/register returns to the handler. When the
// instance has SMTP configured the new account is unverified and
// VerificationRequired is true. When SMTP is unconfigured the account is
// auto-verified so the caller can immediately log in via /v1/auth/token (so
// the first bootstrap admin can register before SMTP exists).
type RegisterResult struct {
	User                 *User
	VerificationRequired bool
}

// Register creates a user via /v1/auth/register. While first-run setup is
// pending it returns ErrSetupRequired so the only path that can mint the
// very first user is /v1/setup/admin (which calls RegisterTx directly inside
// its own atomic ceremony).
func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*RegisterResult, error) {
	if s.setupLock != nil {
		locked, err := s.setupLock.Locked(ctx)
		if err != nil {
			return nil, err
		}
		if !locked {
			return nil, ErrSetupRequired
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	out, _, err := s.RegisterTx(ctx, tx, email, password, displayName)
	if err != nil {
		return nil, err
	}

	smtpReady := false
	if s.mailer != nil {
		ok, ierr := s.mailer.IsConfigured(ctx)
		if ierr == nil {
			smtpReady = ok
		}
	}

	if !smtpReady {
		// Auto-verify so the user can log in immediately. This happens on a
		// fresh install before SMTP is configured (and on every register
		// while SMTP stays unconfigured); recorded in audit so an admin can
		// see retroactively that the gate was open.
		if err := s.users.MarkEmailVerified(ctx, tx, out.ID); err != nil {
			return nil, err
		}
		meta, _ := json.Marshal(map[string]any{"reason": "smtp_unconfigured"})
		_ = s.audit.Insert(ctx, tx, &repo.AuditEntry{
			ActorUserID:  out.ID,
			TargetUserID: &out.ID,
			Action:       "auto_verified_no_smtp",
			Success:      true,
			Metadata:     meta,
		})
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		out.EmailVerifiedAt = ptrNow()
		return &RegisterResult{User: out, VerificationRequired: false}, nil
	}

	// SMTP is configured - issue a 6-digit code, enqueue the email. The user
	// must call /v1/auth/verify with the code, then log in via /v1/auth/token.
	code, err := generateNumericCode(6)
	if err != nil {
		return nil, err
	}
	tok := &repo.VerificationToken{
		UserID:    out.ID,
		Purpose:   repo.PurposeRegister,
		CodeHash:  hashCode(code),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	if err := s.verification.Insert(ctx, tx, tok); err != nil {
		return nil, err
	}
	if err := s.mailer.Enqueue(ctx, tx, email, "verify_register", TemplateVars{
		DisplayName: displayName,
		Code:        code,
		NewEmail:    email,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &RegisterResult{User: out, VerificationRequired: true}, nil
}

func ptrNow() *time.Time { t := time.Now(); return &t }

// RegisterTx is the bootstrap-aware user-creation core, callable inside a
// caller-owned transaction. /v1/setup/admin uses this so the install
// ceremony can commit the user creation atomically with the
// app_setup.completed_at update.
//
// Bootstrap rules: the first non-deleted user becomes role='admin' and
// gets a 'bootstrap_admin' audit row. Concurrent first registrations are
// serialized on pg_advisory_xact_lock('admin_bootstrap'), so only one
// caller observes count==0. Returns the service-level User projection AND
// the underlying repo.User row (the latter is what SetupService needs to
// stamp `completed_by`).
func (s *AuthService) RegisterTx(ctx context.Context, tx pgx.Tx, email, password, displayName string) (*User, *repo.User, error) {
	email = strings.TrimSpace(email)
	displayName = strings.TrimSpace(displayName)
	if email == "" || password == "" || displayName == "" {
		return nil, nil, errors.New("email, password, and display_name are required")
	}
	if len(password) < 10 {
		return nil, nil, errors.New("password must be at least 10 characters")
	}

	emailHash := s.email.HashEmail(email)
	if _, err := s.users.FindByEmailHash(ctx, emailHash); err == nil {
		return nil, nil, ErrEmailTaken
	} else if !errors.Is(err, repo.ErrNotFound) {
		return nil, nil, err
	}

	emailEnc, err := s.email.Encrypt(email)
	if err != nil {
		return nil, nil, err
	}
	pwdHash, err := crypto.HashPassword(password, s.pepper)
	if err != nil {
		return nil, nil, err
	}

	u := &repo.User{
		EmailHash:      emailHash,
		EmailEncrypted: emailEnc,
		DisplayName:    displayName,
		PasswordHash:   pwdHash,
	}

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('admin_bootstrap'))`); err != nil {
		return nil, nil, err
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE deleted_at IS NULL`).Scan(&n); err != nil {
		return nil, nil, err
	}
	role := "user"
	if n == 0 {
		role = "admin"
	}
	u.Role = role
	if err := s.users.CreateWithRole(ctx, tx, u, role); err != nil {
		return nil, nil, err
	}
	if role == "admin" {
		meta, _ := json.Marshal(map[string]any{"reason": "first_user"})
		if err := s.audit.Insert(ctx, tx, &repo.AuditEntry{
			ActorUserID: u.ID,
			Action:      "bootstrap_admin",
			Success:     true,
			Metadata:    meta,
		}); err != nil {
			return nil, nil, err
		}
	}

	out, err := s.toUser(u)
	if err != nil {
		return nil, nil, err
	}
	return out, u, nil
}

// GetUser reloads a user by ID and returns the service-level projection (with
// decrypted email). Returns ErrInvalidCredentials for a missing or soft-deleted
// user. Used by handlers that need the freshest user fields after an update.
func (s *AuthService) GetUser(ctx context.Context, userID uuid.UUID) (*User, error) {
	u, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if u.DeletedAt != nil {
		return nil, ErrInvalidCredentials
	}
	return s.toUser(u)
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// ErrTokenAuthDisabled is returned by the token methods when SetTokenAuth was
// never called (no signing key configured).
var ErrTokenAuthDisabled = errors.New("token auth disabled")

// ErrInvalidBearerToken covers expired/forged access tokens and unknown/expired/
// reused refresh tokens. Handlers map it to 401.
var ErrInvalidBearerToken = errors.New("invalid token")

// TokenPair is the result of issuing or refreshing bearer tokens.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
}

// IssueTokenPair authenticates with a password and returns a JWT access token
// plus a fresh refresh token. Applies the same credential checks (Argon2id,
// email-verified gate) for all token clients.
func (s *AuthService) IssueTokenPair(ctx context.Context, email, password string) (*User, *TokenPair, error) {
	if s.jwtKey == nil || s.refresh == nil {
		return nil, nil, ErrTokenAuthDisabled
	}
	emailHash := s.email.HashEmail(email)
	u, err := s.users.FindByEmailHash(ctx, emailHash)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, err
	}
	ok, err := crypto.VerifyPassword(u.PasswordHash, password, s.pepper)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, ErrInvalidCredentials
	}
	if u.EmailVerifiedAt == nil {
		return nil, nil, ErrEmailUnverified
	}
	pair, err := s.mintTokenPair(ctx, u.ID)
	if err != nil {
		return nil, nil, err
	}
	out, err := s.toUser(u)
	if err != nil {
		return nil, nil, err
	}
	return out, pair, nil
}

// RefreshTokenPair rotates a refresh token and mints a new access token.
// Presenting an already-revoked/rotated token (reuse) revokes the user's whole
// chain and returns ErrInvalidBearerToken.
func (s *AuthService) RefreshTokenPair(ctx context.Context, refreshToken string) (*TokenPair, error) {
	if s.jwtKey == nil || s.refresh == nil {
		return nil, ErrTokenAuthDisabled
	}
	if refreshToken == "" {
		return nil, ErrInvalidBearerToken
	}
	row, err := s.refresh.FindByTokenHash(ctx, hashToken(refreshToken))
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrInvalidBearerToken
	}
	if err != nil {
		return nil, err
	}
	// Reuse detection: a token that's already revoked or has a successor was
	// either rotated or explicitly killed. Treat re-presentation as theft and
	// nuke the whole chain.
	if row.RevokedAt != nil || row.ReplacedBy != nil {
		_ = s.refresh.RevokeAllForUser(ctx, row.UserID)
		return nil, ErrInvalidBearerToken
	}
	if time.Now().After(row.ExpiresAt) {
		return nil, ErrInvalidBearerToken
	}
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return nil, err
	}
	next := &repo.RefreshToken{
		UserID:    row.UserID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}
	if _, err := s.refresh.Rotate(ctx, row.ID, next); err != nil {
		return nil, err
	}
	access, err := s.signAccessToken(row.UserID)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: raw,
		AccessTTL:    s.accessTTL,
		RefreshTTL:   s.refreshTTL,
	}, nil
}

// RevokeRefreshToken revokes a single presented refresh token (token-client
// logout). Idempotent: unknown/empty tokens are a no-op.
func (s *AuthService) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	if s.refresh == nil || refreshToken == "" {
		return nil
	}
	return s.refresh.RevokeByTokenHash(ctx, hashToken(refreshToken))
}

// ResolveAccessToken validates a bearer JWT and returns the user. Used by the
// bearer middleware. Returns ErrInvalidBearerToken for any signature/expiry/claims
// failure or a deleted user.
func (s *AuthService) ResolveAccessToken(ctx context.Context, token string) (*User, error) {
	if s.jwtKey == nil {
		return nil, ErrTokenAuthDisabled
	}
	uid, err := s.parseAccessToken(token)
	if err != nil {
		return nil, ErrInvalidBearerToken
	}
	u, err := s.users.FindByID(ctx, uid)
	if err != nil {
		return nil, ErrInvalidBearerToken
	}
	if u.DeletedAt != nil {
		return nil, ErrInvalidBearerToken
	}
	return s.toUser(u)
}

// MintTokenPairForUser issues a fresh access + refresh token pair for a user
// without re-checking a password. Used after password change, where the caller
// has already revoked the user's other token chains and just needs to keep the
// current client logged in. Returns ErrTokenAuthDisabled when token auth is off.
func (s *AuthService) MintTokenPairForUser(ctx context.Context, userID uuid.UUID) (*TokenPair, error) {
	if s.jwtKey == nil || s.refresh == nil {
		return nil, ErrTokenAuthDisabled
	}
	return s.mintTokenPair(ctx, userID)
}

func (s *AuthService) mintTokenPair(ctx context.Context, userID uuid.UUID) (*TokenPair, error) {
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return nil, err
	}
	rt := &repo.RefreshToken{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}
	if err := s.refresh.Create(ctx, rt); err != nil {
		return nil, err
	}
	access, err := s.signAccessToken(userID)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: raw,
		AccessTTL:    s.accessTTL,
		RefreshTTL:   s.refreshTTL,
	}, nil
}

// newOpaqueToken returns a random 32-byte token (base64url) and its SHA-256.
func newOpaqueToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func (s *AuthService) signAccessToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.jwtKey)
}

func (s *AuthService) parseAccessToken(token string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtKey, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(claims.Subject)
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
