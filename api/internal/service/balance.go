package service

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type SimplifiedDebt struct {
	FromUserID  uuid.UUID
	ToUserID    uuid.UUID
	AmountCents int64
}

type BalancesResult struct {
	Net        []repo.NetBalance
	Simplified []SimplifiedDebt
}

type BalanceService struct {
	balances *repo.BalanceRepo
	groups   *repo.GroupRepo
}

func NewBalanceService(b *repo.BalanceRepo, g *repo.GroupRepo) *BalanceService {
	return &BalanceService{balances: b, groups: g}
}

// Get enforces group membership and returns net + simplified debts.
func (s *BalanceService) Get(ctx context.Context, actorID, groupID uuid.UUID) (*BalancesResult, error) {
	ok, err := s.groups.IsMember(ctx, groupID, actorID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotMember
	}
	nets, err := s.balances.NetBalances(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return &BalancesResult{Net: nets, Simplified: Simplify(nets)}, nil
}

// Simplify collapses net balances into the minimum number of pairwise transfers
// using a greedy algorithm: pair the largest creditor with the largest debtor,
// transfer the smaller magnitude, repeat. Deterministic ordering by user_id on ties.
func Simplify(nets []repo.NetBalance) []SimplifiedDebt {
	type entry struct {
		id  uuid.UUID
		amt int64
	}
	creditors := make([]entry, 0)
	debtors := make([]entry, 0)
	for _, n := range nets {
		switch {
		case n.NetCents > 0:
			creditors = append(creditors, entry{n.UserID, n.NetCents})
		case n.NetCents < 0:
			debtors = append(debtors, entry{n.UserID, -n.NetCents})
		}
	}
	sort.SliceStable(creditors, func(i, j int) bool {
		if creditors[i].amt != creditors[j].amt {
			return creditors[i].amt > creditors[j].amt
		}
		return creditors[i].id.String() < creditors[j].id.String()
	})
	sort.SliceStable(debtors, func(i, j int) bool {
		if debtors[i].amt != debtors[j].amt {
			return debtors[i].amt > debtors[j].amt
		}
		return debtors[i].id.String() < debtors[j].id.String()
	})

	var out []SimplifiedDebt
	i, j := 0, 0
	for i < len(creditors) && j < len(debtors) {
		amt := creditors[i].amt
		if debtors[j].amt < amt {
			amt = debtors[j].amt
		}
		if amt > 0 {
			out = append(out, SimplifiedDebt{
				FromUserID:  debtors[j].id,
				ToUserID:    creditors[i].id,
				AmountCents: amt,
			})
		}
		creditors[i].amt -= amt
		debtors[j].amt -= amt
		if creditors[i].amt == 0 {
			i++
		}
		if debtors[j].amt == 0 {
			j++
		}
	}
	return out
}
