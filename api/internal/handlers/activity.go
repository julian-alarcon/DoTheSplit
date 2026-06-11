package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

func (s *Server) ListActivity(c *gin.Context) {
	u := middleware.User(c)
	groupID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeErr(c, http.StatusBadRequest, "bad_request", "limit must be an integer")
			return
		}
		limit = n
	}
	cursor := c.Query("cursor")
	page, err := s.Activity.List(c.Request.Context(), u.ID, groupID, limit, cursor)
	switch {
	case errors.Is(err, service.ErrNotMember):
		writeErr(c, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, service.ErrBadCursor):
		writeErr(c, http.StatusBadRequest, "bad_request", "invalid cursor")
		return
	case err != nil:
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	c.JSON(http.StatusOK, toAPIActivityPage(page))
}

func toAPIActivityPage(p *service.ActivityPage) apigen.ActivityPage {
	out := apigen.ActivityPage{
		Items: make([]apigen.ActivityItem, 0, len(p.Items)),
	}
	for _, h := range p.Items {
		kind := apigen.ActivityItemTargetKindExpense
		if strings.HasPrefix(string(h.Action), "settlement.") {
			kind = apigen.ActivityItemTargetKindSettlement
		}
		ai := apigen.ActivityItem{
			Id:          h.ID,
			Action:      apigen.ActivityItemAction(h.Action),
			OccurredAt:  h.OccurredAt,
			TargetId:    h.TargetID,
			TargetKind:  kind,
			Description: h.Description,
			AmountCents: h.AmountCents,
			Currency:    h.Currency,
			Recurring:   h.Recurring,
			FromUserId:  h.FromUserID,
			ToUserId:    h.ToUserID,
		}
		if h.ActorID != nil && h.ActorName != nil {
			ai.Actor = &apigen.ActivityActor{
				UserId:          *h.ActorID,
				DisplayName:     *h.ActorName,
				HasAvatar:       h.ActorAvatarUpdatedAt != nil,
				AvatarUpdatedAt: h.ActorAvatarUpdatedAt,
			}
		}
		if h.CategorySlug != nil {
			ai.CategorySlug = h.CategorySlug
		}
		if h.CategoryGroupLabel != nil {
			ai.CategoryGroupLabel = h.CategoryGroupLabel
		}
		out.Items = append(out.Items, ai)
	}
	if p.NextCursor != "" {
		nc := p.NextCursor
		out.NextCursor = &nc
	}
	return out
}
