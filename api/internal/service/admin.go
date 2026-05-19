package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julian-alarcon/dothesplit/api/internal/crypto"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

var (
	ErrLastAdmin     = errors.New("cannot remove the last admin")
	ErrCannotTargetSelf = errors.New("admins cannot target their own account here")
)

// AdminService is the orchestration layer for admin endpoints. It wraps
// repos so destructive ops happen in a single transaction with the audit
// row, and centralises the last-admin guard.
type AdminService struct {
	pool     *pgxpool.Pool
	users    *repo.UserRepo
	groups   *repo.GroupRepo
	sessions *repo.SessionRepo
	audit    *repo.AuditRepo
	email    *crypto.EmailCipher
	pepper   []byte
}

func NewAdminService(pool *pgxpool.Pool, users *repo.UserRepo, groups *repo.GroupRepo, sessions *repo.SessionRepo, audit *repo.AuditRepo, email *crypto.EmailCipher, pepper []byte) *AdminService {
	return &AdminService{
		pool:     pool,
		users:    users,
		groups:   groups,
		sessions: sessions,
		audit:    audit,
		email:    email,
		pepper:   pepper,
	}
}

// AdminUserView decorates the service-layer User with the role + an
// admin-visible deleted_at marker.
type AdminUserView struct {
	ID                 uuid.UUID
	Email              string
	DisplayName        string
	Role               string
	CreatedAt          time.Time
	DeletedAt          *time.Time
	HasAvatar          bool
	WeekStart          int16
	MustChangePassword bool
}

func (s *AdminService) toAdminView(u *repo.User) (*AdminUserView, error) {
	// Soft-deleted users have email_encrypted scrambled with random bytes
	// (not valid AES-GCM), so decrypting would error. Surface a placeholder
	// so admins can still browse the deleted-user history; the
	// already-tombstoned display_name does the actual identification.
	var email string
	if u.DeletedAt != nil {
		email = ""
	} else {
		decrypted, err := s.email.Decrypt(u.EmailEncrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt email: %w", err)
		}
		email = decrypted
	}
	return &AdminUserView{
		ID:                 u.ID,
		Email:              email,
		DisplayName:        u.DisplayName,
		Role:               u.Role,
		CreatedAt:          u.CreatedAt,
		DeletedAt:          u.DeletedAt,
		HasAvatar:          u.AvatarUpdatedAt != nil,
		WeekStart:          u.WeekStart,
		MustChangePassword: u.MustChangePassword,
	}, nil
}

// GetUser returns the admin-view projection for a single user (or
// repo.ErrNotFound).
func (s *AdminService) GetUser(ctx context.Context, id uuid.UUID) (*AdminUserView, error) {
	u, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.toAdminView(u)
}

// SetUserRole promotes or demotes a target user. Admins cannot change their
// own role here (use a different admin), and the last active admin cannot be
// demoted.
func (s *AdminService) SetUserRole(ctx context.Context, actorID, targetID uuid.UUID, newRole, ip, ua string) (*AdminUserView, error) {
	switch newRole {
	case "user", "admin":
	default:
		return nil, errors.New("role must be 'user' or 'admin'")
	}
	if actorID == targetID {
		return nil, ErrCannotTargetSelf
	}
	target, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target.DeletedAt != nil {
		return nil, repo.ErrNotFound
	}
	if target.Role == newRole {
		// No-op; return the current view without writing an audit row.
		return s.toAdminView(target)
	}
	if target.Role == "admin" && newRole == "user" {
		// Demote: refuse to remove the last active admin.
		n, err := s.users.CountActiveAdmins(ctx)
		if err != nil {
			return nil, err
		}
		if n <= 1 {
			return nil, ErrLastAdmin
		}
	}
	if err := s.users.SetRole(ctx, targetID, newRole); err != nil {
		return nil, err
	}
	meta, _ := json.Marshal(map[string]any{
		"from": target.Role,
		"to":   newRole,
	})
	action := "admin_promote_user"
	if newRole == "user" {
		action = "admin_demote_user"
	}
	if err := s.audit.Insert(ctx, nil, &repo.AuditEntry{
		ActorUserID:  actorID,
		TargetUserID: &targetID,
		Action:       action,
		IP:           strPtr(ip),
		UserAgent:    strPtr(ua),
		Success:      true,
		Metadata:     meta,
	}); err != nil {
		return nil, err
	}
	target.Role = newRole
	return s.toAdminView(target)
}

