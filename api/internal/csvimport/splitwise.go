// Package csvimport contains pure parsers for third-party expense exports.
// No DB, no HTTP. The service layer composes these parsers with the repos.
package csvimport

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxCSVBytes is the upper bound on raw input size. 256 KiB is comfortably
// above realistic Splitwise exports (multi-year history for a small group
// is ~80 KiB) and small enough to keep memory pressure trivial.
const MaxCSVBytes = 256 * 1024

// MaxRows caps the parsed record count to prevent pathological inputs.
const MaxRows = 5000

// MaxFieldLen caps individual cell length. A real Splitwise description is
// rarely over 80 chars, so 256 is generous.
const MaxFieldLen = 256

// MinUsers / MaxUsers bound the number of per-user columns. Splitwise itself
// caps groups well below 32, so this is a sanity ceiling rather than a real
// product limit.
const (
	MinUsers = 2
	MaxUsers = 32
)

// Sentinel parse errors. Service layer maps these to HTTP 400.
var (
	ErrCSVTooLarge  = errors.New("csv exceeds size limit")
	ErrCSVBadHeader = errors.New("csv header does not match Splitwise export format")
	ErrCSVNoRows    = errors.New("csv contains no usable expense rows")
	ErrCSVTooMany   = errors.New("csv exceeds row limit")
	ErrCSVFieldLen  = errors.New("csv field exceeds length limit")
)

// expectedHeader is the fixed prefix; columns after these are either
// user-name columns or part of dothesplitExtraColumns (a small whitelist
// of optional metadata columns that dothesplit's own export emits, see
// csvimport/dothesplit.go). The Splitwise parser silently skips them so
// a dothesplit-shaped CSV can also be re-imported through this endpoint.
var expectedHeader = []string{"Date", "Description", "Category", "Cost", "Currency"}

// dothesplitExtraColumns lists the header names the dothesplit exporter
// inserts between Currency and the per-member block. Order does not
// matter; the parser consumes any contiguous run of these names after
// the mandatory prefix and treats the rest of the columns as users.
// Matched case-insensitively.
var dothesplitExtraColumns = map[string]struct{}{
	"time":      {},
	"payer":     {},
	"notes":     {},
	"created":   {},
	"createdby": {},
}

// Row is a successfully parsed expense row. The mandatory fields
// (Date, Description, Category, Cost, Currency, SignedCents) come from
// the Splitwise format. The optional fields are only populated by
// ParseDoTheSplit, which reads the richer dothesplit-flavored header;
// the legacy Parse leaves them zero/empty.
type Row struct {
	Date        time.Time
	Description string
	Category    string
	CostCents   int64
	Currency    string
	// SignedCents holds the per-user signed cents value as it appeared in
	// the CSV, indexed by user-column position (matching Result.UserNames).
	SignedCents []int64
	// Raw is the original CSV record rejoined with commas. Surfaced back to
	// the importer UI when downstream logic (e.g. Decompose) rejects the row.
	Raw string

	// IncurredAt is the full second-precision timestamp for the row when
	// the source carried a Time column (dothesplit format). Zero when
	// only a date was present; callers should fall back to Date.
	IncurredAt time.Time
	// PayerName, when non-empty, names the explicit payer for the row
	// (matches one of Result.UserNames). Used to bypass the
	// sign-based payer inference for ambiguous rows.
	PayerName string
	// Notes is the dothesplit-only "Notes" column. Empty when the source
	// did not carry one (Splitwise has no equivalent).
	Notes string
}

// Result is the full parse outcome.
type Result struct {
	UserNames    []string
	Rows         []Row
	SkippedCount int
	// Skipped holds the raw CSV record (rejoined with commas) for each row
	// the parser dropped: bad date, wrong column count, unparseable cost,
	// etc. Blank lines and the trailing "Total balance" row are intentionally
	// excluded - those are noise rather than user data the importer needs to
	// surface. Capped at MaxSkippedSamples entries so a pathological input
	// can't bloat the response.
	Skipped []string
	// Currencies lists the distinct ISO codes seen across accepted rows,
	// in first-seen order. dothesplit groups are single-currency by design,
	// so a result with len(Currencies) > 1 is the importer's signal to warn
	// the user that the CSV mixes currencies and the converted amounts will
	// not represent the original values faithfully.
	Currencies []string
}

