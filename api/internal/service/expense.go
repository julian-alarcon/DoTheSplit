package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type SplitMode string

const (
	SplitEqual   SplitMode = "equal"
	SplitExact   SplitMode = "exact"
	SplitPercent SplitMode = "percent"
	SplitShares  SplitMode = "shares"
)

// SplitInput is the raw user input: user_id + an optional value whose meaning
// depends on mode (cents for exact, basis points for percent, integer weight for shares).
type SplitInput struct {
	UserID uuid.UUID
	Value  int64
}

var (
	ErrBadSplit       = errors.New("invalid split")
	ErrPayerNotMember = errors.New("payer is not a group member")
	ErrSplitNotMember = errors.New("split user is not a group member")
	ErrForbidden      = errors.New("forbidden")
)

type ExpenseService struct {
	exps       *repo.ExpenseRepo
	groups     *repo.GroupRepo
	categories *CategoryService
}

func NewExpenseService(e *repo.ExpenseRepo, g *repo.GroupRepo, c *CategoryService) *ExpenseService {
	return &ExpenseService{exps: e, groups: g, categories: c}
}

type CreateExpenseInput struct {
	GroupID     uuid.UUID
	PayerID     uuid.UUID
	CategoryID  *uuid.UUID
	AmountCents int64
	Currency    string
	Description string
	IncurredAt  time.Time
	Mode        SplitMode
	Splits      []SplitInput
}

