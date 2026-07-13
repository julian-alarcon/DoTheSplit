package csvimport

import (
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// GroupExpenseRow is one row to be appended to an existing group. Fields
// other than Date, Description and CostCents are best-effort: empty cells
// are kept zero/empty and the service layer fills in defaults from the
// group (currency, payer, category).
type GroupExpenseRow struct {
	Date        time.Time
	IncurredAt  time.Time
	Description string
	Category    string
	CostCents   int64
	Currency    string
	PayerName   string
	Notes       string
	// Created is the original creation timestamp from the optional
	// "Created" column (RFC3339). Zero when absent or unparseable.
	Created time.Time
	// CreatedByName is the original creator's display name from the
	// optional "CreatedBy" column. Empty when absent.
	CreatedByName string
	Raw           string
}

// GroupExpenseResult is the full parse outcome for ParseGroupExpenses.
type GroupExpenseResult struct {
	Rows         []GroupExpenseRow
	Skipped      []string
	SkippedCount int
	Currencies   []string
}

// ParseGroupExpenses reads a DoTheSplit-shaped CSV intended for a single
// existing group. The mandatory header prefix is the same as
// ParseDoTheSplit (`Date,Description,Category,Cost,Currency`); the
// optional middle block (`Time, Payer, Notes, Created, CreatedBy`) is
// recognised in any order; trailing per-member columns are accepted but
// ignored - splits come from the group, not the CSV.
//
// Per-row validation is intentionally looser than the group-creation
// imports: only Date (parseable), Description (non-empty) and Cost (> 0)
// are required. Currency, category, payer and notes default to empty when
// missing; the service layer applies the group's fallbacks.
func ParseGroupExpenses(raw string) (GroupExpenseResult, error) {
	if len(raw) > MaxCSVBytes {
		return GroupExpenseResult{}, ErrCSVTooLarge
	}
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = false

	header, err := readNonBlank(r)
	if err != nil {
		return GroupExpenseResult{}, ErrCSVBadHeader
	}
	if len(header) < len(dothesplitMandatoryHeader) {
		return GroupExpenseResult{}, ErrCSVBadHeader
	}
	for i, want := range dothesplitMandatoryHeader {
		if header[i] != want {
			return GroupExpenseResult{}, ErrCSVBadHeader
		}
	}
	colIdx := map[string]int{
		"time": -1, "payer": -1, "notes": -1, "created": -1, "createdby": -1,
	}
	extrasEnd := len(dothesplitMandatoryHeader)
	for extrasEnd < len(header) {
		key := strings.ToLower(strings.TrimSpace(header[extrasEnd]))
		if _, ok := dothesplitOptionalColumns[key]; !ok {
			break
		}
		if colIdx[key] != -1 {
			return GroupExpenseResult{}, ErrCSVBadHeader
		}
		colIdx[key] = extrasEnd
		extrasEnd++
	}
	// Anything after extrasEnd is silently ignored: it's either a
	// per-member column from a full DoTheSplit export, or noise.

	res := GroupExpenseResult{}
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
				return GroupExpenseResult{}, ErrCSVFieldLen
			}
		}
		row, ok := parseGroupExpenseRow(rec, colIdx)
		if !ok {
			recordSkip(rec)
			continue
		}
		row.Raw = strings.Join(rec, ",")
		res.Rows = append(res.Rows, row)
		if row.Currency != "" && !containsString(res.Currencies, row.Currency) {
			res.Currencies = append(res.Currencies, row.Currency)
		}
		if len(res.Rows) > MaxRows {
			return GroupExpenseResult{}, ErrCSVTooMany
		}
	}

	if len(res.Rows) == 0 {
		return GroupExpenseResult{}, ErrCSVNoRows
	}
	return res, nil
}

func parseGroupExpenseRow(rec []string, colIdx map[string]int) (GroupExpenseRow, bool) {
	dateStr := strings.TrimSpace(rec[0])
	desc := strings.TrimSpace(rec[1])
	cat := strings.TrimSpace(rec[2])
	costStr := strings.TrimSpace(rec[3])
	curStr := strings.TrimSpace(rec[4])

	if desc == "" || costStr == "" {
		return GroupExpenseRow{}, false
	}
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return GroupExpenseRow{}, false
	}
	cost, ok := parseDecimalToCents(costStr)
	if !ok || cost <= 0 {
		return GroupExpenseRow{}, false
	}
	currency := ""
	if curStr != "" {
		if len(curStr) != 3 {
			return GroupExpenseRow{}, false
		}
		currency = strings.ToUpper(curStr)
	}

	row := GroupExpenseRow{
		Date:        d,
		Description: desc,
		Category:    cat,
		CostCents:   cost,
		Currency:    currency,
	}
	if i := colIdx["time"]; i != -1 {
		if t := strings.TrimSpace(rec[i]); t != "" {
			if parsed, err := parseTimeOfDay(t); err == nil {
				row.IncurredAt = mergeDateAndTime(row.Date, parsed)
			}
		}
	}
	if row.IncurredAt.IsZero() {
		row.IncurredAt = row.Date
	}
	if i := colIdx["payer"]; i != -1 {
		row.PayerName = strings.TrimSpace(rec[i])
	}
	if i := colIdx["notes"]; i != -1 {
		row.Notes = strings.TrimSpace(rec[i])
	}
	if i := colIdx["created"]; i != -1 {
		if c := strings.TrimSpace(rec[i]); c != "" {
			if parsed, err := time.Parse(time.RFC3339, c); err == nil {
				row.Created = parsed.UTC()
			}
		}
	}
	if i := colIdx["createdby"]; i != -1 {
		row.CreatedByName = strings.TrimSpace(rec[i])
	}
	return row, true
}
