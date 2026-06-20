package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

func (s *Server) ListTransactions(w http.ResponseWriter, r *http.Request) {
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
	page, err := s.Transactions.List(r.Context(), u.ID, groupID, limit, cursor)
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
	writeJSON(w, http.StatusOK, toAPITransactionPage(page))
}

func toAPITransactionPage(p *service.TransactionPage) apigen.TransactionPage {
	out := apigen.TransactionPage{
		Items: make([]apigen.TransactionItem, 0, len(p.Items)),
	}
	for _, item := range p.Items {
		ai := apigen.TransactionItem{
			Kind:       apigen.TransactionItemKind(item.Kind),
			OccurredAt: item.OccurredAt,
		}
		if item.Cadence != "" {
			cad := apigen.Cadence(item.Cadence)
			ai.Cadence = &cad
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
		out.Items = append(out.Items, ai)
	}
	if p.NextCursor != "" {
		nc := p.NextCursor
		out.NextCursor = &nc
	}
	return out
}
