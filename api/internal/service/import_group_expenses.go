package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/julian-alarcon/dothesplit/api/internal/csvimport"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

// ImportGroupExpensesInput is what the handler hands to Run. The actor
// and groupID come from the path/session; only the body fields live here.
type ImportGroupExpensesInput struct {
	CSV    string
	DryRun bool
}

// ImportGroupExpensesPreviewRow mirrors the OpenAPI shape; the handler
// converts the time field.
type ImportGroupExpensesPreviewRow struct {
	Description      string
	IncurredAt       time.Time
	AmountCents      int64
	Currency         string
	CategorySlug     string
	PayerDisplayName string
}

// ImportGroupExpensesResult is what the importer returns to the handler.
// Same shape for dry-run and commit; the handler picks 200 vs 201.
type ImportGroupExpensesResult struct {
	ExpenseCount  int
	SkippedCount  int
	Skipped       []string
	Preview       []ImportGroupExpensesPreviewRow
	CSVCurrencies []string
}

// GroupExpenseImporter appends expenses from a DoTheSplit-shaped CSV
// into an existing group, deriving every split from the group's
// default-split rule (pinned 2-member percent or equal across all
// members).
type GroupExpenseImporter struct {
	pool       *pgxpool.Pool
	groupRepo  *repo.GroupRepo
	groups     *GroupService
	expenses   *ExpenseService
	categories *CategoryService
}

func NewGroupExpenseImporter(pool *pgxpool.Pool, groupRepo *repo.GroupRepo, groups *GroupService, expenses *ExpenseService, categories *CategoryService) *GroupExpenseImporter {
	return &GroupExpenseImporter{
		pool: pool, groupRepo: groupRepo, groups: groups,
		expenses: expenses, categories: categories,
	}
}

