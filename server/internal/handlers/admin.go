package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/server/internal/apigen"
	"github.com/julian-alarcon/dothesplit/server/internal/middleware"
	"github.com/julian-alarcon/dothesplit/server/internal/repo"
	"github.com/julian-alarcon/dothesplit/server/internal/service"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// stepUp verifies the actor's password and writes a failure audit row when it
// doesn't match. Returns true to continue, false on rejection (response is
// already written).
func (s *Server) stepUp(w http.ResponseWriter, r *http.Request, actorID uuid.UUID, password, action string, targetUser, targetGroup *uuid.UUID) bool {
	if password == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "password required for step-up")
		return false
	}
	err := s.Auth.VerifyPassword(r.Context(), actorID, password)
	if err == nil {
		return true
	}
	ip := s.clientIP(r)
	ua := r.UserAgent()
	if errors.Is(err, service.ErrStepUpRateLimited) {
		s.Admin.LogStepUpFailure(r.Context(), actorID, action, targetUser, targetGroup, ip, ua)
		writeErr(w, http.StatusLocked, "step_up_rate_limited", "too many failed step-up attempts; try again in a minute")
		return false
	}
	s.Admin.LogStepUpFailure(r.Context(), actorID, action, targetUser, targetGroup, ip, ua)
	writeErr(w, http.StatusUnauthorized, "step_up_failed", "step-up password did not match")
	return false
}

func parsePagination(r *http.Request) (int, int) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (s *Server) AdminListUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	includeDeleted := r.URL.Query().Get("include_deleted") == "true"
	rows, total, err := s.Admin.ListUsers(r.Context(), limit, offset, includeDeleted)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "list users failed")
		return
	}
	out := apigen.AdminUserListResponse{Limit: limit, Offset: offset, Total: total, Items: make([]apigen.AdminUser, 0, len(rows))}
	for _, row := range rows {
		out.Items = append(out.Items, toAPIAdminUser(row))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) AdminCreateUser(w http.ResponseWriter, r *http.Request) {
	actor := middleware.User(r.Context())
	var req apigen.AdminUserCreateRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	role := "user"
	if req.Role != nil {
		role = string(*req.Role)
	}
	out, err := s.Admin.CreateUser(r.Context(), actor.ID,
		string(req.Email), req.DisplayName, role, s.clientIP(r), r.UserAgent())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailTaken):
			writeErr(w, http.StatusConflict, "email_taken", "email already registered")
		case errors.Is(err, service.ErrSmtpUnconfigured):
			writeErr(w, http.StatusServiceUnavailable, "smtp_unconfigured",
				"configure SMTP before inviting users - they receive a welcome email to set their password")
		default:
			writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, toAPIAdminUser(*out))
}

