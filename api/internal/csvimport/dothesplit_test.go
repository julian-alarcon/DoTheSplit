package csvimport

import (
	"errors"
	"testing"
	"time"
)

func TestParseDoTheSplit_FullHeader(t *testing.T) {
	in := "Date,Description,Category,Cost,Currency,Time,Payer,Notes,Created,CreatedBy,Alice,Bob\n" +
		"2024-01-01,Coffee,Dining out,4.00,EUR,18:42:00Z,Alice,extra shot,2024-01-01T18:43:10Z,Alice,2.00,-2.00\n" +
		"2024-01-02,Reimburse,Payment,30.00,EUR,09:15:00Z,Bob,,2024-01-02T09:15:42Z,Bob,30.00,-30.00\n"
	res, err := ParseDoTheSplit(in)
	if err != nil {
		t.Fatalf("ParseDoTheSplit: %v", err)
	}
	if got, want := res.UserNames, []string{"Alice", "Bob"}; !equalStrings(got, want) {
		t.Errorf("user names = %v, want %v", got, want)
	}
	if got, want := len(res.Rows), 2; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	r0 := res.Rows[0]
	wantTime := time.Date(2024, 1, 1, 18, 42, 0, 0, time.UTC)
	if !r0.IncurredAt.Equal(wantTime) {
		t.Errorf("row0 IncurredAt = %v, want %v", r0.IncurredAt, wantTime)
	}
	if r0.PayerName != "Alice" {
		t.Errorf("row0 PayerName = %q, want Alice", r0.PayerName)
	}
	if r0.Notes != "extra shot" {
		t.Errorf("row0 Notes = %q, want extra shot", r0.Notes)
	}
	if r0.SignedCents[0] != 200 || r0.SignedCents[1] != -200 {
		t.Errorf("row0 SignedCents = %v", r0.SignedCents)
	}
}

func TestParseDoTheSplit_OptionalColumnsMissing(t *testing.T) {
	// A bare Splitwise-shaped CSV must still parse through the
	// dothesplit endpoint, falling back to no-time / no-payer / no-notes.
	in := "Date,Description,Category,Cost,Currency,Alice,Bob\n" +
		"2024-01-01,Coffee,Dining out,4.00,EUR,2.00,-2.00\n"
	res, err := ParseDoTheSplit(in)
	if err != nil {
		t.Fatalf("ParseDoTheSplit: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	r := res.Rows[0]
	if r.PayerName != "" || r.Notes != "" {
		t.Errorf("expected empty PayerName/Notes when columns absent, got %q / %q", r.PayerName, r.Notes)
	}
	// IncurredAt falls back to the date at midnight UTC.
	wantTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if !r.IncurredAt.Equal(wantTime) {
		t.Errorf("row IncurredAt = %v, want %v", r.IncurredAt, wantTime)
	}
}

func TestParseDoTheSplit_PartialOptionalColumns(t *testing.T) {
	// Only Time and Notes; Payer/Created/CreatedBy missing.
	in := "Date,Description,Category,Cost,Currency,Time,Notes,Alice,Bob\n" +
		"2024-01-01,Coffee,Dining out,4.00,EUR,12:00:00Z,with cinnamon,2.00,-2.00\n"
	res, err := ParseDoTheSplit(in)
	if err != nil {
		t.Fatalf("ParseDoTheSplit: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	r := res.Rows[0]
	if r.Notes != "with cinnamon" {
		t.Errorf("Notes = %q", r.Notes)
	}
	if r.PayerName != "" {
		t.Errorf("PayerName should be empty when column absent, got %q", r.PayerName)
	}
}

func TestParseDoTheSplit_BadHeader(t *testing.T) {
	cases := map[string]string{
		"missing user columns": "Date,Description,Category,Cost,Currency,Alice\n",
		"bad mandatory order":  "Description,Date,Category,Cost,Currency,Alice,Bob\n",
		"same user twice":      "Date,Description,Category,Cost,Currency,Time,Alice,Alice\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseDoTheSplit(raw); !errors.Is(err, ErrCSVBadHeader) {
				t.Errorf("err = %v, want ErrCSVBadHeader", err)
			}
		})
	}
}

func TestPayerIdx(t *testing.T) {
	names := []string{"Alice", "Bob", "Carol"}
	if got := PayerIdx(names, "Bob"); got != 1 {
		t.Errorf("PayerIdx(Bob) = %d, want 1", got)
	}
	if got := PayerIdx(names, ""); got != -1 {
		t.Errorf("PayerIdx empty = %d, want -1", got)
	}
	if got := PayerIdx(names, "Dave"); got != -1 {
		t.Errorf("PayerIdx unknown = %d, want -1", got)
	}
}