func (s *AdminService) ListUsers(ctx context.Context, limit, offset int, includeDeleted bool) ([]AdminUserView, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, total, err := s.users.ListPaginated(ctx, limit, offset, includeDeleted)
	if err != nil {
		return nil, 0, err
	}
	out := make([]AdminUserView, 0, len(rows))
	for i := range rows {
		v, err := s.toAdminView(&rows[i])
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *v)
	}
	return out, total, nil
}

// CreateUser provisions a new account on behalf of an admin. The created
// user is forced to change their password on first login.
func (s *AdminService) CreateUser(ctx context.Context, actorID uuid.UUID, email, displayName, password, role string, ip, ua string) (*AdminUserView, error) {
	email = strings.TrimSpace(email)
	displayName = strings.TrimSpace(displayName)
	if email == "" || displayName == "" || password == "" {
		return nil, errors.New("email, display_name, password required")
	}
	if len(password) < 10 {
		return nil, errors.New("password must be at least 10 characters")
	}
	switch role {
	case "", "user":
		role = "user"
	case "admin":
		// allowed
	default:
		return nil, errors.New("role must be 'user' or 'admin'")
	}

	emailHash := s.email.HashEmail(email)
	if _, err := s.users.FindByEmailHash(ctx, emailHash); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	emailEnc, err := s.email.Encrypt(email)
	if err != nil {
		return nil, err
	}
	pwdHash, err := crypto.HashPassword(password, s.pepper)
	if err != nil {
		return nil, err
	}
	u := &repo.User{
		EmailHash:      emailHash,
		EmailEncrypted: emailEnc,
		DisplayName:    displayName,
		PasswordHash:   pwdHash,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := s.users.CreateWithRole(ctx, tx, u, role, true); err != nil {
		return nil, err
	}
	meta, _ := json.Marshal(map[string]any{"role": role})
	if err := s.audit.Insert(ctx, tx, &repo.AuditEntry{
		ActorUserID:  actorID,
		TargetUserID: &u.ID,
		Action:       "admin_create_user",
		IP:           strPtr(ip),
		UserAgent:    strPtr(ua),
		Success:      true,
		Metadata:     meta,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	u.Role = role
	u.MustChangePassword = true
	return s.toAdminView(u)
}

// DeleteUser soft-deletes the target user, scrubbing identifying fields and
// revoking every session. Refuses to delete the last admin and refuses
// self-targeting (the admin must keep another admin alive before stepping
// down).
func (s *AdminService) DeleteUser(ctx context.Context, actorID, targetID uuid.UUID, ip, ua string) error {
	if actorID == targetID {
		return ErrCannotTargetSelf
	}
	target, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target.DeletedAt != nil {
		return repo.ErrNotFound
	}
	if target.Role == "admin" {
		// Last-admin guard. If this is the only active admin, refuse.
		n, err := s.users.CountActiveAdmins(ctx)
		if err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastAdmin
		}
	}

	tombstone := "Deleted user #" + target.ID.String()[:8]
	scrambled, err := randomBytes(32)
	if err != nil {
		return err
	}
	if err := s.users.SoftDelete(ctx, targetID, tombstone, scrambled, scrambled, "!deleted:"+target.ID.String()); err != nil {
		return err
	}
	if err := s.sessions.DeleteAllForUser(ctx, targetID); err != nil {
		return err
	}
	return s.audit.Insert(ctx, nil, &repo.AuditEntry{
		ActorUserID:  actorID,
		TargetUserID: &targetID,
		Action:       "admin_delete_user",
		IP:           strPtr(ip),
		UserAgent:    strPtr(ua),
		Success:      true,
	})
}

// ResetUserPassword hashes the new password, sets must_change_password=true
// in the same UPDATE, and revokes the target's sessions.
func (s *AdminService) ResetUserPassword(ctx context.Context, actorID, targetID uuid.UUID, newPassword, ip, ua string) error {
	if len(newPassword) < 10 {
		return errors.New("new password must be at least 10 characters")
	}
	target, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target.DeletedAt != nil {
		return repo.ErrNotFound
	}
	hash, err := crypto.HashPassword(newPassword, s.pepper)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePasswordHashWithFlag(ctx, targetID, hash, true); err != nil {
		return err
	}
	if err := s.sessions.DeleteAllForUser(ctx, targetID); err != nil {
		return err
	}
	return s.audit.Insert(ctx, nil, &repo.AuditEntry{
		ActorUserID:  actorID,
		TargetUserID: &targetID,
		Action:       "admin_reset_password",
		IP:           strPtr(ip),
		UserAgent:    strPtr(ua),
		Success:      true,
	})
}

// AdminGroupListItem mirrors the API response shape for /v1/admin/groups.
type AdminGroupListItem struct {
	ID              uuid.UUID
	Name            string
	DefaultCurrency string
	CreatedBy       uuid.UUID
	CreatedAt       time.Time
	MemberCount     int
	ExpenseCount    int
}

func (s *AdminService) ListGroups(ctx context.Context, limit, offset int) ([]AdminGroupListItem, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, total, err := s.groups.ListAll(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]AdminGroupListItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, AdminGroupListItem{
			ID:              r.ID,
			Name:            r.Name,
			DefaultCurrency: r.DefaultCurrency,
			CreatedBy:       r.CreatedBy,
			CreatedAt:       r.CreatedAt,
			MemberCount:     r.MemberCount,
			ExpenseCount:    r.ExpenseCount,
		})
	}
	return out, total, nil
}