func (s *Server) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	actor := middleware.User(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var req apigen.StepUpRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	if !s.stepUp(w, r, actor.ID, req.Password, "admin_delete_user", &id, nil) {
		return
	}
	if err := s.Admin.DeleteUser(r.Context(), actor.ID, id, s.clientIP(r), r.UserAgent()); err != nil {
		switch {
		case errors.Is(err, service.ErrLastAdmin):
			writeErr(w, http.StatusConflict, "last_admin", "cannot remove the last admin")
		case errors.Is(err, service.ErrCannotTargetSelf):
			writeErr(w, http.StatusConflict, "cannot_target_self", "admins cannot delete their own account here")
		case errors.Is(err, repo.ErrNotFound):
			writeErr(w, http.StatusNotFound, "not_found", "user not found")
		default:
			writeErr(w, http.StatusInternalServerError, "internal", "delete user failed")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminResetUserPassword(w http.ResponseWriter, r *http.Request) {
	actor := middleware.User(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var req apigen.StepUpRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	if !s.stepUp(w, r, actor.ID, req.Password, "admin_reset_password", &id, nil) {
		return
	}
	if err := s.Admin.ResetUserPassword(r.Context(), actor.ID, id, s.clientIP(r), r.UserAgent()); err != nil {
		switch {
		case errors.Is(err, repo.ErrNotFound):
			writeErr(w, http.StatusNotFound, "not_found", "user not found")
		case errors.Is(err, service.ErrSmtpUnconfigured):
			writeErr(w, http.StatusServiceUnavailable, "smtp_unconfigured",
				"configure SMTP before sending reset emails")
		default:
			writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	view, err := s.Admin.GetUser(r.Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", "get user failed")
		return
	}
	writeJSON(w, http.StatusOK, toAPIAdminUser(*view))
}

func (s *Server) AdminSetUserRole(w http.ResponseWriter, r *http.Request) {
	actor := middleware.User(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var req apigen.AdminSetUserRoleRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	action := "admin_set_user_role"
	if string(req.Role) == "admin" {
		action = "admin_promote_user"
	} else if string(req.Role) == "user" {
		action = "admin_demote_user"
	}
	if !s.stepUp(w, r, actor.ID, req.Password, action, &id, nil) {
		return
	}
	view, err := s.Admin.SetUserRole(r.Context(), actor.ID, id,
		string(req.Role), s.clientIP(r), r.UserAgent())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrLastAdmin):
			writeErr(w, http.StatusConflict, "last_admin", "cannot demote the last admin")
		case errors.Is(err, service.ErrCannotTargetSelf):
			writeErr(w, http.StatusConflict, "cannot_target_self", "admins cannot change their own role here")
		case errors.Is(err, repo.ErrNotFound):
			writeErr(w, http.StatusNotFound, "not_found", "user not found")
		default:
			writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, toAPIAdminUser(*view))
}

func (s *Server) AdminListGroups(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	rows, total, err := s.Admin.ListGroups(r.Context(), limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "list groups failed")
		return
	}
	out := apigen.AdminGroupListResponse{Limit: limit, Offset: offset, Total: total, Items: make([]apigen.AdminGroup, 0, len(rows))}
	for _, row := range rows {
		out.Items = append(out.Items, apigen.AdminGroup{
			Id:              row.ID,
			Name:            row.Name,
			DefaultCurrency: row.DefaultCurrency,
			CreatedBy:       row.CreatedBy,
			CreatedAt:       row.CreatedAt,
			MemberCount:     row.MemberCount,
			ExpenseCount:    row.ExpenseCount,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) AdminDeleteGroup(w http.ResponseWriter, r *http.Request) {
	actor := middleware.User(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var req apigen.StepUpRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	if !s.stepUp(w, r, actor.ID, req.Password, "admin_delete_group", nil, &id) {
		return
	}
	if err := s.Admin.DeleteGroup(r.Context(), actor.ID, id, s.clientIP(r), r.UserAgent()); err != nil {
		switch {
		case errors.Is(err, repo.ErrNotFound):
			writeErr(w, http.StatusNotFound, "not_found", "group not found")
		default:
			writeErr(w, http.StatusInternalServerError, "internal", "delete group failed")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminGetSmtp(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Smtp.Get(r.Context())
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "smtp not configured")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", "get smtp failed")
		return
	}
	writeJSON(w, http.StatusOK, toAPISmtp(cfg))
}

func (s *Server) AdminUpdateSmtp(w http.ResponseWriter, r *http.Request) {
	actor := middleware.User(r.Context())
	var req apigen.SmtpConfigUpdateRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	in := service.SmtpUpdateInput{
		Host:        req.Host,
		Port:        req.Port,
		Username:    req.Username,
		FromAddress: string(req.FromAddress),
		TLSMode:     string(req.TlsMode),
		Password:    req.SmtpPassword,
		UpdatedBy:   actor.ID,
	}
	if req.AllowPlaintextCredentials != nil {
		in.AllowPlaintextCredentials = *req.AllowPlaintextCredentials
	}
	cfg, err := s.Smtp.Update(r.Context(), in)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSmtpInvalid), errors.Is(err, service.ErrSmtpPlaintextDisabled):
			writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			writeErr(w, http.StatusInternalServerError, "internal", "update smtp failed")
		}
		return
	}
	_ = s.Audit.Insert(r.Context(), nil, &repo.AuditEntry{
		ActorUserID: actor.ID,
		Action:      "admin_update_smtp",
		IP:          strPtrOrNil(s.clientIP(r)),
		UserAgent:   strPtrOrNil(r.UserAgent()),
		Success:     true,
	})
	writeJSON(w, http.StatusOK, toAPISmtp(cfg))
}

func (s *Server) AdminTestSmtp(w http.ResponseWriter, r *http.Request) {
	res, err := s.Smtp.Test(r.Context())
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "smtp not configured")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", "test smtp failed")
		return
	}
	out := apigen.SmtpTestResponse{Success: res.Success}
	if res.Error != "" {
		e := res.Error
		out.Error = &e
	}
	writeJSON(w, http.StatusOK, out)
}

// AdminRevealSmtpPassword returns the stored SMTP password as cleartext.
// Admin-only and explicitly audit-logged: revealing a credential is the kind
// of action ops should be able to discover later in the audit feed, both for
// incident response and for regular review.
func (s *Server) AdminRevealSmtpPassword(w http.ResponseWriter, r *http.Request) {
	actor := middleware.User(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	pw, err := s.Smtp.RevealPassword(r.Context())
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "smtp not configured")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", "reveal smtp password failed")
		return
	}
	if pw == "" {
		writeErr(w, http.StatusNotFound, "not_found", "no password stored")
		return
	}
	_ = s.Audit.Insert(r.Context(), nil, &repo.AuditEntry{
		ActorUserID: actor.ID,
		Action:      "admin_view_smtp_password",
		IP:          strPtrOrNil(s.clientIP(r)),
		UserAgent:   strPtrOrNil(r.UserAgent()),
		Success:     true,
	})
	writeJSON(w, http.StatusOK, apigen.SmtpPasswordResponse{Password: pw})
}

// AdminSendSmtpTestEmail dispatches a real plain-text test email to the
// admin's own address, synchronously. Bypasses the outbox so SMTP errors
// surface immediately in the UI instead of disappearing into worker retries.
func (s *Server) AdminSendSmtpTestEmail(w http.ResponseWriter, r *http.Request) {
	actor := middleware.User(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	ok, err := s.Mailer.IsConfigured(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "load smtp config failed")
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "not_found", "smtp not configured")
		return
	}
	out := apigen.SmtpTestResponse{Success: true}
	if err := s.Mailer.SendNow(r.Context(), actor.Email, "smtp_test", service.TemplateVars{
		DisplayName: actor.DisplayName,
		WebOrigin:   s.Cfg.WebOrigin,
	}); err != nil {
		out.Success = false
		msg := err.Error()
		out.Error = &msg
	}
	_ = s.Audit.Insert(r.Context(), nil, &repo.AuditEntry{
		ActorUserID: actor.ID,
		Action:      "admin_send_smtp_test",
		IP:          strPtrOrNil(s.clientIP(r)),
		UserAgent:   strPtrOrNil(r.UserAgent()),
		Success:     out.Success,
	})
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) AdminListAudit(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	action := r.URL.Query().Get("action")
	rows, total, err := s.Admin.ListAudit(r.Context(), action, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "list audit failed")
		return
	}
	out := apigen.AdminAuditListResponse{Limit: limit, Offset: offset, Total: total, Items: make([]apigen.AdminAuditEntry, 0, len(rows))}
	for _, row := range rows {
		out.Items = append(out.Items, toAPIAuditEntry(row))
	}
	writeJSON(w, http.StatusOK, out)
}

func toAPIAdminUser(u service.AdminUserView) apigen.AdminUser {
	out := apigen.AdminUser{
		Id:          u.ID,
		DisplayName: u.DisplayName,
		Role:        apigen.AdminUserRole(u.Role),
		CreatedAt:   u.CreatedAt,
		DeletedAt:   u.DeletedAt,
		HasAvatar:   u.HasAvatar,
		WeekStart:   apigen.AdminUserWeekStart(u.WeekStart),
	}
	// Email is nullable on the wire because soft-deleted users have a
	// scrambled email_encrypted that can't be decrypted.
	if u.Email != "" {
		e := openapi_types.Email(u.Email)
		out.Email = &e
	}
	return out
}

func toAPISmtp(c *service.SmtpConfig) apigen.SmtpConfig {
	out := apigen.SmtpConfig{
		Host:        c.Host,
		Port:        c.Port,
		Username:    c.Username,
		FromAddress: openapi_types.Email(c.FromAddress),
		TlsMode:     apigen.SmtpTlsMode(c.TLSMode),
		PasswordSet: c.PasswordSet,
	}
	if !c.UpdatedAt.IsZero() {
		t := c.UpdatedAt
		out.UpdatedAt = &t
	}
	out.UpdatedBy = c.UpdatedBy
	return out
}

func toAPIAuditEntry(e repo.AuditEntry) apigen.AdminAuditEntry {
	out := apigen.AdminAuditEntry{
		Id:            e.ID,
		ActorUserId:   e.ActorUserID,
		TargetUserId:  e.TargetUserID,
		TargetGroupId: e.TargetGroupID,
		Action:        e.Action,
		Ip:            e.IP,
		UserAgent:     e.UserAgent,
		Success:       e.Success,
		CreatedAt:     e.CreatedAt,
	}
	if len(e.Metadata) > 0 {
		var m map[string]interface{}
		_ = json.Unmarshal(e.Metadata, &m)
		if m != nil {
			out.Metadata = &m
		}
	}
	return out
}

// strPtrOrNil mirrors service.strPtr so handlers don't import unexported helpers.
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
