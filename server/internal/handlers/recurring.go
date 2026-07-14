package handlers

import (
	"errors"
	"net/http"

	"github.com/julian-alarcon/dothesplit/server/internal/apigen"
	"github.com/julian-alarcon/dothesplit/server/internal/middleware"
	"github.com/julian-alarcon/dothesplit/server/internal/repo"
	"github.com/julian-alarcon/dothesplit/server/internal/service"
)

func (s *Server) ListRecurringExpenses(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	list, err := s.Recurring.List(r.Context(), u.ID, groupID)
	if errors.Is(err, service.ErrNotMember) {
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]apigen.RecurringExpense, 0, len(list))
	for i := range list {
		out = append(out, toAPIRecurring(&list[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) CreateRecurringExpense(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var req apigen.CreateRecurringExpenseRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	currency := ""
	if req.Currency != nil {
		currency = *req.Currency
	}
	splits := make([]service.SplitInput, len(req.Splits))
	for i, sp := range req.Splits {
		v := int64(0)
		if sp.Value != nil {
			v = *sp.Value
		}
		splits[i] = service.SplitInput{UserID: sp.UserId, Value: v}
	}
	out, err := s.Recurring.Create(r.Context(), u.ID, service.CreateRecurringInput{
		GroupID:     groupID,
		PayerID:     req.PayerId,
		CategoryID:  req.CategoryId,
		AmountCents: req.AmountCents,
		Currency:    currency,
		Description: req.Description,
		Mode:        service.SplitMode(req.Mode),
		Splits:      splits,
		Cadence:     string(req.Cadence),
		NextRunAt:   req.NextRunAt,
	})
	switch {
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, service.ErrUnknownCategory):
		writeErr(w, http.StatusBadRequest, "bad_request", "unknown category_id")
		return
	case err != nil:
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toAPIRecurring(out))
}

func (s *Server) DeleteRecurringExpense(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	err := s.Recurring.Delete(r.Context(), u.ID, id)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "not found")
		return
	case errors.Is(err, service.ErrForbidden):
		writeErr(w, http.StatusForbidden, "forbidden", "not permitted")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toAPIRecurring(e *repo.RecurringExpense) apigen.RecurringExpense {
	splits := make([]apigen.SplitInput, 0, len(e.SplitTemplate))
	for _, t := range e.SplitTemplate {
		v := t.Value
		splits = append(splits, apigen.SplitInput{UserId: t.UserID, Value: &v})
	}
	return apigen.RecurringExpense{
		Id:          e.ID,
		GroupId:     e.GroupID,
		PayerId:     e.PayerID,
		CategoryId:  e.CategoryID,
		AmountCents: e.AmountCents,
		Currency:    e.Currency,
		Description: e.Description,
		Mode:        apigen.SplitMode(e.Mode),
		Splits:      splits,
		Cadence:     apigen.Cadence(e.Cadence),
		NextRunAt:   e.NextRunAt,
		CreatedAt:   e.CreatedAt,
	}
}