// MaxSkippedSamples caps how many raw skipped lines we surface back to the
// caller. Beyond this we still increment SkippedCount but stop appending.
const MaxSkippedSamples = 50

// SplitwisePaymentCategory marks rows that Splitwise emits when a member
// hands cash over to another to settle a balance, rather than incurring a
// shared expense. We translate these to dothesplit settlements rather than
// expenses (see DecomposeSettlement). The string is the literal category
// label Splitwise writes; matched case-insensitively.
const SplitwisePaymentCategory = "Payment"

// IsPaymentRow reports whether a parsed row is a Splitwise "Payment" entry,
// i.e. a settlement, not an expense. Caller decides which Decompose* path
// to use.
func IsPaymentRow(row Row) bool {
	return strings.EqualFold(strings.TrimSpace(row.Category), SplitwisePaymentCategory)
}

// DerivedSettlement is one dothesplit settlement produced from a Splitwise
// "Payment" row. The signed-cents convention is the same as for expenses:
// the user with the positive value is the one whose balance went up
// (i.e. they handed money to another member to settle a debt) and is the
// `from_user`; the negative value is the recipient and the `to_user`.
type DerivedSettlement struct {
	Note        string
	AmountCents int64
	FromIdx     int
	ToIdx       int
}

// DecomposeSettlement translates a "Payment" row into one DerivedSettlement.
// A well-formed Splitwise payment row has exactly one positive value (payer)
// and exactly one negative value (recipient); every other user is zero.
// Returns ok=false if the row has multiple payers or recipients, all-zeros,
// asymmetric magnitudes, or a cost that doesn't match the moved amount.
func DecomposeSettlement(row Row) (DerivedSettlement, bool) {
	n := len(row.SignedCents)
	if n < MinUsers || row.CostCents <= 0 {
		return DerivedSettlement{}, false
	}
	posIdx, negIdx := -1, -1
	for i, v := range row.SignedCents {
		switch {
		case v > 0:
			if posIdx != -1 {
				return DerivedSettlement{}, false
			}
			posIdx = i
		case v < 0:
			if negIdx != -1 {
				return DerivedSettlement{}, false
			}
			negIdx = i
		}
	}
	if posIdx == -1 || negIdx == -1 {
		return DerivedSettlement{}, false
	}
	amount := row.SignedCents[posIdx]
	if -row.SignedCents[negIdx] != amount {
		return DerivedSettlement{}, false
	}
	// Splitwise stamps `Cost` with the same magnitude as the moved amount,
	// so reject rows where it disagrees - that's a sign the row is something
	// other than a clean two-party transfer.
	if row.CostCents != amount {
		return DerivedSettlement{}, false
	}
	return DerivedSettlement{
		Note:        row.Description,
		AmountCents: amount,
		FromIdx:     posIdx,
		ToIdx:       negIdx,
	}, true
}

// DerivedExpense is one dothesplit expense produced from a CSV row. A row
// with multiple creditors produces multiple DerivedExpenses (see Decompose);
// a single-creditor row produces exactly one.
type DerivedExpense struct {
	Description string
	AmountCents int64
	// PayerIdx is the user-column index of the payer (matching SignedCents).
	PayerIdx int
	// Shares is keyed by user-column index. Length == number of users.
	// Entries sum to AmountCents.
	Shares []int64
}

