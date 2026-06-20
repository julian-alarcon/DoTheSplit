package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

func (s *Server) GetBalances(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	res, err := s.Balances.Get(r.Context(), u.ID, groupID)
	if errors.Is(err, service.ErrNotMember) {
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	net := make([]apigen.Balance, 0, len(res.Net))
	for _, b := range res.Net {
		net = append(net, apigen.Balance{
			UserId:      b.UserID,
			DisplayName: b.DisplayName,
			NetCents:    b.NetCents,
		})
	}
	simp := make([]apigen.SimplifiedDebt, 0, len(res.Simplified))
	for _, d := range res.Simplified {
		simp = append(simp, apigen.SimplifiedDebt{
			FromUserId:  d.FromUserID,
			ToUserId:    d.ToUserID,
			AmountCents: d.AmountCents,
		})
	}
	writeJSON(w, http.StatusOK, apigen.BalancesResponse{Net: net, Simplified: simp})
}

func (s *Server) ListSettlements(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	settlements, err := s.Settlements.List(r.Context(), u.ID, groupID)
	if errors.Is(err, service.ErrNotMember) {
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]apigen.Settlement, 0, len(settlements))
	for i := range settlements {
		out = append(out, toAPISettlement(&settlements[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) CreateSettlement(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var req apigen.CreateSettlementRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	note := ""
	if req.Note != nil {
		note = *req.Note
	}
	// Leave zero when omitted; the service anchors the default to noon UTC.
	var settledAt time.Time
	if req.SettledAt != nil {
		settledAt = *req.SettledAt
	}
	fromUserID := u.ID
	if req.FromUserId != nil {
		fromUserID = *req.FromUserId
	}
	out, err := s.Settlements.Create(r.Context(), u.ID, service.CreateSettlementInput{
		GroupID:     groupID,
		FromUserID:  fromUserID,
		ToUserID:    req.ToUserId,
		AmountCents: req.AmountCents,
		Note:        note,
		SettledAt:   settledAt,
	})
	switch {
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case err != nil:
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toAPISettlement(out))
}

func (s *Server) GetSettlement(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	st, err := s.Settlements.Get(r.Context(), u.ID, id)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "settlement not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPISettlement(st))
}

func (s *Server) UpdateSettlement(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var req apigen.UpdateSettlementRequest
	if !bindStrictJSON(w, r, &req) {
		return
	}
	in := service.UpdateSettlementInput{
		FromUserID:  req.FromUserId,
		ToUserID:    req.ToUserId,
		AmountCents: req.AmountCents,
		Note:        req.Note,
		SettledAt:   req.SettledAt,
	}
	out, err := s.Settlements.Update(r.Context(), u.ID, id, in)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "settlement not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case err != nil:
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPISettlement(out))
}

func (s *Server) DeleteSettlement(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	settlementID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	err := s.Settlements.Delete(r.Context(), u.ID, settlementID)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "settlement not found")
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

// RestoreSettlement un-deletes a soft-deleted settlement (any group member).
func (s *Server) RestoreSettlement(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	settlementID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	out, err := s.Settlements.Restore(r.Context(), u.ID, settlementID)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "settlement not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		return
	case errors.Is(err, service.ErrAlreadyActive):
		writeErr(w, http.StatusConflict, "conflict", "settlement is not deleted")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPISettlement(out))
}

func toAPISettlement(s *repo.Settlement) apigen.Settlement {
	var note *string
	if s.Note != "" {
		n := s.Note
		note = &n
	}
	return apigen.Settlement{
		Id:          s.ID,
		GroupId:     s.GroupID,
		FromUserId:  s.FromUser,
		ToUserId:    s.ToUser,
		AmountCents: s.AmountCents,
		Note:        note,
		SettledAt:   s.SettledAt,
		CreatedAt:   s.CreatedAt,
		DeletedAt:   s.DeletedAt,
	}
}
