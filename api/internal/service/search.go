package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

var ErrBadSearchQuery = errors.New("query is too short")

const (
	searchMinQueryLen = 2
	searchMaxQueryLen = 200
	searchDefaultLim  = 100
	searchMaxLim      = 200
)

// SearchService finds expenses + settlements across all groups the actor is a
// member of whose description / notes / settlement note contain a case-
// insensitive substring of the supplied query. The result is a single flat
// list with the actor's groups (or the requested subset they actually belong
// to) attached, so the client can group-by and render filter chips with no
// follow-up round-trip.
type SearchService struct {
	groups   *GroupService
	groupsRp *repo.GroupRepo
	search   *repo.SearchRepo
	expenses *repo.ExpenseRepo
	settles  *repo.SettlementRepo
}

func NewSearchService(g *GroupService, gr *repo.GroupRepo, s *repo.SearchRepo, e *repo.ExpenseRepo, st *repo.SettlementRepo) *SearchService {
	return &SearchService{groups: g, groupsRp: gr, search: s, expenses: e, settles: st}
}

// SearchResult bundles flat hits with the (member-only) group descriptors the
// client needs to render section headers and the per-group filter chip row.
type SearchResult struct {
	Query                string
	Items                []ActivityItem
	Groups               []SearchGroupInfo
	AvailableCategoryIDs []uuid.UUID
}

type SearchGroupInfo struct {
	Group   repo.Group
	Members []repo.GroupMember
}

// Search runs the query. `requestedGroups` is the optional `group_id` filter
// from the request - any id the actor is not a member of is silently dropped
// (matches the OpenAPI spec). When the resulting set is empty (either the
// actor has no groups, or every requested id was filtered out), the response
// has no items but still echoes the query so the client renders a clean
// "no results" state instead of a 404.
//
// `categoryID` (optional) narrows the result to one expense category and
// excludes settlements entirely. An unknown id naturally produces zero
// matches.
func (s *SearchService) Search(ctx context.Context, actorID uuid.UUID, q string, requestedGroups []uuid.UUID, categoryID *uuid.UUID, limit int) (*SearchResult, error) {
	q = strings.TrimSpace(q)
	if len(q) < searchMinQueryLen {
		return nil, ErrBadSearchQuery
	}
	if len(q) > searchMaxQueryLen {
		q = q[:searchMaxQueryLen]
	}
	if limit <= 0 {
		limit = searchDefaultLim
	}
	if limit > searchMaxLim {
		limit = searchMaxLim
	}

	allGroups, err := s.groupsRp.ListForUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	memberGroupIDs := make(map[uuid.UUID]bool, len(allGroups))
	for _, g := range allGroups {
		memberGroupIDs[g.ID] = true
	}

	// Resolve the effective group set. If the caller asked for specific group
	// ids, intersect with their memberships; otherwise search them all.
	var effectiveGroups []repo.Group
	if len(requestedGroups) > 0 {
		filterSet := make(map[uuid.UUID]bool, len(requestedGroups))
		for _, id := range requestedGroups {
			if memberGroupIDs[id] {
				filterSet[id] = true
			}
		}
		for _, g := range allGroups {
			if filterSet[g.ID] {
				effectiveGroups = append(effectiveGroups, g)
			}
		}
	} else {
		effectiveGroups = allGroups
	}

	groupInfos := make([]SearchGroupInfo, 0, len(effectiveGroups))
	groupIDs := make([]uuid.UUID, 0, len(effectiveGroups))
	for _, g := range effectiveGroups {
		members, err := s.groupsRp.ListMembers(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		groupInfos = append(groupInfos, SearchGroupInfo{Group: g, Members: members})
		groupIDs = append(groupIDs, g.ID)
	}

	result := &SearchResult{Query: q, Groups: groupInfos}
	if len(groupIDs) == 0 {
		return result, nil
	}

	rows, err := s.search.SearchActivity(ctx, groupIDs, q, categoryID, limit)
	if err != nil {
		return nil, err
	}

	// Available categories are computed independently of `categoryID` so the
	// client can still offer those categories as switchable options in the
	// filter picker after the user has narrowed.
	availCats, err := s.search.AvailableCategories(ctx, groupIDs, q)
	if err != nil {
		return nil, err
	}
	result.AvailableCategoryIDs = availCats

	var expenseIDs, settlementIDs []uuid.UUID
	for _, r := range rows {
		switch r.Kind {
		case repo.ActivityExpense:
			expenseIDs = append(expenseIDs, r.ID)
		case repo.ActivitySettlement:
			settlementIDs = append(settlementIDs, r.ID)
		}
	}
	expenses, err := s.expenses.FindByIDs(ctx, expenseIDs)
	if err != nil {
		return nil, err
	}
	settlements, err := s.settles.FindByIDs(ctx, settlementIDs)
	if err != nil {
		return nil, err
	}

	items := make([]ActivityItem, 0, len(rows))
	for _, row := range rows {
		item := ActivityItem{Kind: row.Kind, OccurredAt: row.OccurredAt}
		switch row.Kind {
		case repo.ActivityExpense:
			e, ok := expenses[row.ID]
			if !ok {
				continue
			}
			item.Expense = e
		case repo.ActivitySettlement:
			st, ok := settlements[row.ID]
			if !ok {
				continue
			}
			item.Settlement = &st
		}
		items = append(items, item)
	}
	result.Items = items
	return result, nil
}