// DeleteGroup hard-deletes the group via the existing repo cascade. The
// migration on group_members/expenses/splits/settlements/recurring/etc.
// already fans the delete out for us.
func (s *AdminService) DeleteGroup(ctx context.Context, actorID, groupID uuid.UUID, ip, ua string) error {
	if err := s.groups.Delete(ctx, groupID); err != nil {
		return err
	}
	return s.audit.Insert(ctx, nil, &repo.AuditEntry{
		ActorUserID:   actorID,
		TargetGroupID: &groupID,
		Action:        "admin_delete_group",
		IP:            strPtr(ip),
		UserAgent:     strPtr(ua),
		Success:       true,
	})
}

// LogStepUpFailure is called by handlers when a step-up password verify
// fails, so failed attempts are recorded against the actor with the action
// they were trying to perform.
func (s *AdminService) LogStepUpFailure(ctx context.Context, actorID uuid.UUID, action string, target *uuid.UUID, group *uuid.UUID, ip, ua string) {
	meta, _ := json.Marshal(map[string]any{"attempted_action": action})
	_ = s.audit.Insert(ctx, nil, &repo.AuditEntry{
		ActorUserID:   actorID,
		TargetUserID:  target,
		TargetGroupID: group,
		Action:        "step_up_failed",
		IP:            strPtr(ip),
		UserAgent:     strPtr(ua),
		Success:       false,
		Metadata:      meta,
	})
}

// ListAudit returns paginated audit entries.
func (s *AdminService) ListAudit(ctx context.Context, action string, limit, offset int) ([]repo.AuditEntry, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.audit.List(ctx, repo.AuditFilter{Action: action}, limit, offset)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}