// Parse converts a Splitwise N-person CSV export into a Result. Malformed
// rows are skipped silently and counted in SkippedCount; only structural
// problems (size, header, etc.) return an error.
func Parse(raw string) (Result, error) {
	if len(raw) > MaxCSVBytes {
		return Result{}, ErrCSVTooLarge
	}

	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1 // we'll enforce width ourselves so blank lines don't crash
	r.TrimLeadingSpace = false

	header, err := readNonBlank(r)
	if err != nil {
		return Result{}, ErrCSVBadHeader
	}
	if len(header) < len(expectedHeader)+MinUsers {
		return Result{}, ErrCSVBadHeader
	}
	for i, want := range expectedHeader {
		if header[i] != want {
			return Result{}, ErrCSVBadHeader
		}
	}
	// Skip any contiguous run of dothesplit-only optional columns
	// directly after the mandatory header prefix. Anything else from
	// that point on is a user name.
	extrasEnd := len(expectedHeader)
	for extrasEnd < len(header) {
		key := strings.ToLower(strings.TrimSpace(header[extrasEnd]))
		if _, ok := dothesplitExtraColumns[key]; !ok {
			break
		}
		extrasEnd++
	}
	if len(header)-extrasEnd < MinUsers || len(header)-extrasEnd > MaxUsers {
		return Result{}, ErrCSVBadHeader
	}
	userNames := make([]string, 0, len(header)-extrasEnd)
	seen := make(map[string]struct{}, len(header))
	for _, raw := range header[extrasEnd:] {
		name := strings.TrimSpace(raw)
		if name == "" {
			return Result{}, ErrCSVBadHeader
		}
		if _, dup := seen[name]; dup {
			return Result{}, ErrCSVBadHeader
		}
		seen[name] = struct{}{}
		userNames = append(userNames, name)
	}
	userColStart := extrasEnd

	res := Result{UserNames: userNames}
	width := len(header)

	recordSkip := func(rec []string) {
		res.SkippedCount++
		if len(res.Skipped) < MaxSkippedSamples {
			res.Skipped = append(res.Skipped, strings.Join(rec, ","))
		}
	}

	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			res.SkippedCount++
			continue
		}
		if isBlankRecord(rec) {
			continue
		}
		if len(rec) != width {
			recordSkip(rec)
			continue
		}
		if isTotalBalanceRow(rec) {
			continue
		}
		for _, f := range rec {
			if utf8.RuneCountInString(f) > MaxFieldLen {
				return Result{}, ErrCSVFieldLen
			}
		}
		row, ok := parseRow(rec, len(userNames), userColStart)
		if !ok {
			recordSkip(rec)
			continue
		}
		row.Raw = strings.Join(rec, ",")
		res.Rows = append(res.Rows, row)
		if !containsString(res.Currencies, row.Currency) {
			res.Currencies = append(res.Currencies, row.Currency)
		}
		if len(res.Rows) > MaxRows {
			return Result{}, ErrCSVTooMany
		}
	}

	if len(res.Rows) == 0 {
		return Result{}, ErrCSVNoRows
	}
	return res, nil
}

func readNonBlank(r *csv.Reader) ([]string, error) {
	for {
		rec, err := r.Read()
		if err != nil {
			return nil, err
		}
		if !isBlankRecord(rec) {
			return rec, nil
		}
	}
}

func isBlankRecord(rec []string) bool {
	for _, f := range rec {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}

func isTotalBalanceRow(rec []string) bool {
	// Splitwise appends "<date_or_blank>,Total balance, , ,<currency>,<a>,<b>,..."
	// at the foot of every export. Older exports leave Date empty; newer
	// exports stamp it with the export date. The Description column is the
	// stable signal: matching on it (case-insensitive) catches both shapes.
	return len(rec) > 1 && strings.EqualFold(strings.TrimSpace(rec[1]), "Total balance")
}

func parseRow(rec []string, n, userColStart int) (Row, bool) {
	dateStr := strings.TrimSpace(rec[0])
	desc := strings.TrimSpace(rec[1])
	cat := strings.TrimSpace(rec[2])
	costStr := strings.TrimSpace(rec[3])
	curStr := strings.TrimSpace(rec[4])

	if desc == "" || costStr == "" {
		return Row{}, false
	}
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return Row{}, false
	}
	cost, ok := parseDecimalToCents(costStr)
	if !ok || cost <= 0 {
		return Row{}, false
	}
	if len(curStr) != 3 {
		return Row{}, false
	}

	values := make([]int64, n)
	for i := 0; i < n; i++ {
		v, ok := parseDecimalToCents(strings.TrimSpace(rec[userColStart+i]))
		if !ok {
			return Row{}, false
		}
		values[i] = v
	}

	return Row{
		Date:        d,
		Description: desc,
		Category:    cat,
		CostCents:   cost,
		Currency:    strings.ToUpper(curStr),
		SignedCents: values,
	}, true
}

