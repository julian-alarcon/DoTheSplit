package service

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

var ErrBadCursor = errors.New("invalid cursor")

const (
	transactionDefaultLimit = 50
	transactionMaxLimit     = 100
)

type TransactionService struct {
	groups      *GroupService
	transaction repo.TransactionRepo
	expenses    repo.ExpenseRepo
	settlements repo.SettlementRepo
	recurring   repo.RecurringRepo
}

func NewTransactionService(g *GroupService, a repo.TransactionRepo, e repo.ExpenseRepo, s repo.SettlementRepo, r repo.RecurringRepo) *TransactionService {
	return &TransactionService{groups: g, transaction: a, expenses: e, settlements: s, recurring: r}
}

// TransactionItem mirrors the OpenAPI schema: exactly one of Expense / Settlement
// is set, with the discriminator in Kind. Cadence is non-empty only when the
// item is an expense whose content matches a recurring template.
type TransactionItem struct {
	Kind       repo.TransactionKind
	OccurredAt time.Time
	Cadence    string // empty when not applicable
	Expense    *repo.Expense
	Settlement *repo.Settlement
}

type TransactionPage struct {
	Items      []TransactionItem
	NextCursor string // empty when there are no more items
}

// List returns one page of the merged transaction feed for the group. It enforces
// membership, hydrates expense/settlement payloads in batched queries, and
// emits an opaque cursor that continues strictly after the last returned row.
func (s *TransactionService) List(ctx context.Context, actorID, groupID uuid.UUID, limit int, cursor string) (*TransactionPage, error) {
	if err := s.groups.RequireMember(ctx, groupID, actorID); err != nil {
		return nil, err
	}
	after, err := decodeTransactionCursor(cursor)
	if err != nil {
		return nil, ErrBadCursor
	}
	if limit <= 0 {
		limit = transactionDefaultLimit
	}
	if limit > transactionMaxLimit {
		limit = transactionMaxLimit
	}
	rows, err := s.transaction.ListByGroup(ctx, groupID, limit+1, after)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	// Collect ids per kind for batch hydration.
	var expenseIDs, settlementIDs []uuid.UUID
	for _, r := range rows {
		switch r.Kind {
		case repo.TransactionExpense:
			expenseIDs = append(expenseIDs, r.ID)
		case repo.TransactionSettlement:
			settlementIDs = append(settlementIDs, r.ID)
		}
	}
	expenses, err := s.expenses.FindByIDs(ctx, expenseIDs)
	if err != nil {
		return nil, err
	}
	settlements, err := s.settlements.FindByIDs(ctx, settlementIDs)
	if err != nil {
		return nil, err
	}
	// Cadence comes straight from the recurring_expense_id FK stamped on the
	// expense at materialization time. Batch-load the cadence for the distinct
	// templates referenced on this page.
	var cadenceByExpense map[uuid.UUID]string
	if len(expenseIDs) > 0 {
		var templateIDs []uuid.UUID
		seen := map[uuid.UUID]bool{}
		expenseToTemplate := map[uuid.UUID]uuid.UUID{}
		for _, id := range expenseIDs {
			e, ok := expenses[id]
			if !ok || e.RecurringExpenseID == nil {
				continue
			}
			expenseToTemplate[id] = *e.RecurringExpenseID
			if !seen[*e.RecurringExpenseID] {
				seen[*e.RecurringExpenseID] = true
				templateIDs = append(templateIDs, *e.RecurringExpenseID)
			}
		}
		cadenceByTemplate, err := s.recurring.CadenceByIDs(ctx, templateIDs)
		if err != nil {
			return nil, err
		}
		cadenceByExpense = make(map[uuid.UUID]string, len(expenseToTemplate))
		for expID, tmplID := range expenseToTemplate {
			if c, ok := cadenceByTemplate[tmplID]; ok {
				cadenceByExpense[expID] = c
			}
		}
	}
	items := make([]TransactionItem, 0, len(rows))
	for _, row := range rows {
		item := TransactionItem{Kind: row.Kind, OccurredAt: row.OccurredAt}
		switch row.Kind {
		case repo.TransactionExpense:
			e, ok := expenses[row.ID]
			if !ok {
				// The row was soft-deleted between the index query and the
				// hydration query - skip it. Pagination still progresses.
				continue
			}
			item.Expense = e
			if c, ok := cadenceByExpense[row.ID]; ok {
				item.Cadence = c
			}
		case repo.TransactionSettlement:
			st, ok := settlements[row.ID]
			if !ok {
				continue
			}
			item.Settlement = &st
		}
		items = append(items, item)
	}
	page := &TransactionPage{Items: items}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor = encodeTransactionCursor(last)
	}
	return page, nil
}

// Cursor format: base64-url(occurred_at | created_at | kind | uuid), each
// time RFC3339Nano. Pipes are safe because RFC3339 doesn't use them and our
// kinds don't either. created_at is the secondary sort key; kind is kept in
// the payload for forward compatibility but is not used in the WHERE clause.
func encodeTransactionCursor(r repo.TransactionRow) string {
	raw := r.OccurredAt.UTC().Format(time.RFC3339Nano) + "|" +
		r.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" +
		string(r.Kind) + "|" + r.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeTransactionCursor(s string) (*repo.TransactionRow, error) {
	if s == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(decoded), "|", 4)
	if len(parts) != 4 {
		return nil, errors.New("malformed cursor")
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return nil, err
	}
	kind := repo.TransactionKind(parts[2])
	if kind != repo.TransactionExpense && kind != repo.TransactionSettlement {
		return nil, errors.New("malformed cursor kind")
	}
	id, err := uuid.Parse(parts[3])
	if err != nil {
		return nil, err
	}
	return &repo.TransactionRow{Kind: kind, OccurredAt: occurredAt, CreatedAt: createdAt, ID: id}, nil
}
