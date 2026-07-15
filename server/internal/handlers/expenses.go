package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/julian-alarcon/dothesplit/server/internal/apigen"
	"github.com/julian-alarcon/dothesplit/server/internal/middleware"
	"github.com/julian-alarcon/dothesplit/server/internal/repo"
	"github.com/julian-alarcon/dothesplit/server/internal/service"
)

func (s *Server) ListExpenses(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	exps, err := s.Expenses.List(r.Context(), u.ID, groupID)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) {
			writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]apigen.Expense, 0, len(exps))
	for i := range exps {
		out = append(out, toAPIExpense(&exps[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) CreateExpense(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var req apigen.CreateExpenseRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	// Leave zero when omitted; the service anchors the default to noon UTC.
	var incurredAt time.Time
	if req.IncurredAt != nil {
		incurredAt = *req.IncurredAt
	}
	currency := ""
	if req.Currency != nil {
		currency = *req.Currency
	}
	notes := ""
	if req.Notes != nil {
		notes = *req.Notes
	}

	splits := make([]service.SplitInput, len(req.Splits))
	for i, sp := range req.Splits {
		v := int64(0)
		if sp.Value != nil {
			v = *sp.Value
		}
		splits[i] = service.SplitInput{UserID: sp.UserId, Value: v}
	}

	out, err := s.Expenses.Create(r.Context(), u.ID, service.CreateExpenseInput{
		GroupID:     groupID,
		PayerID:     req.PayerId,
		CategoryID:  req.CategoryId,
		AmountCents: req.AmountCents,
		Currency:    currency,
		Description: req.Description,
		Notes:       notes,
		IncurredAt:  incurredAt,
		Mode:        service.SplitMode(req.Mode),
		Splits:      splits,
	})
	switch {
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, service.ErrUnknownCategory):
		writeErr(w, http.StatusBadRequest, "bad_request", "unknown category_id")
		return
	case errors.Is(err, service.ErrPayerNotMember), errors.Is(err, service.ErrSplitNotMember),
		errors.Is(err, service.ErrBadSplit):
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toAPIExpense(out))
}

// GetExpense returns one expense (member-only).
func (s *Server) GetExpense(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	e, err := s.Expenses.Get(r.Context(), u.ID, id)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "expense not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPIExpense(e))
}

// UpdateExpense edits description / amount / category / payer / splits.
// Any group member may edit; the change is recorded in the revision history.
func (s *Server) UpdateExpense(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var req apigen.UpdateExpenseRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	in := service.UpdateExpenseInput{
		Description: req.Description,
		AmountCents: req.AmountCents,
		CategoryID:  req.CategoryId,
		PayerID:     req.PayerId,
		Notes:       req.Notes,
		IncurredAt:  req.IncurredAt,
	}
	if req.Mode != nil {
		m := service.SplitMode(*req.Mode)
		in.Mode = &m
	}
	if req.Splits != nil {
		splits := make([]service.SplitInput, len(*req.Splits))
		for i, sp := range *req.Splits {
			v := int64(0)
			if sp.Value != nil {
				v = *sp.Value
			}
			splits[i] = service.SplitInput{UserID: sp.UserId, Value: v}
		}
		in.Splits = splits
	}

	out, err := s.Expenses.Update(r.Context(), u.ID, id, in)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "expense not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, service.ErrUnknownCategory):
		writeErr(w, http.StatusBadRequest, "bad_request", "unknown category_id")
		return
	case errors.Is(err, service.ErrPayerNotMember):
		writeErr(w, http.StatusBadRequest, "bad_request", "payer is not a group member")
		return
	case errors.Is(err, service.ErrSplitNotMember):
		writeErr(w, http.StatusBadRequest, "bad_request", "split user is not a group member")
		return
	case errors.Is(err, service.ErrBadSplit):
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPIExpense(out))
}

// ListExpenseRevisions returns the edit history for an expense.
func (s *Server) ListExpenseRevisions(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	revs, err := s.Expenses.ListRevisions(r.Context(), u.ID, id)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "expense not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]apigen.ExpenseRevision, 0, len(revs))
	for i := range revs {
		rev := revs[i]
		out = append(out, apigen.ExpenseRevision{
			Id:        rev.ID,
			ExpenseId: rev.ExpenseID,
			EditedBy:  rev.EditedBy,
			EditedAt:  rev.EditedAt,
			Field:     apigen.ExpenseRevisionField(rev.Field),
			OldValue:  rev.OldValue,
			NewValue:  rev.NewValue,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	expenseID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	err := s.Expenses.Delete(r.Context(), u.ID, expenseID)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "expense not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RestoreExpense un-deletes a soft-deleted expense (any group member).
func (s *Server) RestoreExpense(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	expenseID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	out, err := s.Expenses.Restore(r.Context(), u.ID, expenseID)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "expense not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, service.ErrAlreadyActive):
		writeErr(w, http.StatusConflict, "conflict", "expense is not deleted")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPIExpense(out))
}

func toAPIExpense(e *repo.Expense) apigen.Expense {
	splits := make([]apigen.Split, 0, len(e.Splits))
	for _, sp := range e.Splits {
		splits = append(splits, apigen.Split{UserId: sp.UserID, ShareCents: sp.ShareCents})
	}
	return apigen.Expense{
		Id:          e.ID,
		GroupId:     e.GroupID,
		PayerId:     e.PayerID,
		CreatedBy:   e.CreatedBy,
		CategoryId:  e.CategoryID,
		AmountCents: e.AmountCents,
		Currency:    e.Currency,
		Description: e.Description,
		Notes:       e.Notes,
		IncurredAt:  e.IncurredAt,
		CreatedAt:   e.CreatedAt,
		DeletedAt:   e.DeletedAt,
		Splits:      splits,
	}
}