// parseDecimalToCents accepts an optionally-signed decimal with up to 2
// fractional digits ("12", "12.3", "12.34", "-12.34") and returns the value
// in integer cents. Rejects any other shape.
func parseDecimalToCents(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	neg := false
	switch s[0] {
	case '-':
		neg = true
		s = s[1:]
	case '+':
		s = s[1:]
	}
	if s == "" {
		return 0, false
	}
	intPart, fracPart, hasDot := strings.Cut(s, ".")
	if intPart == "" {
		return 0, false
	}
	for _, c := range intPart {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	if hasDot {
		if len(fracPart) == 0 || len(fracPart) > 2 {
			return 0, false
		}
		for _, c := range fracPart {
			if c < '0' || c > '9' {
				return 0, false
			}
		}
		if len(fracPart) == 1 {
			fracPart += "0"
		}
	} else {
		fracPart = "00"
	}
	whole, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, false
	}
	cents, err := strconv.ParseInt(fracPart, 10, 64)
	if err != nil {
		return 0, false
	}
	if whole > (math.MaxInt64-99)/100 {
		return 0, false
	}
	v := whole*100 + cents
	if neg {
		v = -v
	}
	return v, true
}

// Decompose translates a Splitwise row into one or more dothesplit expenses.
// Splitwise stores per-user signed cents that represent each user's net
// balance change for the row, matching the convention of the trailing
// "Total balance" footer:
//
//	value[i] = paid[i] - share[i]
//	positive = creditor (paid more than they owe, is owed money)
//	negative = debtor (owe more than they paid)
//
// A row is well-formed when sum(value) == 0 (within rounding tolerance) and
// at least one user is on each side. Decompose handles two cases:
//
//   - Single creditor: emits ONE expense at the full row cost. The creditor
//     is the payer; their share = cost - value[creditor]; every other user's
//     share = -value[i] (their owed cents); uninvolved (zero) users get 0.
//     Total split sums to cost. This preserves cost fidelity for any row
//     where only one person fronted the money, regardless of group size.
//
//   - Multiple creditors: emits ONE expense per creditor, each amount =
//     value[creditor]. The creditor's portion is split among the debtors
//     proportionally to |value[debtor]|. The originating description gets a
//     "[k/K]" suffix so the imported group is still browsable. Total imported
//     amount equals sum(creditor values), which can be less than the row's
//     nominal cost (the difference is the share each creditor "paid for
//     themselves" - dothesplit can't represent that under a single-payer
//     model so we drop it from the displayed amount while preserving every
//     user's net balance).
//
// Returns ok=false when the row is structurally invalid (all-zero values,
// same-sign, sum too far from zero, no debtors, no creditors).
func Decompose(row Row) ([]DerivedExpense, bool) {
	n := len(row.SignedCents)
	if n < MinUsers || row.CostCents <= 0 {
		return nil, false
	}

	values := make([]int64, n)
	copy(values, row.SignedCents)

	// Reject all-zero rows (no balance change).
	allZero := true
	for _, v := range values {
		if v != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return nil, false
	}

	// Tolerate up to N cents of rounding drift in the per-user values
	// (Splitwise rounds each side independently to two decimal places).
	// Snap by absorbing the discrepancy into the largest-magnitude creditor.
	sum := int64(0)
	for _, v := range values {
		sum += v
	}
	if abs64(sum) > int64(n) {
		return nil, false
	}
	if sum != 0 {
		biggestCred := -1
		for i, v := range values {
			if v > 0 && (biggestCred == -1 || v > values[biggestCred]) {
				biggestCred = i
			}
		}
		if biggestCred == -1 {
			return nil, false
		}
		values[biggestCred] -= sum
	}

	creditors := make([]int, 0, n)
	debtors := make([]int, 0, n)
	for i, v := range values {
		switch {
		case v > 0:
			creditors = append(creditors, i)
		case v < 0:
			debtors = append(debtors, i)
		}
	}
	if len(creditors) == 0 || len(debtors) == 0 {
		return nil, false
	}

	if len(creditors) == 1 {
		c := creditors[0]
		shares := make([]int64, n)
		for i, v := range values {
			if i == c {
				shares[i] = row.CostCents - v
			} else if v < 0 {
				shares[i] = -v
			} // zero-value users keep share 0
		}
		// Defensive checks: every share must be non-negative and they must
		// sum to the row cost. If the cost+value math goes weird (only
		// possible with crafted CSVs), reject the row.
		var total int64
		for _, s := range shares {
			if s < 0 {
				return nil, false
			}
			total += s
		}
		if total != row.CostCents {
			return nil, false
		}
		return []DerivedExpense{{
			Description: row.Description,
			AmountCents: row.CostCents,
			PayerIdx:    c,
			Shares:      shares,
		}}, true
	}

	// Multi-creditor: emit one expense per creditor.
	out := make([]DerivedExpense, 0, len(creditors))
	totalDebt := int64(0)
	for _, d := range debtors {
		totalDebt += -values[d]
	}
	for k, c := range creditors {
		amount := values[c]
		shares := distributeProportionally(amount, values, debtors, totalDebt)
		// Sanity: ensure shares sum to amount.
		var total int64
		for _, s := range shares {
			total += s
		}
		if total != amount {
			return nil, false
		}
		desc := row.Description
		if len(creditors) > 1 {
			desc = fmt.Sprintf("%s [%d/%d]", row.Description, k+1, len(creditors))
		}
		out = append(out, DerivedExpense{
			Description: desc,
			AmountCents: amount,
			PayerIdx:    c,
			Shares:      shares,
		})
	}
	return out, true
}

