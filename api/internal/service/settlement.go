package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type SettlementService struct {
	settlements *repo.SettlementRepo
	groups      *repo.GroupRepo
}

func NewSettlementService(s *repo.SettlementRepo, g *repo.GroupRepo) *SettlementService {
	return &SettlementService{settlements: s, groups: g}
}

type CreateSettlementInput struct {
	GroupID     uuid.UUID
	FromUserID  uuid.UUID
	ToUserID    uuid.UUID
	AmountCents int64
	Note        string
	SettledAt   time.Time
}

func (s *SettlementService) Create(ctx context.Context, actorID uuid.UUID, in CreateSettlementInput) (*repo.Settlement, error) {
	if in.AmountCents <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	if in.FromUserID == in.ToUserID {
		return nil, errors.New("from and to must differ")
	}
	// Actor must be the payer (from_user).
	if actorID != in.FromUserID {
		return nil, ErrForbidden
	}
	// Both parties must be group members.
	for _, uid := range []uuid.UUID{in.FromUserID, in.ToUserID} {
		ok, err := s.groups.IsMember(ctx, in.GroupID, uid)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrNotMember
		}
	}
	if in.SettledAt.IsZero() {
		in.SettledAt = time.Now().UTC()
	}
	st := &repo.Settlement{
		GroupID:     in.GroupID,
		FromUser:    in.FromUserID,
		ToUser:      in.ToUserID,
		AmountCents: in.AmountCents,
		Note:        in.Note,
		SettledAt:   in.SettledAt,
	}
	if err := s.settlements.Create(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *SettlementService) List(ctx context.Context, actorID, groupID uuid.UUID) ([]repo.Settlement, error) {
	ok, err := s.groups.IsMember(ctx, groupID, actorID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotMember
	}
	return s.settlements.ListByGroup(ctx, groupID)
}