// Run is the single entry point. Validates membership, parses the CSV,
// resolves payers/categories/splits, and either previews (DryRun) or
// commits the expenses one by one through ExpenseService.Create.
func (s *GroupExpenseImporter) Run(ctx context.Context, actorID, groupID uuid.UUID, in ImportGroupExpensesInput) (ImportGroupExpensesResult, error) {
	if err := s.groups.RequireMember(ctx, groupID, actorID); err != nil {
		return ImportGroupExpensesResult{}, err
	}
	g, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return ImportGroupExpensesResult{}, err
	}
	parsed, err := csvimport.ParseGroupExpenses(in.CSV)
	if err != nil {
		return ImportGroupExpensesResult{}, err
	}
	members, err := s.groupRepo.ListMembers(ctx, groupID)
	if err != nil {
		return ImportGroupExpensesResult{}, err
	}
	liveMembers := make([]repo.GroupMember, 0, len(members))
	for _, m := range members {
		if m.DeletedAt == nil {
			liveMembers = append(liveMembers, m)
		}
	}
	if len(liveMembers) == 0 {
		return ImportGroupExpensesResult{}, fmt.Errorf("%w: no live members", ErrBadSplit)
	}
	nameIdx := make(map[string]uuid.UUID, len(liveMembers))
	for _, m := range liveMembers {
		nameIdx[strings.ToLower(strings.TrimSpace(m.DisplayName))] = m.UserID
	}
	displayName := func(id uuid.UUID) string {
		for _, m := range liveMembers {
			if m.UserID == id {
				return m.DisplayName
			}
		}
		return ""
	}

	splitMode, splitInputs := s.deriveSplit(g, liveMembers)

	cats, err := s.categories.List(ctx)
	if err != nil {
		return ImportGroupExpensesResult{}, err
	}
	otherID, err := s.categories.DefaultID(ctx)
	if err != nil {
		return ImportGroupExpensesResult{}, err
	}
	byLowerLabel := func(lbl string) (string, bool) {
		for _, c := range cats {
			if strings.EqualFold(c.Label, lbl) {
				return c.ID.String(), true
			}
		}
		return "", false
	}

	skipped := append([]string(nil), parsed.Skipped...)
	skippedCount := parsed.SkippedCount
	type plan struct {
		input     CreateExpenseInput
		preview   ImportGroupExpensesPreviewRow
		categoryS string
	}
	plans := make([]plan, 0, len(parsed.Rows))

	for _, row := range parsed.Rows {
		payerID, ok := s.resolvePayer(row.PayerName, actorID, nameIdx)
		if !ok {
			skippedCount++
			if len(skipped) < csvimport.MaxSkippedSamples {
				skipped = append(skipped, row.Raw)
			}
			continue
		}
		currency := row.Currency
		if currency == "" {
			currency = g.DefaultCurrency
		}
		var catUUID uuid.UUID
		var catSlug string
		if strings.TrimSpace(row.Category) == "" {
			catUUID = otherID
			catSlug = "other"
		} else {
			catIDStr := csvimport.MapCategory(row.Category, row.Description, byLowerLabel, otherID.String())
			parsedID, perr := uuid.Parse(catIDStr)
			if perr != nil {
				return ImportGroupExpensesResult{}, fmt.Errorf("internal: bad category id %q", catIDStr)
			}
			catUUID = parsedID
			for _, c := range cats {
				if c.ID == catUUID {
					catSlug = c.Slug
					break
				}
			}
		}
		when := row.IncurredAt
		if when.IsZero() {
			when = row.Date
		}
		input := CreateExpenseInput{
			GroupID:     groupID,
			PayerID:     payerID,
			CategoryID:  &catUUID,
			AmountCents: row.CostCents,
			Currency:    currency,
			Description: row.Description,
			Notes:       row.Notes,
			IncurredAt:  when,
			Mode:        splitMode,
			Splits:      splitInputs,
		}
		plans = append(plans, plan{
			input:     input,
			categoryS: catSlug,
			preview: ImportGroupExpensesPreviewRow{
				Description:      row.Description,
				IncurredAt:       when,
				AmountCents:      row.CostCents,
				Currency:         currency,
				CategorySlug:     catSlug,
				PayerDisplayName: displayName(payerID),
			},
		})
	}

	preview := make([]ImportGroupExpensesPreviewRow, 0, PreviewLimit)
	for i, p := range plans {
		if i >= PreviewLimit {
			break
		}
		preview = append(preview, p.preview)
	}

	res := ImportGroupExpensesResult{
		ExpenseCount:  len(plans),
		SkippedCount:  skippedCount,
		Skipped:       skipped,
		Preview:       preview,
		CSVCurrencies: append([]string(nil), parsed.Currencies...),
	}
	if in.DryRun {
		return res, nil
	}
	for _, p := range plans {
		if _, err := s.expenses.Create(ctx, actorID, p.input); err != nil {
			return ImportGroupExpensesResult{}, err
		}
	}
	return res, nil
}

// deriveSplit picks the split mode + per-user inputs for the whole import.
// 2-member group with a pinned default ⇒ percent (basis points). Otherwise
// equal across all live members. Returns inputs in a stable order (matches
// liveMembers / default-split order) so resolveSplits' remainder-cent
// distribution is deterministic.
func (s *GroupExpenseImporter) deriveSplit(g *repo.Group, liveMembers []repo.GroupMember) (SplitMode, []SplitInput) {
	if len(g.DefaultSplit) > 0 && len(liveMembers) == 2 {
		out := make([]SplitInput, 0, len(g.DefaultSplit))
		for _, e := range g.DefaultSplit {
			out = append(out, SplitInput{UserID: e.UserID, Value: e.BasisPoints})
		}
		return SplitPercent, out
	}
	out := make([]SplitInput, 0, len(liveMembers))
	for _, m := range liveMembers {
		out = append(out, SplitInput{UserID: m.UserID})
	}
	return SplitEqual, out
}

// resolvePayer returns the payer's user_id and ok=true when the row's
// Payer column matches a live member name (case-insensitive) or is
// empty (in which case the importing actor is the payer). An unknown
// non-empty name returns ok=false so the caller skips the row.
func (s *GroupExpenseImporter) resolvePayer(name string, actorID uuid.UUID, idx map[string]uuid.UUID) (uuid.UUID, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return actorID, true
	}
	if id, ok := idx[strings.ToLower(trimmed)]; ok {
		return id, true
	}
	return uuid.Nil, false
}