// distributeProportionally divides `amount` cents among the debtors,
// weighting each debtor by their positive value over `totalDebt`. Rounding
// remainder is allocated to the largest debtor to preserve the cent-level
// invariant `sum(out) == amount`.
func distributeProportionally(amount int64, values []int64, debtors []int, totalDebt int64) []int64 {
	out := make([]int64, len(values))
	if totalDebt <= 0 || amount <= 0 || len(debtors) == 0 {
		return out
	}
	var allocated int64
	largest := debtors[0]
	for _, d := range debtors {
		debt := -values[d]
		share := debt * amount / totalDebt
		out[d] = share
		allocated += share
		if debt > -values[largest] {
			largest = d
		}
	}
	if rem := amount - allocated; rem != 0 {
		out[largest] += rem
	}
	return out
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// MapCategory resolves a Splitwise category string to a dothesplit category
// label (lowercased), which the caller's resolver looks up to get the UUID.
// Falls back to "other" when no match is found.
//
// Two Splitwise categories conflate multiple dothesplit categories and need
// the description to disambiguate:
//
//   - "TV/Phone/Internet" -> "TV", "Phone", or "Internet" depending on the
//     description (default "Internet").
//   - "Bus/train" -> "Train" if the description mentions train, otherwise
//     "Bus".
//
// The description is matched on whole-word-ish substrings (case-insensitive).
func MapCategory(splitwiseCategory, description string, byLowerLabel func(string) (catID string, ok bool), otherID string) string {
	key := strings.ToLower(strings.TrimSpace(splitwiseCategory))
	desc := strings.ToLower(description)

	switch key {
	case "tv/phone/internet":
		// Order matters - "tv" is a 2-char token so we check it first to
		// avoid e.g. "phone" inside a longer word.
		for _, hit := range []struct{ needle, label string }{
			{"tv", "tv"},
			{"phone", "phone"},
			{"internet", "internet"},
		} {
			if containsWord(desc, hit.needle) {
				if id, ok := byLowerLabel(hit.label); ok {
					return id
				}
			}
		}
		if id, ok := byLowerLabel("internet"); ok {
			return id
		}
		return otherID
	case "bus/train":
		if containsWord(desc, "train") {
			if id, ok := byLowerLabel("train"); ok {
				return id
			}
		}
		if id, ok := byLowerLabel("bus"); ok {
			return id
		}
		return otherID
	}

	if id, ok := byLowerLabel(key); ok {
		return id
	}
	if alias, ok := splitwiseAliases[key]; ok {
		if id, ok := byLowerLabel(alias); ok {
			return id
		}
	}
	return otherID
}

// containsWord reports whether needle appears anywhere in haystack as a
// case-insensitive substring. Both inputs must already be lowercase. We
// deliberately accept partial matches like "iPhone" -> "phone" since these
// heuristics only fire when the source category is already narrow
// (TV/Phone/Internet, Bus/train), and the cost of a false match within
// those categories is at most a category that's still close to right.
func containsWord(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	return strings.Contains(haystack, needle)
}

// splitwiseAliases bridges Splitwise labels that map 1:1 to a dothesplit
// label with a different name. The key is the lowercased Splitwise label;
// the value is the lowercased dothesplit label that the resolver lookups
// against. Splitwise's "...-Other" labels map to dothesplit's generic
// "Other"; the standalone "General" Splitwise category does the same.
var splitwiseAliases = map[string]string{
	"entertainment - other":  "other",
	"food and drink - other": "snacks",
	"home - other":           "other",
	"life - other":           "other",
	"transportation - other": "other",
	"utilities - other":      "other",
	"general":                "other",
	"heat/gas":               "heating / gas",
	"gas/fuel":               "gas / fuel",
}
