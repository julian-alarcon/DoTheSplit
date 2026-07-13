package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/api/internal/crypto"
	"github.com/julian-alarcon/dothesplit/api/internal/csvimport"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

// PreviewLimit is the number of expense rows surfaced back to the importer UI.
const PreviewLimit = 6

// ImportSplitwiseInput captures the user-confirmed metadata that accompanies
// the raw CSV. Members are listed in CSV column order; the importer fails
// if the count doesn't match the parsed header.
type ImportSplitwiseInput struct {
	CSV             string
	GroupName       string
	DefaultCurrency string
	Members         []ImportSplitwiseMember
	DryRun          bool
}

type ImportSplitwiseMember struct {
	CSVName string
	Email   string
}

// ImportSplitwisePreviewRow mirrors the OpenAPI shape; the handler converts.
type ImportSplitwisePreviewRow struct {
	Description  string
	IncurredAt   interface{} // time.Time, kept opaque to avoid extra import cycles in callers
	AmountCents  int64
	Currency     string
	CategorySlug string
	PayerCSVName string
}

// ImportSplitwiseSettlementPreview is one settlement that the importer will
// create from a "Payment" row. Mirrors the OpenAPI shape.
type ImportSplitwiseSettlementPreview struct {
	Note        string
	SettledAt   interface{} // time.Time, opaque to keep the handler's apigen import contained.
	AmountCents int64
	Currency    string
	FromCSVName string
	ToCSVName   string
}

// ImportSplitwiseBalance is one member's projected net balance for the
// imported group, derived from the expenses that will actually be created.
// Sign convention matches the dothesplit /balances endpoint: positive
// net_cents = the user is owed money, negative = the user owes money.
type ImportSplitwiseBalance struct {
	CSVName  string
	NetCents int64
}

// ImportSplitwiseResult is what the handler returns to the caller.
type ImportSplitwiseResult struct {
	GroupID           *uuid.UUID
	GroupName         string
	DefaultCurrency   string
	Members           []ImportSplitwiseMember
	ExpenseCount      int
	SettlementCount   int
	SkippedCount      int
	Skipped           []string
	Balances          []ImportSplitwiseBalance
	Preview           []ImportSplitwisePreviewRow
	SettlementPreview []ImportSplitwiseSettlementPreview
	// CSVCurrencies is the list of distinct ISO currency codes seen in the
	// CSV, in first-seen order. dothesplit groups are single-currency by
	// design, so the UI uses len(CSVCurrencies) > 1 to warn that amounts
	// from the original file will be imported under one currency and the
	// numeric values will no longer represent the source faithfully.
	CSVCurrencies []string
}

// SplitwiseImporter orchestrates the parser, the user/group/expense services,
// and the underlying repo. It is intentionally a thin glue layer: the parser
// is pure, group/expense creation reuses the same validation paths as the
// regular UI, and the new behavior is concentrated in the email-resolution
// step (FindOrCreateStub) so the security boundary is easy to audit.
type SplitwiseImporter struct {
	store       repo.Store
	users       repo.UserRepo
	groupRepo   repo.GroupRepo
	expenses    *ExpenseService
	categories  *CategoryService
	settlements repo.SettlementRepo
	auth        *AuthService
	email       *crypto.EmailCipher
}

func NewSplitwiseImporter(store repo.Store, users repo.UserRepo, groupRepo repo.GroupRepo, expenses *ExpenseService, categories *CategoryService, settlements repo.SettlementRepo, auth *AuthService, email *crypto.EmailCipher) *SplitwiseImporter {
	return &SplitwiseImporter{
		store: store, users: users, groupRepo: groupRepo,
		expenses: expenses, categories: categories, settlements: settlements,
		auth: auth, email: email,
	}
}

// Run validates the input, parses the CSV with the Splitwise parser,
// and either returns a preview (DryRun=true) or creates the group +
// expenses (DryRun=false).
func (s *SplitwiseImporter) Run(ctx context.Context, actorID uuid.UUID, in ImportSplitwiseInput) (ImportSplitwiseResult, error) {
	return s.run(ctx, actorID, in, csvimport.Parse)
}

