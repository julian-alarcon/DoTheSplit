package csvimport

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseGroupExpenses_Minimal(t *testing.T) {
	in := "Date,Description,Category,Cost,Currency\n" +
		"2026-06-01,Pizza,,42.00,\n"
	res, err := ParseGroupExpenses(in)
	if err != nil {
		t.Fatalf("ParseGroupExpenses: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	r := res.Rows[0]
	if r.Description != "Pizza" || r.CostCents != 4200 {
		t.Errorf("row = %+v", r)
	}
	if r.Currency != "" {
		t.Errorf("Currency = %q, want empty", r.Currency)
	}
	if r.Category != "" || r.PayerName != "" || r.Notes != "" {
		t.Errorf("expected empty optional fields, got %+v", r)
	}
	wantDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !r.Date.Equal(wantDate) || !r.IncurredAt.Equal(wantDate) {
		t.Errorf("date/incurred mismatch: %v / %v", r.Date, r.IncurredAt)
	}
}

func TestParseGroupExpenses_FullRow(t *testing.T) {
	in := "Date,Description,Category,Cost,Currency,Time,Payer,Notes\n" +
		"2026-06-01,Pizza,Food,42.00,EUR,19:30:00,Alice,Friday night\n"
	res, err := ParseGroupExpenses(in)
	if err != nil {
		t.Fatalf("ParseGroupExpenses: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	r := res.Rows[0]
	if r.Currency != "EUR" || r.Category != "Food" || r.PayerName != "Alice" || r.Notes != "Friday night" {
		t.Errorf("row = %+v", r)
	}
	wantTime := time.Date(2026, 6, 1, 19, 30, 0, 0, time.UTC)
	if !r.IncurredAt.Equal(wantTime) {
		t.Errorf("IncurredAt = %v, want %v", r.IncurredAt, wantTime)
	}
	if got := res.Currencies; len(got) != 1 || got[0] != "EUR" {
		t.Errorf("Currencies = %v", got)
	}
}

func TestParseGroupExpenses_TrailingMemberColumnsIgnored(t *testing.T) {
	// A full DoTheSplit export should be accepted, with the per-member
	// columns silently ignored.
	in := "Date,Description,Category,Cost,Currency,Time,Payer,Notes,Created,CreatedBy,Alice,Bob\n" +
		"2026-06-01,Pizza,Food,42.00,EUR,19:30:00,Alice,,,,21.00,-21.00\n"
	res, err := ParseGroupExpenses(in)
	if err != nil {
		t.Fatalf("ParseGroupExpenses: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	r := res.Rows[0]
	if r.PayerName != "Alice" || r.CostCents != 4200 {
		t.Errorf("row = %+v", r)
	}
}

func TestParseGroupExpenses_CreatedColumns(t *testing.T) {
	// Created/CreatedBy present with values are parsed; a re-import can then
	// restore the original creator/timestamp.
	in := "Date,Description,Category,Cost,Currency,Created,CreatedBy,Alice,Bob\n" +
		"2026-06-01,Pizza,Food,42.00,EUR,2026-06-01T19:31:07Z,Alice,21.00,-21.00\n"
	res, err := ParseGroupExpenses(in)
	if err != nil {
		t.Fatalf("ParseGroupExpenses: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	r := res.Rows[0]
	wantCreated := time.Date(2026, 6, 1, 19, 31, 7, 0, time.UTC)
	if !r.Created.Equal(wantCreated) {
		t.Errorf("Created = %v, want %v", r.Created, wantCreated)
	}
	if r.CreatedByName != "Alice" {
		t.Errorf("CreatedByName = %q, want Alice", r.CreatedByName)
	}
}

func TestParseGroupExpenses_CreatedColumnsAbsent(t *testing.T) {
	// Without the columns, Created/CreatedByName stay zero/empty so the
	// importer falls back to the current time and the importing actor.
	in := "Date,Description,Category,Cost,Currency,Time,Payer,Notes\n" +
		"2026-06-01,Pizza,Food,42.00,EUR,19:30:00,Alice,Friday night\n"
	res, err := ParseGroupExpenses(in)
	if err != nil {
		t.Fatalf("ParseGroupExpenses: %v", err)
	}
	r := res.Rows[0]
	if !r.Created.IsZero() || r.CreatedByName != "" {
		t.Errorf("expected zero Created/empty CreatedByName, got %v / %q", r.Created, r.CreatedByName)
	}
}

func TestParseGroupExpenses_SkipsMalformed(t *testing.T) {
	in := "Date,Description,Category,Cost,Currency\n" +
		",no date,,1.00,EUR\n" +
		"2026-06-01,,Food,1.00,EUR\n" +
		"2026-06-01,bad cost,Food,abc,EUR\n" +
		"2026-06-01,zero cost,Food,0.00,EUR\n" +
		"2026-06-01,ok,Food,1.00,EUR\n"
	res, err := ParseGroupExpenses(in)
	if err != nil {
		t.Fatalf("ParseGroupExpenses: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	if res.SkippedCount != 4 {
		t.Errorf("skipped = %d, want 4", res.SkippedCount)
	}
}

func TestParseGroupExpenses_BadCurrencyShape(t *testing.T) {
	in := "Date,Description,Category,Cost,Currency\n" +
		"2026-06-01,Pizza,,1.00,EU\n"
	res, err := ParseGroupExpenses(in)
	if !errors.Is(err, ErrCSVNoRows) {
		t.Errorf("err = %v, want ErrCSVNoRows (the bad row should have been skipped)", err)
	}
	_ = res
}

func TestParseGroupExpenses_BadHeader(t *testing.T) {
	cases := []string{
		"foo,bar\n",
		"Date,Description,Category,Cost\n",
		"Date,Description,WrongCol,Cost,Currency\n",
	}
	for _, c := range cases {
		if _, err := ParseGroupExpenses(c); !errors.Is(err, ErrCSVBadHeader) {
			t.Errorf("input=%q err = %v, want ErrCSVBadHeader", c, err)
		}
	}
}

func TestParseGroupExpenses_NoRows(t *testing.T) {
	in := "Date,Description,Category,Cost,Currency\n"
	if _, err := ParseGroupExpenses(in); !errors.Is(err, ErrCSVNoRows) {
		t.Errorf("err = %v, want ErrCSVNoRows", err)
	}
}

func TestParseGroupExpenses_TooLarge(t *testing.T) {
	big := "Date,Description,Category,Cost,Currency\n" + strings.Repeat("x", MaxCSVBytes)
	if _, err := ParseGroupExpenses(big); !errors.Is(err, ErrCSVTooLarge) {
		t.Errorf("err = %v, want ErrCSVTooLarge", err)
	}
}

func TestParseGroupExpenses_MultipleCurrencies(t *testing.T) {
	in := "Date,Description,Category,Cost,Currency\n" +
		"2026-06-01,a,,1.00,EUR\n" +
		"2026-06-02,b,,1.00,USD\n" +
		"2026-06-03,c,,1.00,EUR\n"
	res, err := ParseGroupExpenses(in)
	if err != nil {
		t.Fatalf("ParseGroupExpenses: %v", err)
	}
	if got := res.Currencies; len(got) != 2 || got[0] != "EUR" || got[1] != "USD" {
		t.Errorf("Currencies = %v", got)
	}
}
