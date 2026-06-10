package service

import (
	"context"
	"encoding/csv"
	"io"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

// GroupCSVExporter writes a group's full ledger as a CSV in the
// dothesplit format (a superset of the Splitwise export). The format
// is the input contract for both /v1/imports/splitwise (which skips
// the extra columns) and the future /v1/imports/dothesplit.
type GroupCSVExporter struct {
	groups      *GroupService
	groupRepo   *repo.GroupRepo
	expenses    *ExpenseService
	settlements *repo.SettlementRepo
	categories  *CategoryService
	users       *repo.UserRepo
}

func NewGroupCSVExporter(g *GroupService, gr *repo.GroupRepo, e *ExpenseService, st *repo.SettlementRepo, c *CategoryService, u *repo.UserRepo) *GroupCSVExporter {
	return &GroupCSVExporter{groups: g, groupRepo: gr, expenses: e, settlements: st, categories: c, users: u}
}

// ExportResult carries metadata the handler needs to build the response
// (filename, content type) along with the rendered body.
type ExportResult struct {
	GroupName string
	GeneratedAt time.Time
}

// Export writes the CSV body to w and returns metadata. Membership is
// enforced via GroupService.RequireMember.
func (s *GroupCSVExporter) Export(ctx context.Context, w io.Writer, actorID, groupID uuid.UUID) (ExportResult, error) {
	if err := s.groups.RequireMember(ctx, groupID, actorID); err != nil {
		return ExportResult{}, err
	}
	g, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return ExportResult{}, err
	}
	members, err := s.groupRepo.ListMembers(ctx, groupID)
	if err != nil {
		return ExportResult{}, err
	}
	exps, err := s.expenses.List(ctx, actorID, groupID)
	if err != nil {
		return ExportResult{}, err
	}
	stls, err := s.settlements.ListByGroup(ctx, groupID)
	if err != nil {
		return ExportResult{}, err
	}

	// userID -> display name. Members first; payers/creators that left
	// the group still show their stored display name (or the
	// "Deleted user #..." tombstone).
	nameByID := make(map[uuid.UUID]string, len(members))
	for _, m := range members {
		nameByID[m.UserID] = m.DisplayName
	}
	resolveName := func(id uuid.UUID) string {
		if n, ok := nameByID[id]; ok {
			return n
		}
		u, err := s.users.FindByID(ctx, id)
		if err != nil || u == nil {
			return ""
		}
		nameByID[id] = u.DisplayName
		return u.DisplayName
	}

	memberCount := len(members)
	memberIdx := make(map[uuid.UUID]int, memberCount)
	for i, m := range members {
		memberIdx[m.UserID] = i
	}

	type ledgerRow struct {
		Date        time.Time // incurred_at / settled_at (UTC)
		Created     time.Time
		Description string
		Category    string
		CostCents   int64
		Currency    string
		PayerName   string
		Notes       string
		CreatedBy   string
		Signed      []int64 // length == memberCount
	}

	rows := make([]ledgerRow, 0, len(exps)+len(stls))

	for _, e := range exps {
		signed := make([]int64, memberCount)
		paid := map[uuid.UUID]int64{e.PayerID: e.AmountCents}
		share := make(map[uuid.UUID]int64, len(e.Splits))
		for _, sp := range e.Splits {
			share[sp.UserID] += sp.ShareCents
		}
		for uid, idx := range memberIdx {
			signed[idx] = paid[uid] - share[uid]
		}
		// Anyone outside the current member set who appears on this
		// expense's splits/payer is silently absorbed - they do not
		// have a member column. The signed-cents invariant therefore
		// only holds for the current members; that is acceptable
		// because the CSV's per-row sum exists for the current
		// member set, which is the audience for the export.
		cat, err := s.categories.Resolve(ctx, &e.CategoryID)
		if err != nil {
			return ExportResult{}, err
		}
		rows = append(rows, ledgerRow{
			Date:        e.IncurredAt,
			Created:     e.CreatedAt,
			Description: e.Description,
			Category:    cat.Label,
			CostCents:   e.AmountCents,
			Currency:    e.Currency,
			PayerName:   resolveName(e.PayerID),
			Notes:       e.Notes,
			CreatedBy:   resolveName(e.CreatedBy),
			Signed:      signed,
		})
	}

	for _, st := range stls {
		if st.DeletedAt != nil {
			continue
		}
		signed := make([]int64, memberCount)
		if i, ok := memberIdx[st.FromUser]; ok {
			signed[i] += st.AmountCents
		}
		if i, ok := memberIdx[st.ToUser]; ok {
			signed[i] -= st.AmountCents
		}
		rows = append(rows, ledgerRow{
			Date:        st.SettledAt,
			Created:     st.CreatedAt,
			Description: st.Note,
			Category:    "Payment",
			CostCents:   st.AmountCents,
			Currency:    g.DefaultCurrency,
			PayerName:   resolveName(st.FromUser),
			Notes:       "",
			CreatedBy:   "",
			Signed:      signed,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if !rows[i].Date.Equal(rows[j].Date) {
			return rows[i].Date.Before(rows[j].Date)
		}
		return rows[i].Created.Before(rows[j].Created)
	})

	now := time.Now().UTC()
	cw := csv.NewWriter(w)

	header := []string{"Date", "Description", "Category", "Cost", "Currency", "Time", "Payer", "Notes", "Created", "CreatedBy"}
	for _, m := range members {
		header = append(header, neutralizeCSVField(m.DisplayName))
	}
	if err := cw.Write(header); err != nil {
		return ExportResult{}, err
	}

	totals := make([]int64, memberCount)
	for _, r := range rows {
		t := r.Date.UTC()
		c := r.Created.UTC()
		rec := []string{
			t.Format("2006-01-02"),
			neutralizeCSVField(r.Description),
			neutralizeCSVField(r.Category),
			centsToDecimal(r.CostCents),
			r.Currency,
			t.Format("15:04:05Z"),
			neutralizeCSVField(r.PayerName),
			neutralizeCSVField(r.Notes),
			c.Format(time.RFC3339),
			neutralizeCSVField(r.CreatedBy),
		}
		for i, v := range r.Signed {
			rec = append(rec, centsToDecimal(v))
			totals[i] += v
		}
		if err := cw.Write(rec); err != nil {
			return ExportResult{}, err
		}
	}

	footer := []string{
		now.Format("2006-01-02"),
		"Total balance",
		" ",
		" ",
		g.DefaultCurrency,
		now.Format("15:04:05Z"),
		" ",
		" ",
		now.Format(time.RFC3339),
		" ",
	}
	for _, v := range totals {
		footer = append(footer, centsToDecimal(v))
	}
	if err := cw.Write(footer); err != nil {
		return ExportResult{}, err
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return ExportResult{}, err
	}
	return ExportResult{GroupName: g.Name, GeneratedAt: now}, nil
}

// centsToDecimal renders an int64 cents value as a decimal with 2
// fraction digits (no thousand separators, locale-independent).
// Mirrors the format the Splitwise importer's parseDecimalToCents
// expects.
func centsToDecimal(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	whole := v / 100
	frac := v % 100
	s := strconv.FormatInt(whole, 10) + "." + twoDigits(frac)
	if neg {
		s = "-" + s
	}
	return s
}

func twoDigits(v int64) string {
	if v < 10 {
		return "0" + strconv.FormatInt(v, 10)
	}
	return strconv.FormatInt(v, 10)
}

// neutralizeCSVField defuses spreadsheet formula injection. A cell whose first
// character is one of = + - @ (or a leading TAB/CR, which some parsers strip
// before re-evaluating the next char) is treated as a live formula by Excel,
// LibreOffice, and Google Sheets when the export is opened. Prefixing such a
// value with a single quote forces the spreadsheet to render it as literal
// text. Empty cells are left untouched.
func neutralizeCSVField(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