// Create validates inputs, resolves the split mode to exact share_cents per user,
// and writes the expense + splits in a single transaction.
func (s *ExpenseService) Create(ctx context.Context, actorID uuid.UUID, in CreateExpenseInput) (*repo.Expense, error) {
	if err := s.requireMember(ctx, in.GroupID, actorID); err != nil {
		return nil, err
	}
	if in.AmountCents <= 0 {
		return nil, fmt.Errorf("%w: amount must be > 0", ErrBadSplit)
	}
	if in.Description == "" {
		return nil, fmt.Errorf("%w: description required", ErrBadSplit)
	}
	if in.Currency == "" {
		g, err := s.groups.FindByID(ctx, in.GroupID)
		if err != nil {
			return nil, err
		}
		in.Currency = g.DefaultCurrency
	}
	if in.IncurredAt.IsZero() {
		in.IncurredAt = time.Now().UTC()
	}

	// Validate payer is a group member.
	if ok, err := s.groups.IsMember(ctx, in.GroupID, in.PayerID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrPayerNotMember
	}

	// Validate every split user is a member, and resolve shares.
	for _, sp := range in.Splits {
		if ok, err := s.groups.IsMember(ctx, in.GroupID, sp.UserID); err != nil {
			return nil, err
		} else if !ok {
			return nil, ErrSplitNotMember
		}
	}

	shares, err := resolveSplits(in.Mode, in.AmountCents, in.Splits)
	if err != nil {
		return nil, err
	}

	cat, err := s.categories.Resolve(ctx, in.CategoryID)
	if err != nil {
		return nil, err
	}

	e := &repo.Expense{
		GroupID:     in.GroupID,
		PayerID:     in.PayerID,
		CategoryID:  cat.ID,
		AmountCents: in.AmountCents,
		Currency:    in.Currency,
		Description: in.Description,
		IncurredAt:  in.IncurredAt,
		Splits:      shares,
	}
	if err := s.exps.CreateWithSplits(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// Get returns a single expense by id, enforcing group membership.
func (s *ExpenseService) Get(ctx context.Context, actorID, id uuid.UUID) (*repo.Expense, error) {
	e, err := s.exps.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if e.DeletedAt != nil {
		return nil, repo.ErrNotFound
	}
	if err := s.requireMember(ctx, e.GroupID, actorID); err != nil {
		return nil, err
	}
	return e, nil
}

// UpdateExpenseInput mirrors the PATCH body (all fields optional).
type UpdateExpenseInput struct {
	Description *string
	AmountCents *int64
	CategoryID  *uuid.UUID
	PayerID     *uuid.UUID
}

// Update edits description / amount / category / payer on an expense.
// Only the current payer or group creator may update. Splits are rescaled when amount changes.
// Every changed field appends an expense_revisions row.
func (s *ExpenseService) Update(ctx context.Context, actorID, expenseID uuid.UUID, in UpdateExpenseInput) (*repo.Expense, error) {
	if in.Description == nil && in.AmountCents == nil && in.CategoryID == nil && in.PayerID == nil {
		return nil, fmt.Errorf("%w: nothing to update", ErrBadSplit)
	}
	if in.AmountCents != nil && *in.AmountCents <= 0 {
		return nil, fmt.Errorf("%w: amount must be > 0", ErrBadSplit)
	}
	if in.Description != nil && *in.Description == "" {
		return nil, fmt.Errorf("%w: description cannot be empty", ErrBadSplit)
	}
	existing, err := s.exps.FindByID(ctx, expenseID)
	if err != nil {
		return nil, err
	}
	if existing.DeletedAt != nil {
		return nil, repo.ErrNotFound
	}
	g, err := s.groups.FindByID(ctx, existing.GroupID)
	if err != nil {
		return nil, err
	}
	if actorID != existing.PayerID && actorID != g.CreatedBy {
		return nil, ErrForbidden
	}
	if in.CategoryID != nil {
		if _, err := s.categories.Resolve(ctx, in.CategoryID); err != nil {
			return nil, err
		}
	}
	if in.PayerID != nil {
		ok, err := s.groups.IsMember(ctx, existing.GroupID, *in.PayerID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrPayerNotMember
		}
	}
	return s.exps.UpdateWithRescale(ctx, expenseID, actorID, in.Description, in.AmountCents, in.CategoryID, in.PayerID)
}

// ListRevisions returns the full edit history of an expense (oldest first).
// Membership is enforced through the expense's group.
func (s *ExpenseService) ListRevisions(ctx context.Context, actorID, expenseID uuid.UUID) ([]repo.ExpenseRevision, error) {
	if _, err := s.Get(ctx, actorID, expenseID); err != nil {
		return nil, err
	}
	return s.exps.ListRevisions(ctx, expenseID)
}

func (s *ExpenseService) List(ctx context.Context, actorID, groupID uuid.UUID) ([]repo.Expense, error) {
	if err := s.requireMember(ctx, groupID, actorID); err != nil {
		return nil, err
	}
	return s.exps.ListByGroup(ctx, groupID)
}

// Delete is allowed for the payer or the group creator. Soft-deletes.
func (s *ExpenseService) Delete(ctx context.Context, actorID, expenseID uuid.UUID) error {
	e, err := s.exps.FindByID(ctx, expenseID)
	if errors.Is(err, repo.ErrNotFound) {
		return repo.ErrNotFound
	}
	if err != nil {
		return err
	}
	if e.DeletedAt != nil {
		return repo.ErrNotFound
	}
	// Load group to check creator.
	g, err := s.groups.FindByID(ctx, e.GroupID)
	if err != nil {
		return err
	}
	if actorID != e.PayerID && actorID != g.CreatedBy {
		return ErrForbidden
	}
	return s.exps.SoftDelete(ctx, expenseID)
}

func (s *ExpenseService) requireMember(ctx context.Context, groupID, userID uuid.UUID) error {
	ok, err := s.groups.IsMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotMember
	}
	return nil
}

// resolveSplits turns user input + mode into final share_cents per user,
// preserving the invariant that shares sum to amount (remainder cents are
// distributed deterministically to early splits).
func resolveSplits(mode SplitMode, amount int64, in []SplitInput) ([]repo.Split, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("%w: no splits provided", ErrBadSplit)
	}
	// Detect duplicates.
	seen := map[uuid.UUID]bool{}
	for _, sp := range in {
		if seen[sp.UserID] {
			return nil, fmt.Errorf("%w: duplicate user in splits", ErrBadSplit)
		}
		seen[sp.UserID] = true
	}

	out := make([]repo.Split, len(in))
	for i, sp := range in {
		out[i].UserID = sp.UserID
	}

	switch mode {
	case SplitEqual:
		base := amount / int64(len(in))
		rem := amount - base*int64(len(in))
		for i := range out {
			out[i].ShareCents = base
		}
		for i := int64(0); i < rem; i++ {
			out[i].ShareCents++
		}
	case SplitExact:
		var sum int64
		for i, sp := range in {
			if sp.Value < 0 {
				return nil, fmt.Errorf("%w: exact share must be >= 0", ErrBadSplit)
			}
			out[i].ShareCents = sp.Value
			sum += sp.Value
		}
		if sum != amount {
			return nil, fmt.Errorf("%w: exact shares sum to %d, want %d", ErrBadSplit, sum, amount)
		}
	case SplitPercent:
		var bps int64
		for _, sp := range in {
			if sp.Value < 0 {
				return nil, fmt.Errorf("%w: percent value must be >= 0", ErrBadSplit)
			}
			bps += sp.Value
		}
		if bps != 10000 {
			return nil, fmt.Errorf("%w: percents must sum to 100 (10000 bps), got %d", ErrBadSplit, bps)
		}
		var assigned int64
		for i, sp := range in {
			share := amount * sp.Value / 10000
			out[i].ShareCents = share
			assigned += share
		}
		// Distribute rounding remainder to early users.
		for i := int64(0); i < amount-assigned; i++ {
			out[i].ShareCents++
		}
	case SplitShares:
		var total int64
		for _, sp := range in {
			if sp.Value <= 0 {
				return nil, fmt.Errorf("%w: share weights must be > 0", ErrBadSplit)
			}
			total += sp.Value
		}
		var assigned int64
		for i, sp := range in {
			share := amount * sp.Value / total
			out[i].ShareCents = share
			assigned += share
		}
		for i := int64(0); i < amount-assigned; i++ {
			out[i].ShareCents++
		}
	default:
		return nil, fmt.Errorf("%w: unknown mode %q", ErrBadSplit, mode)
	}
	return out, nil
}
