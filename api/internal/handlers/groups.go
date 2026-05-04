package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// parseUUID reads a path param and writes 400 on invalid UUIDs.
func parseUUID(c *gin.Context, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(key))
	if err != nil {
		writeErr(c, http.StatusBadRequest, "bad_request", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func (s *Server) ListGroups(c *gin.Context) {
	u := middleware.User(c)
	groups, membersByGroup, err := s.Groups.List(c.Request.Context(), u.ID)
	if err != nil {
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]apigen.Group, 0, len(groups))
	for _, g := range groups {
		out = append(out, toAPIGroup(&g, membersByGroup[g.ID]))
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) CreateGroup(c *gin.Context) {
	u := middleware.User(c)
	var req apigen.CreateGroupRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	currency := ""
	if req.DefaultCurrency != nil {
		currency = *req.DefaultCurrency
	}
	g, members, err := s.Groups.Create(c.Request.Context(), req.Name, currency, u.ID)
	if err != nil {
		writeErr(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	c.JSON(http.StatusCreated, toAPIGroup(g, members))
}

func (s *Server) UpdateGroup(c *gin.Context) {
	u := middleware.User(c)
	groupID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var req apigen.UpdateGroupRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	g, members, err := s.Groups.Update(c.Request.Context(), groupID, u.ID, req.Name, req.DefaultCurrency)
	switch {
	case errors.Is(err, service.ErrNotMember):
		writeErr(c, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, repo.ErrNotFound):
		writeErr(c, http.StatusNotFound, "not_found", "group not found")
		return
	case errors.Is(err, service.ErrBadCurrency):
		writeErr(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	case err != nil:
		writeErr(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	c.JSON(http.StatusOK, toAPIGroup(g, members))
}

func (s *Server) DeleteGroup(c *gin.Context) {
	u := middleware.User(c)
	groupID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	err := s.Groups.Delete(c.Request.Context(), groupID, u.ID)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(c, http.StatusNotFound, "not_found", "group not found")
		return
	case errors.Is(err, service.ErrNotCreator):
		writeErr(c, http.StatusForbidden, "forbidden", "only the group creator can delete the group")
		return
	case err != nil:
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) AddGroupMember(c *gin.Context) {
	u := middleware.User(c)
	groupID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var req apigen.AddMemberRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	m, err := s.Groups.AddMember(c.Request.Context(), groupID, u.ID, string(req.Email))
	switch {
	case errors.Is(err, service.ErrNotMember):
		writeErr(c, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, service.ErrInviteeNotFound):
		writeErr(c, http.StatusNotFound, "not_found", "invitee is not registered")
		return
	case err != nil:
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	c.JSON(http.StatusCreated, toAPIMember(m))
}

func toAPIGroup(g *repo.Group, members []repo.GroupMember) apigen.Group {
	ms := make([]apigen.GroupMember, 0, len(members))
	for i := range members {
		ms = append(ms, toAPIMember(&members[i]))
	}
	return apigen.Group{
		Id:              g.ID,
		Name:            g.Name,
		DefaultCurrency: g.DefaultCurrency,
		CreatedBy:       g.CreatedBy,
		CreatedAt:       g.CreatedAt,
		Members:         ms,
	}
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
