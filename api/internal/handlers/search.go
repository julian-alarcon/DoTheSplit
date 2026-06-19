package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

func (s *Server) Search(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	query := r.URL.Query()
	q := query.Get("q")

	limit := 0
	if raw := query.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "bad_request", "limit must be an integer")
			return
		}
		limit = n
	}

	var groupIDs []uuid.UUID
	for _, raw := range query["group_id"] {
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "bad_request", "group_id must be a UUID")
			return
		}
		groupIDs = append(groupIDs, id)
	}

	var categoryID *uuid.UUID
	if raw := query.Get("category_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "bad_request", "category_id must be a UUID")
			return
		}
		categoryID = &id
	}

	res, err := s.SearchSvc.Search(r.Context(), u.ID, q, groupIDs, categoryID, limit)
	switch {
	case errors.Is(err, service.ErrBadSearchQuery):
		writeErr(w, http.StatusBadRequest, "bad_request", "q must be at least 2 characters")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPISearchResponse(res))
}

func toAPISearchResponse(r *service.SearchResult) apigen.SearchResponse {
	items := make([]apigen.TransactionItem, 0, len(r.Items))
	for _, item := range r.Items {
		ai := apigen.TransactionItem{
			Kind:       apigen.TransactionItemKind(item.Kind),
			OccurredAt: item.OccurredAt,
		}
		switch item.Kind {
		case repo.TransactionExpense:
			if item.Expense != nil {
				e := toAPIExpense(item.Expense)
				ai.Expense = &e
			}
		case repo.TransactionSettlement:
			if item.Settlement != nil {
				st := toAPISettlement(item.Settlement)
				ai.Settlement = &st
			}
		}
		items = append(items, ai)
	}
	groups := make([]apigen.SearchGroupRef, 0, len(r.Groups))
	for _, gi := range r.Groups {
		ms := make([]apigen.GroupMember, 0, len(gi.Members))
		for i := range gi.Members {
			ms = append(ms, toAPIMember(&gi.Members[i]))
		}
		groups = append(groups, apigen.SearchGroupRef{
			Id:              gi.Group.ID,
			Name:            gi.Group.Name,
			DefaultCurrency: gi.Group.DefaultCurrency,
			Members:         ms,
		})
	}
	availCats := make([]uuid.UUID, 0, len(r.AvailableCategoryIDs))
	availCats = append(availCats, r.AvailableCategoryIDs...)
	return apigen.SearchResponse{
		Query:                r.Query,
		Items:                items,
		Groups:               groups,
		AvailableCategoryIds: availCats,
	}
}
