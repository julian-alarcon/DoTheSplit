package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type RecurringService struct {
	recurring     repo.RecurringRepo
	expenses      repo.ExpenseRepo
	groups        repo.GroupRepo
	categories    *CategoryService
	users         repo.UserRepo
	notifications *NotificationService
}

func NewRecurringService(r repo.RecurringRepo, e repo.ExpenseRepo, g repo.GroupRepo, c *CategoryService) *RecurringService {
	return &RecurringService{recurring: r, expenses: e, groups: g, categories: c}
}

// SetNotifications enables per-member email notifications for materialized
// recurring runs. Optional - left nil in tests that don't exercise the mailer.
func (s *RecurringService) SetNotifications(users repo.UserRepo, n *NotificationService) {
	s.users = users
	s.notifications = n
}

type CreateRecurringInput struct {
	GroupID     uuid.UUID
	PayerID     uuid.UUID
	CategoryID  *uuid.UUID
	AmountCents int64
	Currency    string
	Description string
	Mode        SplitMode
	Splits      []SplitInput
	Cadence     string
	NextRunAt   time.Time
}

func (s *RecurringService) Create(ctx context.Context, actorID uuid.UUID, in CreateRecurringInput) (*repo.RecurringExpense, error) {
	if ok, err := s.groups.IsMember(ctx, in.GroupID, actorID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotMember
	}
	if !isValidCadence(in.Cadence) {
		return nil, errors.New("invalid cadence")
	}
	if in.AmountCents <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	// Validate the template by trying to resolve it now against the current members.
	if _, err := resolveSplits(in.Mode, in.AmountCents, in.Splits); err != nil {
		return nil, err
	}
	if in.Currency == "" {
		g, err := s.groups.FindByID(ctx, in.GroupID)
		if err != nil {
			return nil, err
		}
		in.Currency = g.DefaultCurrency
	}
	cat, err := s.categories.Resolve(ctx, in.CategoryID)
	if err != nil {
		return nil, err
	}
	tmpl := make([]repo.SplitTemplateEntry, len(in.Splits))
	for i, sp := range in.Splits {
		tmpl[i] = repo.SplitTemplateEntry{UserID: sp.UserID, Value: sp.Value}
	}
	e := &repo.RecurringExpense{
		GroupID: in.GroupID, PayerID: in.PayerID,
		CategoryID:  cat.ID,
		AmountCents: in.AmountCents, Currency: in.Currency,
		Description: in.Description, Mode: string(in.Mode),
		SplitTemplate: tmpl,
		Cadence:       in.Cadence, NextRunAt: in.NextRunAt,
	}
	if err := s.recurring.Create(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *RecurringService) List(ctx context.Context, actorID, groupID uuid.UUID) ([]repo.RecurringExpense, error) {
	if ok, err := s.groups.IsMember(ctx, groupID, actorID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotMember
	}
	return s.recurring.ListByGroup(ctx, groupID)
}

// Delete is allowed for any group member (v1 simplification).
func (s *RecurringService) Delete(ctx context.Context, actorID, id uuid.UUID) error {
	e, err := s.recurring.FindByID(ctx, id)
	if errors.Is(err, repo.ErrNotFound) {
		return repo.ErrNotFound
	}
	if err != nil {
		return err
	}
	if ok, err := s.groups.IsMember(ctx, e.GroupID, actorID); err != nil {
		return err
	} else if !ok {
		return ErrForbidden
	}
	return s.recurring.SoftDelete(ctx, id)
}

// notifyPlan carries the data needed to email members about one materialized
// recurring run. Collected inside the claim tx and dispatched after commit so
// the notification reads never contend with the open write transaction - which
// matters on SQLite, where a single-writer connection held by the tx would
// otherwise deadlock a concurrent pool read.
type notifyPlan struct {
	groupID     uuid.UUID
	description string
	amount      string
}

// Tick materializes every due recurring expense into a regular expense row and
// advances next_run_at by the cadence. Returns the number of expenses created.
//
// All writes for the batch (materialized expenses, their activity events, and
// the next_run_at advances) happen on the single transaction returned by
// ClaimDue, so a partial failure rolls the whole tick back instead of
// materializing expenses whose template still points at the old run time (which
// would double-materialize on the next tick). Member notifications are
// dispatched only after the commit succeeds.
func (s *RecurringService) Tick(ctx context.Context) (int, error) {
	tx, due, err := s.recurring.ClaimDue(ctx, 100)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // commit below replaces this

	created := 0
	var plans []notifyPlan
	for _, r := range due {
		splits := make([]SplitInput, len(r.SplitTemplate))
		for i, t := range r.SplitTemplate {
			splits[i] = SplitInput{UserID: t.UserID, Value: t.Value}
		}
		shares, err := resolveSplits(SplitMode(r.Mode), r.AmountCents, splits)
		if err != nil {
			return 0, fmt.Errorf("resolve splits for recurring %s: %w", r.ID, err)
		}
		// The recurring template doesn't track who set it up, so attribute
		// materialized expenses to the payer. Good enough - it's the same
		// user the template "belongs to" - and avoids leaving a NULL behind.
		rID := r.ID
		e := &repo.Expense{
			GroupID:            r.GroupID,
			PayerID:            r.PayerID,
			CreatedBy:          r.PayerID,
			CategoryID:         r.CategoryID,
			AmountCents:        r.AmountCents,
			Currency:           r.Currency,
			Description:        r.Description,
			IncurredAt:         r.NextRunAt,
			RecurringExpenseID: &rID,
			Splits:             shares,
		}
		// Materialize on the claim tx. On Postgres the tx already holds
		// FOR NO KEY UPDATE on the recurring row; taking the FK's FOR KEY SHARE
		// on that same row within the same tx never blocks. On SQLite the single
		// writer runs everything on this one connection.
		if err := s.expenses.CreateWithSplitsTx(ctx, tx, e); err != nil {
			return 0, err
		}
		next := advanceCadence(r.NextRunAt, r.Cadence)
		if err := s.recurring.UpdateNextRunTx(ctx, tx, r.ID, next); err != nil {
			return 0, err
		}
		if s.notifications != nil && s.users != nil {
			plans = append(plans, notifyPlan{
				groupID:     r.GroupID,
				description: r.Description,
				amount:      formatMoney(r.AmountCents, r.Currency),
			})
		}
		created++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	// Notify opted-in members after commit. Best-effort: failures don't affect
	// the already-committed materialization.
	for _, p := range plans {
		groupName := ""
		if g, gerr := s.groups.FindByID(ctx, p.groupID); gerr == nil {
			groupName = g.Name
		}
		members, _ := s.groups.ListMembers(ctx, p.groupID)
		for _, m := range members {
			_ = s.notifications.NotifyIfEnabled(ctx, nil, m.UserID,
				PrefKeyRecurringRun, "recurring_run", TemplateVars{
					GroupName:   groupName,
					Description: p.description,
					Amount:      p.amount,
				})
		}
	}
	return created, nil
}

func isValidCadence(c string) bool {
	switch c {
	case "daily", "weekly", "biweekly", "monthly", "yearly":
		return true
	}
	return false
}

func advanceCadence(from time.Time, cadence string) time.Time {
	switch cadence {
	case "daily":
		return from.AddDate(0, 0, 1)
	case "weekly":
		return from.AddDate(0, 0, 7)
	case "biweekly":
		return from.AddDate(0, 0, 14)
	case "monthly":
		return from.AddDate(0, 1, 0)
	case "yearly":
		return from.AddDate(1, 0, 0)
	}
	return from
}
