package csvimport

import (
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// dothesplitMandatoryHeader is the prefix every dothesplit CSV must
// have; further columns are either named optional fields or user
// columns. The Splitwise importer also accepts these (and silently
// skips the optional ones - see expectedHeader / dothesplitExtraColumns
// in splitwise.go).
var dothesplitMandatoryHeader = []string{"Date", "Description", "Category", "Cost", "Currency"}

// dothesplitOptionalColumns lists the named extras the parser knows
// how to consume. A header may include any subset; missing columns
// degrade gracefully.
var dothesplitOptionalColumns = map[string]struct{}{
	"time":      {},
	"payer":     {},
	"notes":     {},
	"created":   {},
	"createdby": {},
}

// ParseDoTheSplit reads a dothesplit-flavored CSV: the mandatory
// Splitwise prefix, then an optional run of named metadata columns,
// then the per-member columns. Otherwise it shares all behavior with
// Parse (size limits, blank-row handling, "Total balance" skip).
func ParseDoTheSplit(raw string) (Result, error) {
	if len(raw) > MaxCSVBytes {
		return Result{}, ErrCSVTooLarge
	}
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = false

	header, err := readNonBlank(r)
	if err != nil {
		return Result{}, ErrCSVBadHeader
	}
	if len(header) < len(dothesplitMandatoryHeader)+MinUsers {
		return Result{}, ErrCSVBadHeader
	}
	for i, want := range dothesplitMandatoryHeader {
		if header[i] != want {
			return Result{}, ErrCSVBadHeader
		}
	}
	// Track the column index of each known optional column. -1 means
	// "not present"; the row reader uses these offsets to populate
	// optional Row fields.
	colIdx := map[string]int{
		"time": -1, "payer": -1, "notes": -1, "created": -1, "createdby": -1,
	}
	extrasEnd := len(dothesplitMandatoryHeader)
	for extrasEnd < len(header) {
		key := strings.ToLower(strings.TrimSpace(header[extrasEnd]))
		if _, ok := dothesplitOptionalColumns[key]; !ok {
			break
		}
		if _, dup := colIdx[key]; dup && colIdx[key] != -1 {
			return Result{}, ErrCSVBadHeader
		}
		colIdx[key] = extrasEnd
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
		row, ok := parseRow(rec, len(userNames), extrasEnd)
		if !ok {
			recordSkip(rec)
			continue
		}
		// Populate the dothesplit-only optional fields from their
		// header positions, when present.
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

// parseTimeOfDay accepts "HH:MM:SS", "HH:MM:SSZ", or RFC3339 time-only
// formats and returns the parsed time as a time.Time on the zero date,
// in UTC.
func parseTimeOfDay(s string) (time.Time, error) {
	for _, layout := range []string{"15:04:05Z", "15:04:05", "15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, errors.New("unrecognized time format")
}

func mergeDateAndTime(date, tod time.Time) time.Time {
	return time.Date(
		date.Year(), date.Month(), date.Day(),
		tod.Hour(), tod.Minute(), tod.Second(), 0,
		time.UTC,
	)
}

// PayerIdx returns the index of payerName in userNames, or -1 if not
// found. Convenience helper for callers that received an explicit
// Row.PayerName.
func PayerIdx(userNames []string, payerName string) int {
	if payerName == "" {
		return -1
	}
	for i, n := range userNames {
		if n == payerName {
			return i
		}
	}
	return -1
}
