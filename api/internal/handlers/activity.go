package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

func (s *Server) ListActivity(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "bad_request", "limit must be an integer")
			return
		}
		limit = n
	}
	cursor := r.URL.Query().Get("cursor")
	page, err := s.Activity.List(r.Context(), u.ID, groupID, limit, cursor)
	switch {
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, service.ErrBadCursor):
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid cursor")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPIActivityPage(page))
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
