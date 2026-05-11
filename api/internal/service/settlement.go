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

// Get returns a single settlement by id, enforcing group membership.
func (s *SettlementService) Get(ctx context.Context, actorID, id uuid.UUID) (*repo.Settlement, error) {
	st, err := s.settlements.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if st.DeletedAt != nil {
		return nil, repo.ErrNotFound
	}
	ok, err := s.groups.IsMember(ctx, st.GroupID, actorID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotMember
	}
	return st, nil
}

// Delete soft-deletes a settlement. Any group member may delete; the row is
// preserved with deleted_at so the audit trail survives.
func (s *SettlementService) Delete(ctx context.Context, actorID, settlementID uuid.UUID) error {
	st, err := s.settlements.FindByID(ctx, settlementID)
	if errors.Is(err, repo.ErrNotFound) {
		return repo.ErrNotFound
	}
	if err != nil {
		return err
	}
	if st.DeletedAt != nil {
		return repo.ErrNotFound
	}
	ok, err := s.groups.IsMember(ctx, st.GroupID, actorID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotMember
	}
	return s.settlements.SoftDelete(ctx, settlementID)
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
