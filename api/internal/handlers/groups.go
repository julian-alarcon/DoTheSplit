package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// parseUUID reads a path param and writes 400 on invalid UUIDs.
func parseUUID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(key))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func (s *Server) ListGroups(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groups, membersByGroup, err := s.Groups.List(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]apigen.Group, 0, len(groups))
	for _, g := range groups {
		out = append(out, toAPIGroup(&g, membersByGroup[g.ID]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) CreateGroup(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	var req apigen.CreateGroupRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	currency := ""
	if req.DefaultCurrency != nil {
		currency = *req.DefaultCurrency
	}
	g, members, err := s.Groups.Create(r.Context(), req.Name, currency, u.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toAPIGroup(g, members))
}

func (s *Server) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var req apigen.UpdateGroupRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	in := service.UpdateGroupInput{
		Name:            req.Name,
		DefaultCurrency: req.DefaultCurrency,
		CreatedBy:       req.CreatedBy,
	}
	if req.DefaultSplit != nil {
		entries := make([]repo.DefaultSplitEntry, len(*req.DefaultSplit))
		for i, e := range *req.DefaultSplit {
			entries[i] = repo.DefaultSplitEntry{UserID: e.UserId, BasisPoints: int64(e.BasisPoints)}
		}
		in.DefaultSplit = &entries
	}
	g, members, err := s.Groups.Update(r.Context(), groupID, u.ID, in)
	switch {
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "group not found")
		return
	case errors.Is(err, service.ErrNotCreator):
		writeErr(w, http.StatusForbidden, "forbidden", "only the group creator can transfer ownership")
		return
	case errors.Is(err, service.ErrNewOwnerNotMember):
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	case errors.Is(err, service.ErrBadCurrency), errors.Is(err, service.ErrBadDefaultSplit):
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	case errors.Is(err, service.ErrCurrencyLocked):
		writeErr(w, http.StatusConflict, "currency_locked", err.Error())
		return
	case err != nil:
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPIGroup(g, members))
}

func (s *Server) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	err := s.Groups.Delete(r.Context(), groupID, u.ID)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "group not found")
		return
	case errors.Is(err, service.ErrNotCreator):
		writeErr(w, http.StatusForbidden, "forbidden", "only the group creator can delete the group")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AddGroupMember(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var req apigen.AddMemberRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	m, err := s.Groups.AddMember(r.Context(), groupID, u.ID, string(req.Email))
	switch {
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, service.ErrInviteeNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "invitee is not registered")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toAPIMember(m))
}

// RemoveGroupMember removes a member from a group. The creator can remove
// any non-creator member; any member can remove themselves (leave). Requires
// the target's net balance to be zero.
func (s *Server) RemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	targetID, ok := parseUUID(w, r, "userId")
	if !ok {
		return
	}
	err := s.Groups.RemoveMember(r.Context(), groupID, u.ID, targetID)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "member not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, service.ErrNotCreator):
		writeErr(w, http.StatusForbidden, "forbidden", "only the group creator can remove other members")
		return
	case errors.Is(err, service.ErrCannotRemoveCreator):
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	case errors.Is(err, service.ErrBalanceNotZero):
		writeErr(w, http.StatusBadRequest, "bad_request", "This member still has an outstanding balance. Settle up so their balance is zero, then remove them. Otherwise their share of the group's expenses would be dropped from the ledger.")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toAPIGroup(g *repo.Group, members []repo.GroupMember) apigen.Group {
	ms := make([]apigen.GroupMember, 0, len(members))
	for i := range members {
		ms = append(ms, toAPIMember(&members[i]))
	}
	out := apigen.Group{
		Id:              g.ID,
		Name:            g.Name,
		DefaultCurrency: g.DefaultCurrency,
		CreatedBy:       g.CreatedBy,
		CreatedAt:       g.CreatedAt,
		Members:         ms,
	}
	if len(g.DefaultSplit) > 0 {
		entries := make([]apigen.DefaultSplitEntry, len(g.DefaultSplit))
		for i, e := range g.DefaultSplit {
			entries[i] = apigen.DefaultSplitEntry{UserId: e.UserID, BasisPoints: int(e.BasisPoints)}
		}
		out.DefaultSplit = &entries
	}
	return out
}

// ExportGroupCSV streams the full group ledger as a Splitwise-compatible
// CSV with dothesplit-only metadata columns. Any group member can call
// it (membership enforced by the service).
func (s *Server) ExportGroupCSV(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var buf bytes.Buffer
	res, err := s.Exporter.Export(r.Context(), &buf, u.ID, groupID)
	switch {
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "group not found")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	filename := fmt.Sprintf("%s_%s_export.csv", slugifyForFilename(res.GroupName), res.GeneratedAt.Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// slugifyForFilename produces a filesystem-safe slug. Lowercase ASCII
// alphanumerics and dashes only; runs of other characters collapse to
// a single dash; capped at 40 chars; falls back to "group" when the
// input has no usable characters.
func slugifyForFilename(name string) string {
	var b strings.Builder
	prevDash := true
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// Drop non-ASCII letters/digits to keep filenames portable.
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
		if b.Len() >= 40 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "group"
	}
	return out
}

func toAPIMember(m *repo.GroupMember) apigen.GroupMember {
	return apigen.GroupMember{
		UserId:          m.UserID,
		DisplayName:     m.DisplayName,
		JoinedAt:        m.JoinedAt,
		HasAvatar:       m.AvatarUpdatedAt != nil,
		AvatarUpdatedAt: m.AvatarUpdatedAt,
		DeletedAt:       m.DeletedAt,
	}
}
