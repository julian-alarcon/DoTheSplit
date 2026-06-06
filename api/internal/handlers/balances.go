package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

func (s *Server) GetBalances(c *gin.Context) {
	u := middleware.User(c)
	groupID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	res, err := s.Balances.Get(c.Request.Context(), u.ID, groupID)
	if errors.Is(err, service.ErrNotMember) {
		writeErr(c, http.StatusForbidden, "forbidden", "not a group member")
		return
	}
	if err != nil {
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
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
	c.JSON(http.StatusOK, apigen.BalancesResponse{Net: net, Simplified: simp})
}

func (s *Server) ListSettlements(c *gin.Context) {
	u := middleware.User(c)
	groupID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	settlements, err := s.Settlements.List(c.Request.Context(), u.ID, groupID)
	if errors.Is(err, service.ErrNotMember) {
		writeErr(c, http.StatusForbidden, "forbidden", "not a group member")
		return
	}
	if err != nil {
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]apigen.Settlement, 0, len(settlements))
	for i := range settlements {
		out = append(out, toAPISettlement(&settlements[i]))
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) CreateSettlement(c *gin.Context) {
	u := middleware.User(c)
	groupID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var req apigen.CreateSettlementRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	note := ""
	if req.Note != nil {
		note = *req.Note
	}
	settledAt := time.Now().UTC()
	if req.SettledAt != nil {
		settledAt = *req.SettledAt
	}
	fromUserID := u.ID
	if req.FromUserId != nil {
		fromUserID = *req.FromUserId
	}
	out, err := s.Settlements.Create(c.Request.Context(), u.ID, service.CreateSettlementInput{
		GroupID:     groupID,
		FromUserID:  fromUserID,
		ToUserID:    req.ToUserId,
		AmountCents: req.AmountCents,
		Note:        note,
		SettledAt:   settledAt,
	})
	switch {
	case errors.Is(err, service.ErrNotMember):
		writeErr(c, http.StatusForbidden, "forbidden", "not a group member")
		return
	case err != nil:
		writeErr(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	c.JSON(http.StatusCreated, toAPISettlement(out))
}

func (s *Server) GetSettlement(c *gin.Context) {
	u := middleware.User(c)
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	st, err := s.Settlements.Get(c.Request.Context(), u.ID, id)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(c, http.StatusNotFound, "not_found", "settlement not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(c, http.StatusForbidden, "forbidden", "not a group member")
		return
	case err != nil:
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	c.JSON(http.StatusOK, toAPISettlement(st))
}

func (s *Server) UpdateSettlement(c *gin.Context) {
	u := middleware.User(c)
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var req apigen.UpdateSettlementRequest
	if !bindStrictJSON(c, &req) {
		return
	}
	in := service.UpdateSettlementInput{
		FromUserID:  req.FromUserId,
		ToUserID:    req.ToUserId,
		AmountCents: req.AmountCents,
		Note:        req.Note,
		SettledAt:   req.SettledAt,
	}
	out, err := s.Settlements.Update(c.Request.Context(), u.ID, id, in)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(c, http.StatusNotFound, "not_found", "settlement not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(c, http.StatusForbidden, "forbidden", "not a group member")
		return
	case err != nil:
		writeErr(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	c.JSON(http.StatusOK, toAPISettlement(out))
}

func (s *Server) DeleteSettlement(c *gin.Context) {
	u := middleware.User(c)
	settlementID, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	err := s.Settlements.Delete(c.Request.Context(), u.ID, settlementID)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeErr(c, http.StatusNotFound, "not_found", "settlement not found")
		return
	case errors.Is(err, service.ErrNotMember):
		writeErr(c, http.StatusForbidden, "forbidden", "not a group member")
		return
	case err != nil:
		writeErr(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
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
	}
}