// RunDoTheSplit is the same flow as Run but uses ParseDoTheSplit, the
// parser that understands the richer header dothesplit's own export
// emits (Time, Payer, Notes, Created, CreatedBy). The post-parse
// logic - email resolution, group/member creation, expense and
// settlement creation - is identical, so we share the inner method.
func (s *SplitwiseImporter) RunDoTheSplit(ctx context.Context, actorID uuid.UUID, in ImportSplitwiseInput) (ImportSplitwiseResult, error) {
	return s.run(ctx, actorID, in, csvimport.ParseDoTheSplit)
}

// run is the shared implementation: it validates the input, calls
// `parse` to read the CSV, then either previews or commits.
func (s *SplitwiseImporter) run(ctx context.Context, actorID uuid.UUID, in ImportSplitwiseInput, parse func(string) (csvimport.Result, error)) (ImportSplitwiseResult, error) {
	groupName := strings.TrimSpace(in.GroupName)
	if groupName == "" {
		return ImportSplitwiseResult{}, errors.New("group_name is required")
	}
	if len(groupName) > 80 {
		return ImportSplitwiseResult{}, errors.New("group_name too long")
	}
	cur, err := normalizeCurrency(in.DefaultCurrency)
	if err != nil {
		return ImportSplitwiseResult{}, err
	}
	if cur == "" {
		cur = DefaultGroupCurrency
	}
	in.DefaultCurrency = cur

	if len(in.Members) < csvimport.MinUsers || len(in.Members) > csvimport.MaxUsers {
		return ImportSplitwiseResult{}, fmt.Errorf("members must be between %d and %d", csvimport.MinUsers, csvimport.MaxUsers)
	}
	emails := make(map[string]struct{}, len(in.Members))
	for _, m := range in.Members {
		if strings.TrimSpace(m.CSVName) == "" {
			return ImportSplitwiseResult{}, errors.New("members[].csv_name is required")
		}
		email := strings.ToLower(strings.TrimSpace(m.Email))
		if email == "" {
			return ImportSplitwiseResult{}, errors.New("members[].email is required")
		}
		if _, dup := emails[email]; dup {
			return ImportSplitwiseResult{}, errors.New("members must have distinct emails")
		}
		emails[email] = struct{}{}
	}

	parsed, err := parse(in.CSV)
	if err != nil {
		return ImportSplitwiseResult{}, err
	}
	if len(parsed.UserNames) != len(in.Members) {
		return ImportSplitwiseResult{}, fmt.Errorf("members count (%d) does not match csv user columns (%d)", len(in.Members), len(parsed.UserNames))
	}

	// The CSV header is the source of truth for member names; client-supplied
	// names are only used to detect blank fields. Echo the real names back so
	// the UI can render labels like "Email for Julian Alarcon" instead of
	// whatever placeholder the dry-run sent.
	resolvedMembers := make([]ImportSplitwiseMember, len(in.Members))
	for i := range in.Members {
		resolvedMembers[i] = ImportSplitwiseMember{
			CSVName: parsed.UserNames[i],
			Email:   in.Members[i].Email,
		}
	}

	// Build the preview before any DB work; identical for dry_run and commit.
	pb := s.buildPreview(parsed, resolvedMembers, cur)
	skippedRaw := append([]string(nil), parsed.Skipped...)
	for _, raw := range pb.extraSkippedRaw {
		if len(skippedRaw) >= csvimport.MaxSkippedSamples {
			break
		}
		skippedRaw = append(skippedRaw, raw)
	}
	res := ImportSplitwiseResult{
		GroupName:         groupName,
		DefaultCurrency:   cur,
		Members:           resolvedMembers,
		ExpenseCount:      pb.expenseCount,
		SettlementCount:   pb.settlementCount,
		SkippedCount:      parsed.SkippedCount + pb.extraSkipped,
		Skipped:           skippedRaw,
		Balances:          pb.balances,
		Preview:           pb.expenses,
		SettlementPreview: pb.settlements,
		CSVCurrencies:     append([]string(nil), parsed.Currencies...),
	}
	if in.DryRun {
		return res, nil
	}

	// Everything below is the commit phase. It runs in a SINGLE transaction so
	// the import is all-or-nothing: a failure (including a cancelled request
	// context when the caller times out) rolls back the whole group instead of
	// leaving a partially-imported group behind. The membership reads in the
	// regular ExpenseService/SettlementService.Create paths can't see the rows
	// we insert in this uncommitted tx, so we deliberately bypass them and call
	// the tx-aware repo writers directly - the payer and split users are
	// members by construction (built from memberIDs created in this same tx).
	otherID, err := s.categories.DefaultID(ctx)
	if err != nil {
		return ImportSplitwiseResult{}, err
	}
	cats, err := s.categories.List(ctx)
	if err != nil {
		return ImportSplitwiseResult{}, err
	}
	byLowerLabel := func(lbl string) (string, bool) {
		for _, c := range cats {
			if strings.EqualFold(c.Label, lbl) {
				return c.ID.String(), true
			}
		}
		return "", false
	}

	tx, err := s.store.Begin(ctx)
	if err != nil {
		return ImportSplitwiseResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Resolve every member email to a user_id, creating non-loginable stubs
	// for any unknown address. The same return shape is used for "already
	// existed" and "just created" so the response can't be used to
	// enumerate the user table.
	memberIDs := make([]uuid.UUID, len(resolvedMembers))
	for i, m := range resolvedMembers {
		uid, err := s.resolveOrStub(ctx, tx, m)
		if err != nil {
			return ImportSplitwiseResult{}, err
		}
		memberIDs[i] = uid
	}
	// Distinct user-IDs are required for splits; duplicate stubs would also
	// indicate the caller passed the same email twice (we already check the
	// raw strings above, but the email_hash collision check protects against
	// case/Unicode normalisation surprises).
	idSeen := make(map[uuid.UUID]struct{}, len(memberIDs))
	for _, id := range memberIDs {
		if _, dup := idSeen[id]; dup {
			return ImportSplitwiseResult{}, errors.New("members must resolve to distinct users")
		}
		idSeen[id] = struct{}{}
	}

	// Create the group with the actor as creator (auto-added as member).
	g, err := s.groupRepo.CreateTx(ctx, tx, groupName, cur, actorID)
	if err != nil {
		return ImportSplitwiseResult{}, err
	}
	// Add each resolved member that isn't already the creator. We bypass
	// GroupService.AddMember because that path looks up by email and would
	// surface ErrInviteeNotFound, which is the enumeration oracle we are
	// explicitly avoiding.
	for _, uid := range memberIDs {
		if uid == actorID {
			continue
		}
		if err := s.groupRepo.AddMemberTx(ctx, tx, g.ID, uid); err != nil {
			return ImportSplitwiseResult{}, err
		}
	}

	// Loop expenses, mapping categories and translating signs to exact splits.
	for _, row := range parsed.Rows {
		// Prefer the second-precision IncurredAt the dothesplit
		// parser populates; fall back to the date-only Date for
		// Splitwise rows.
		when := row.IncurredAt
		if when.IsZero() {
			when = row.Date
		}
		if csvimport.IsPaymentRow(row) {
			st, ok := csvimport.DecomposeSettlement(row)
			if !ok {
				res.SkippedCount++
				continue
			}
			// We bypass SettlementService.Create here because that path
			// requires actor == from_user (the payer) - a sensible rule for
			// the regular UI but wrong for an import where the actor is just
			// the operator and any member could be the historical payer.
			settlement := &repo.Settlement{
				GroupID:  g.ID,
				FromUser: memberIDs[st.FromIdx],
				ToUser:   memberIDs[st.ToIdx],
				// Settlements have no currency column - they ride the
				// group's default. Nothing to do here besides the comment.
				AmountCents: st.AmountCents,
				Note:        st.Note,
				SettledAt:   when,
				// Optional CSV "Created" column pins the original creation time;
				// zero lets the DB/Go default apply.
				CreatedAt: row.Created,
			}
			// Optional CSV "CreatedBy" column: the settlement's creator lives on
			// the activity event, so pass the resolved member as the event actor.
			// Unknown/empty name falls back to the importing operator.
			settlementActor := actorID
			if idx := csvimport.PayerIdx(parsed.UserNames, row.CreatedByName); idx != -1 {
				settlementActor = memberIDs[idx]
			}
			if err := s.settlements.CreateTx(ctx, tx, settlement, settlementActor); err != nil {
				return ImportSplitwiseResult{}, err
			}
			continue
		}
		derived, ok := csvimport.Decompose(row)
		if !ok {
			res.SkippedCount++
			continue
		}
		catIDStr := csvimport.MapCategory(row.Category, row.Description, byLowerLabel, otherID.String())
		catUUID, err := uuid.Parse(catIDStr)
		if err != nil {
			return ImportSplitwiseResult{}, fmt.Errorf("internal: bad category id %q", catIDStr)
		}
		// If the row carries an explicit Payer (dothesplit format),
		// honor it: the sign-based inference picks one creditor when
		// there are several, but the explicit column is the
		// load-bearing source of truth for the original group state.
		explicitPayer := csvimport.PayerIdx(parsed.UserNames, row.PayerName)
		for _, e := range derived {
			// Data-quality guards that ExpenseService.Create would normally
			// enforce. We bypass that path (see above), so re-check the bits
			// resolveSplits doesn't cover and skip the row rather than abort
			// the whole import - same forgiving behaviour as Decompose's !ok.
			if e.AmountCents <= 0 || strings.TrimSpace(e.Description) == "" {
				res.SkippedCount++
				continue
			}
			payerIdx := e.PayerIdx
			if explicitPayer != -1 && len(derived) == 1 {
				payerIdx = explicitPayer
			}
			splits := make([]SplitInput, 0, len(e.Shares))
			for i, share := range e.Shares {
				if share == 0 {
					continue
				}
				splits = append(splits, SplitInput{UserID: memberIDs[i], Value: share})
			}
			shares, err := resolveSplits(SplitExact, e.AmountCents, splits)
			if err != nil {
				// A row whose shares don't sum to its amount is bad data, not
				// an infrastructure failure: skip it, keep the import going.
				res.SkippedCount++
				continue
			}
			incurred := when
			if incurred.IsZero() {
				incurred = defaultOccurredAt()
			}
			// Optional CSV "CreatedBy" column restores the original author;
			// unknown/empty name falls back to the importing operator.
			creator := actorID
			if idx := csvimport.PayerIdx(parsed.UserNames, row.CreatedByName); idx != -1 {
				creator = memberIDs[idx]
			}
			exp := &repo.Expense{
				GroupID:   g.ID,
				PayerID:   memberIDs[payerIdx],
				CreatedBy: creator,
				// Optional CSV "Created" column pins the original creation time.
				CreatedAt:  row.Created,
				CategoryID: catUUID,
				// Always the group's currency. dothesplit groups are
				// single-currency; for mixed-currency Splitwise CSVs we
				// surface a warning in the response (CSVCurrencies) but the
				// stored values still ride the chosen group currency. The
				// raw figures travel unchanged so a fully-settled group's
				// balances still project to zero.
				AmountCents: e.AmountCents,
				Currency:    cur,
				Description: e.Description,
				Notes:       row.Notes,
				IncurredAt:  incurred,
				Splits:      shares,
			}
			if err := s.expenses.CreateWithSplitsTx(ctx, tx, exp); err != nil {
				return ImportSplitwiseResult{}, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ImportSplitwiseResult{}, err
	}

	res.GroupID = &g.ID
	return res, nil
}

// resolveOrStub returns the user_id matching the email, creating a non-loginable
// placeholder when no active row exists. The display name for new stubs is
// the CSV name with a clear "(imported)" suffix so it's distinguishable from a
// self-chosen name.
func (s *SplitwiseImporter) resolveOrStub(ctx context.Context, tx repo.Tx, m ImportSplitwiseMember) (uuid.UUID, error) {
	email := strings.TrimSpace(m.Email)
	hash := s.email.HashEmail(email)
	enc, err := s.email.Encrypt(email)
	if err != nil {
		return uuid.Nil, err
	}
	pwd, err := s.auth.ScrambledPasswordHash()
	if err != nil {
		return uuid.Nil, err
	}
	display := strings.TrimSpace(m.CSVName) + " (imported)"
	u, err := s.users.FindOrCreateStub(ctx, tx, hash, enc, display, pwd)
	if err != nil {
		return uuid.Nil, err
	}
	return u.ID, nil
}

// previewBuild captures everything buildPreview returns. Plain struct so we
// don't grow the named-return list past readability.
type previewBuild struct {
	expenses        []ImportSplitwisePreviewRow
	settlements     []ImportSplitwiseSettlementPreview
	expenseCount    int
	settlementCount int
	extraSkipped    int
	extraSkippedRaw []string
	balances        []ImportSplitwiseBalance
}

// buildPreview formats the first PreviewLimit derived expenses + settlements
// for the response and reports the total valid count plus any rows the
// parser accepted but downstream decomposition rejected (e.g. all-zero or
// one-sided). The raw CSV text of those rejected rows is returned alongside
// so the UI can surface them next to the parser-level skipped lines. It
// also accumulates per-member projected net balances over the derived
// expenses AND settlements (positive = the member is owed, negative = the
// member owes), matching the dothesplit /balances endpoint convention. With
// settlements included the balances of a fully-settled Splitwise group
// project to zero, exactly like the trailing CSV "Total balance" footer.
//
// The preview rows always carry the user-chosen group currency, not the
// per-row CSV currency: dothesplit groups are single-currency, so this is
// also what the committed expenses/settlements will end up with. The
// per-row CSV currencies are surfaced separately as Result.CSVCurrencies
// so the UI can warn about mixed-currency imports.
func (s *SplitwiseImporter) buildPreview(parsed csvimport.Result, members []ImportSplitwiseMember, groupCurrency string) previewBuild {
	out := previewBuild{
		expenses:    make([]ImportSplitwisePreviewRow, 0, PreviewLimit),
		settlements: make([]ImportSplitwiseSettlementPreview, 0, PreviewLimit),
	}
	netCents := make([]int64, len(members))
	for _, row := range parsed.Rows {
		when := row.IncurredAt
		if when.IsZero() {
			when = row.Date
		}
		if csvimport.IsPaymentRow(row) {
			st, ok := csvimport.DecomposeSettlement(row)
			if !ok {
				out.extraSkipped++
				out.extraSkippedRaw = append(out.extraSkippedRaw, row.Raw)
				continue
			}
			out.settlementCount++
			// Settlements move balance one-for-one: from_user paid (their
			// "owes" goes down -> balance up), to_user received (balance
			// down). Same sign convention as the per-row Splitwise values.
			netCents[st.FromIdx] += st.AmountCents
			netCents[st.ToIdx] -= st.AmountCents
			if len(out.settlements) < PreviewLimit {
				out.settlements = append(out.settlements, ImportSplitwiseSettlementPreview{
					Note:        st.Note,
					SettledAt:   when,
					AmountCents: st.AmountCents,
					Currency:    groupCurrency,
					FromCSVName: members[st.FromIdx].CSVName,
					ToCSVName:   members[st.ToIdx].CSVName,
				})
			}
			continue
		}
		derived, ok := csvimport.Decompose(row)
		if !ok {
			out.extraSkipped++
			out.extraSkippedRaw = append(out.extraSkippedRaw, row.Raw)
			continue
		}
		explicitPayer := csvimport.PayerIdx(parsed.UserNames, row.PayerName)
		for _, e := range derived {
			payerIdx := e.PayerIdx
			if explicitPayer != -1 && len(derived) == 1 {
				payerIdx = explicitPayer
			}
			out.expenseCount++
			// Each derived expense contributes paid - share to its members'
			// balances. Positive = creditor (the payer net of their own
			// share); negative = debtor (their share, since they paid 0).
			for i, share := range e.Shares {
				netCents[i] -= share
			}
			netCents[payerIdx] += e.AmountCents
			if len(out.expenses) < PreviewLimit {
				out.expenses = append(out.expenses, ImportSplitwisePreviewRow{
					Description:  e.Description,
					IncurredAt:   when,
					AmountCents:  e.AmountCents,
					Currency:     groupCurrency,
					CategorySlug: row.Category,
					PayerCSVName: members[payerIdx].CSVName,
				})
			}
		}
	}
	out.balances = make([]ImportSplitwiseBalance, len(members))
	for i, m := range members {
		out.balances[i] = ImportSplitwiseBalance{CSVName: m.CSVName, NetCents: netCents[i]}
	}
	return out
}
