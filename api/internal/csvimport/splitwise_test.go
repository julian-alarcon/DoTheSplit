package csvimport

import (
	"errors"
	"strings"
	"testing"
)

const goodHeader = "Date,Description,Category,Cost,Currency,Alice,Bob\n"

func TestParse_TracksCurrencies(t *testing.T) {
	in := goodHeader +
		"2024-01-01,Coffee,Dining out,4.00,EUR,-2.00,2.00\n" +
		"2024-01-02,Hotel,Hotel,200.00,USD,100.00,-100.00\n" +
		"2024-01-03,Brunch,Dining out,30.00,EUR,15.00,-15.00\n"
	res, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []string{"EUR", "USD"}
	if !equalStrings(res.Currencies, want) {
		t.Errorf("Currencies = %v, want %v (first-seen order, distinct)", res.Currencies, want)
	}
}

func TestParse_SingleCurrencyOnly(t *testing.T) {
	in := goodHeader +
		"2024-01-01,Coffee,Dining out,4.00,EUR,-2.00,2.00\n"
	res, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !equalStrings(res.Currencies, []string{"EUR"}) {
		t.Errorf("Currencies = %v, want [EUR]", res.Currencies)
	}
}

func TestParse_HappyRows_TwoUsers(t *testing.T) {
	// Splitwise convention: positive value = creditor (paid more than share);
	// negative value = debtor. Row 0: Alice paid 4.00, both share 2.00, Alice
	// is +2.00, Bob is -2.00. Row 1: Bob paid 200, both share 100.
	in := goodHeader +
		"2024-01-01,Coffee,Dining out,4.00,EUR,2.00,-2.00\n" +
		"2024-01-02,Hotel,Hotel,200.00,EUR,-100.00,100.00\n"

	res, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := res.UserNames, []string{"Alice", "Bob"}; !equalStrings(got, want) {
		t.Errorf("user names = %v, want %v", got, want)
	}
	if got, want := len(res.Rows), 2; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	if got, want := res.Rows[0].CostCents, int64(400); got != want {
		t.Errorf("row0 cost = %d, want %d", got, want)
	}
	if got, want := res.Rows[1].SignedCents, []int64{-10000, 10000}; !equalInts(got, want) {
		t.Errorf("row1 signed = %v, want %v", got, want)
	}
}

func TestParse_HappyRows_ThreeUsers(t *testing.T) {
	in := "Date,Description,Category,Cost,Currency,Alice,Bob,Carol\n" +
		"2024-01-01,Test1,Parking,60.00,EUR,40.00,-20.00,-20.00\n" +
		"2024-01-03,FinalP,Childcare,90.00,EUR,22.50,45.00,-67.50\n"

	res, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := res.UserNames, []string{"Alice", "Bob", "Carol"}; !equalStrings(got, want) {
		t.Errorf("user names = %v, want %v", got, want)
	}
	if got, want := len(res.Rows), 2; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	if got, want := res.Rows[0].SignedCents, []int64{4000, -2000, -2000}; !equalInts(got, want) {
		t.Errorf("row0 signed = %v, want %v", got, want)
	}
}

func TestParse_BadHeader(t *testing.T) {
	cases := map[string]string{
		"missing user columns": "Date,Description,Category,Cost,Currency,Alice\n",
		"reordered":            "Description,Date,Category,Cost,Currency,Alice,Bob\n",
		"renamed first col":    "Day,Description,Category,Cost,Currency,Alice,Bob\n",
		"same user twice":      "Date,Description,Category,Cost,Currency,Alice,Alice\n",
		"blank user column":    "Date,Description,Category,Cost,Currency,Alice,\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(raw); !errors.Is(err, ErrCSVBadHeader) {
				t.Errorf("err = %v, want ErrCSVBadHeader", err)
			}
		})
	}
}

func TestParse_TooLarge(t *testing.T) {
	in := goodHeader + strings.Repeat("a", MaxCSVBytes)
	if _, err := Parse(in); !errors.Is(err, ErrCSVTooLarge) {
		t.Errorf("err = %v, want ErrCSVTooLarge", err)
	}
}

func TestParse_NoUsableRows(t *testing.T) {
	in := goodHeader + "\n,Total balance, , ,EUR,0.00,-0.00\n"
	if _, err := Parse(in); !errors.Is(err, ErrCSVNoRows) {
		t.Errorf("err = %v, want ErrCSVNoRows", err)
	}
}

func TestParse_SkipsBadAndTotalRows(t *testing.T) {
	in := goodHeader +
		"\n" +
		"2024-01-01,Good,Dining out,4.00,EUR,2.00,-2.00\n" +
		"bad-date,BadDate,Dining out,5.00,EUR,2.50,-2.50\n" +
		",Total balance, , ,EUR,2.00,-2.00\n"
	res, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Errorf("rows = %d, want 1 (only Good row survives; bad-date skipped, total skipped)", len(res.Rows))
	}
	if res.SkippedCount == 0 {
		t.Errorf("expected skipped > 0 for bad-date")
	}
	for _, r := range res.Rows {
		if r.Description == "BadDate" {
			t.Errorf("BadDate should have been skipped")
		}
	}
}

func TestParse_SkipsDoTheSplitExtraColumns(t *testing.T) {
	// dothesplit's own export adds metadata columns between Currency and
	// the per-member block. The Splitwise parser must skip them so a
	// dothesplit CSV remains importable through this endpoint.
	header := "Date,Description,Category,Cost,Currency,Time,Payer,Notes,Created,CreatedBy,Alice,Bob\n"
	in := header +
		"2024-01-01,Coffee,Dining out,4.00,EUR,12:00:00Z,Alice,extra shot,2024-01-01T12:00:00Z,Alice,2.00,-2.00\n" +
		"2024-01-02,Hotel,Hotel,200.00,EUR,09:00:00Z,Bob,,2024-01-02T09:00:00Z,Bob,-100.00,100.00\n"
	res, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := res.UserNames, []string{"Alice", "Bob"}; !equalStrings(got, want) {
		t.Errorf("user names = %v, want %v", got, want)
	}
	if got, want := len(res.Rows), 2; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	if got, want := res.Rows[0].SignedCents, []int64{200, -200}; !equalInts(got, want) {
		t.Errorf("row0 signed = %v, want %v", got, want)
	}
	if got, want := res.Rows[1].CostCents, int64(20000); got != want {
		t.Errorf("row1 cost = %d, want %d", got, want)
	}
}

func TestDecompose_TwoUsers_EvenSplit(t *testing.T) {
	// Alice paid 10, both share 5 -> Alice +5 (creditor), Bob -5 (debtor).
	row := Row{
		CostCents:   1000,
		Description: "Coffee",
		SignedCents: []int64{500, -500},
	}
	exps, ok := Decompose(row)
	if !ok || len(exps) != 1 {
		t.Fatalf("ok=%v len=%d", ok, len(exps))
	}
	got := exps[0]
	if got.PayerIdx != 0 || got.AmountCents != 1000 {
		t.Errorf("payer=%d amount=%d, want 0 / 1000", got.PayerIdx, got.AmountCents)
	}
	if !equalInts(got.Shares, []int64{500, 500}) {
		t.Errorf("shares = %v, want [500 500]", got.Shares)
	}
}

func TestDecompose_TwoUsers_LoanRow(t *testing.T) {
	// Alice paid 200, Bob owes the full 200. Balance: alice +200, bob -200.
	row := Row{
		CostCents:   200,
		Description: "Loan",
		SignedCents: []int64{200, -200},
	}
	exps, ok := Decompose(row)
	if !ok || len(exps) != 1 {
		t.Fatalf("ok=%v len=%d", ok, len(exps))
	}
	got := exps[0]
	if got.PayerIdx != 0 || got.AmountCents != 200 {
		t.Errorf("payer=%d amount=%d", got.PayerIdx, got.AmountCents)
	}
	if !equalInts(got.Shares, []int64{0, 200}) {
		t.Errorf("shares = %v, want [0 200]", got.Shares)
	}
}

func TestDecompose_ThreeUsers_SingleCreditor(t *testing.T) {
	// Alice paid 90; everyone owes a share. Balance: alice = 90 - 22.50 =
	// +67.50; bob = -45; carol = -22.50. Sum = 0.
	row := Row{
		CostCents:   9000,
		Description: "Brunch",
		SignedCents: []int64{6750, -4500, -2250},
	}
	exps, ok := Decompose(row)
	if !ok || len(exps) != 1 {
		t.Fatalf("ok=%v len=%d", ok, len(exps))
	}
	got := exps[0]
	if got.PayerIdx != 0 || got.AmountCents != 9000 {
		t.Errorf("payer=%d amount=%d", got.PayerIdx, got.AmountCents)
	}
	if !equalInts(got.Shares, []int64{2250, 4500, 2250}) {
		t.Errorf("shares = %v, want [2250 4500 2250]", got.Shares)
	}
}

func TestDecompose_ThreeUsers_MultipleCreditors(t *testing.T) {
	// trip-to-tokio sample row: cost=60, J=+40 (creditor; paid 60, owes 20),
	// D=-20 (debtor), M=-20 (debtor). Wait, two debtors here means it's
	// actually single-creditor. Use a true multi-creditor row instead:
	// cost=60, J=-40 (debtor), D=+20 (creditor; paid 30, owes 10),
	// M=+20 (creditor; paid 30, owes 10). Two creditors -> two expenses.
	row := Row{
		CostCents:   6000,
		Description: "Test1",
		SignedCents: []int64{-4000, 2000, 2000},
	}
	exps, ok := Decompose(row)
	if !ok {
		t.Fatalf("ok=%v", ok)
	}
	if len(exps) != 2 {
		t.Fatalf("expected 2 expenses, got %d", len(exps))
	}
	// Both expenses cover 20 each, total 40 (Julian's 40 owed); the other
	// 20 of the 60 cost is each creditor's own self-share which has no
	// net-balance impact and is dropped.
	for k, e := range exps {
		if e.AmountCents != 2000 {
			t.Errorf("exp[%d] amount=%d, want 2000", k, e.AmountCents)
		}
		if e.Shares[0] != 2000 {
			t.Errorf("exp[%d] shares[0]=%d, want 2000 (julian owes full amount)", k, e.Shares[0])
		}
	}
	if exps[0].PayerIdx == exps[1].PayerIdx {
		t.Errorf("two creditors should have different payer indices")
	}
	if !strings.Contains(exps[0].Description, "[1/2]") || !strings.Contains(exps[1].Description, "[2/2]") {
		t.Errorf("descriptions should be suffixed: %q %q", exps[0].Description, exps[1].Description)
	}
}

func TestDecompose_ThreeUsers_MultipleCreditorsAndDebtors(t *testing.T) {
	// FinalP-style row: cost=90, J=+22.50 (creditor), D=+45 (creditor),
	// M=-67.50 (debtor). Two creditors (J,D), one debtor (M). Each creditor's
	// expense has M owing their full amount.
	row := Row{
		CostCents:   9000,
		Description: "FinalP",
		SignedCents: []int64{2250, 4500, -6750},
	}
	exps, ok := Decompose(row)
	if !ok {
		t.Fatalf("ok=%v", ok)
	}
	if len(exps) != 2 {
		t.Fatalf("expected 2 expenses, got %d", len(exps))
	}
	// Total imported = 22.50 + 45 = 67.50; cost was 90 (M's self-share of
	// 22.50 dropped).
	var total int64
	for _, e := range exps {
		total += e.AmountCents
	}
	if total != 6750 {
		t.Errorf("sum amounts = %d, want 6750", total)
	}
}

func TestDecompose_RejectsBadRows(t *testing.T) {
	cases := map[string]Row{
		"all zero":       {CostCents: 1000, SignedCents: []int64{0, 0, 0}},
		"only debtors":   {CostCents: 1000, SignedCents: []int64{-500, -500, 0}},
		"only creditors": {CostCents: 1000, SignedCents: []int64{500, 500, 0}},
		"sum way off":    {CostCents: 1000, SignedCents: []int64{100, -50, 0}},
	}
	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := Decompose(row); ok {
				t.Errorf("expected ok=false")
			}
		})
	}
}

func TestIsPaymentRow(t *testing.T) {
	if !IsPaymentRow(Row{Category: "Payment"}) {
		t.Errorf("Payment should match")
	}
	if !IsPaymentRow(Row{Category: " payment "}) {
		t.Errorf("case + whitespace should match")
	}
	if IsPaymentRow(Row{Category: "Dining out"}) {
		t.Errorf("non-payment must not match")
	}
}

func TestDecomposeSettlement_HappyPath(t *testing.T) {
	// vacuna-5g row 25: "Fernanda D. paid Nathaly V.", cost 1123148.5;
	// Nathaly = -1123148.5 (recipient), Fernanda = +1123148.5 (payer).
	row := Row{
		Description: "Fernanda D. paid Nathaly V.",
		Category:    "Payment",
		CostCents:   112314850,
		Currency:    "COP",
		SignedCents: []int64{-112314850, 112314850, 0, 0},
	}
	st, ok := DecomposeSettlement(row)
	if !ok {
		t.Fatalf("ok=false")
	}
	if st.FromIdx != 1 || st.ToIdx != 0 {
		t.Errorf("from=%d to=%d, want from=1 to=0", st.FromIdx, st.ToIdx)
	}
	if st.AmountCents != 112314850 {
		t.Errorf("amount=%d", st.AmountCents)
	}
	if st.Note != "Fernanda D. paid Nathaly V." {
		t.Errorf("note=%q", st.Note)
	}
}

func TestDecomposeSettlement_RejectsBadShape(t *testing.T) {
	cases := map[string]Row{
		"all zero":            {CostCents: 100, SignedCents: []int64{0, 0, 0, 0}},
		"two payers":          {CostCents: 100, SignedCents: []int64{50, 50, -100, 0}},
		"two recipients":      {CostCents: 100, SignedCents: []int64{100, -50, -50, 0}},
		"asymmetric":          {CostCents: 100, SignedCents: []int64{100, -50, 0, 0}},
		"cost mismatch":       {CostCents: 200, SignedCents: []int64{100, -100, 0, 0}},
		"single column user":  {CostCents: 100, SignedCents: []int64{100}},
	}
	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := DecomposeSettlement(row); ok {
				t.Errorf("expected ok=false for %q", name)
			}
		})
	}
}

func TestVacuna5G_BalancesProjectToZero(t *testing.T) {
	// Subset of vacuna-5g_2026-06-06_export.csv with one expense + the
	// Payment that should cancel it. Walking expenses + settlements, the
	// net balances must project to zero, matching the CSV's "Total balance"
	// footer of 0,0,0,0.
	header := "Date,Description,Category,Cost,Currency,Nathaly,Fernanda,Julian,Milena\n"
	in := header +
		// Taxi: Julian paid 25000, all four split evenly. Julian net +18750
		// (the "paid 25k, owes 6250" residue); each of the others net -6250.
		"2021-06-05,Taxi Al hotel,Taxi,25000,COP,-6250,-6250,18750,-6250\n" +
		// Three settlements that exactly close out everyone's debt to Julian.
		// Sign convention: payer is positive (balance went up), recipient is
		// negative.
		"2021-06-05,Nathaly V. paid Julian A.,Payment,6250,COP,6250,0,-6250,0\n" +
		"2021-06-05,Fernanda D. paid Julian A.,Payment,6250,COP,0,6250,-6250,0\n" +
		"2021-06-05,Milena S. paid Julian A.,Payment,6250,COP,0,0,-6250,6250\n"
	res, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	net := []int64{0, 0, 0, 0}
	for _, row := range res.Rows {
		if IsPaymentRow(row) {
			st, ok := DecomposeSettlement(row)
			if !ok {
				t.Fatalf("DecomposeSettlement %q: not ok", row.Description)
			}
			net[st.FromIdx] += st.AmountCents
			net[st.ToIdx] -= st.AmountCents
			continue
		}
		exps, ok := Decompose(row)
		if !ok {
			t.Fatalf("Decompose %q: not ok", row.Description)
		}
		for _, e := range exps {
			for i, s := range e.Shares {
				net[i] -= s
			}
			net[e.PayerIdx] += e.AmountCents
		}
	}
	for i, v := range net {
		if v != 0 {
			t.Errorf("net[%d]=%d, want 0 after expense + matching payment", i, v)
		}
	}
}

func TestDecompose_RoundingTolerance(t *testing.T) {
	// Sum is +1 cent; should snap into the largest creditor and still produce
	// a valid expense.
	row := Row{
		CostCents:   2999,
		Description: "Odd",
		SignedCents: []int64{1500, -1499},
	}
	exps, ok := Decompose(row)
	if !ok || len(exps) != 1 {
		t.Fatalf("ok=%v len=%d", ok, len(exps))
	}
	if exps[0].Shares[0]+exps[0].Shares[1] != 2999 {
		t.Errorf("shares don't sum to cost: %v", exps[0].Shares)
	}
}

func TestDecompose_TripToTokio_BalancesMatchCSV(t *testing.T) {
	// End-to-end balance check against the trailing "Total balance" row of
	// trip-to-tokio_2026-06-06_export.csv. After running every row through
	// Decompose and accumulating paid - share per member, the totals must
	// match the CSV footer exactly: J=+20.50, D=-66.50, M=+46.00.
	rows := []Row{
		{CostCents: 6000, Description: "Test1", SignedCents: []int64{4000, -2000, -2000}},
		{CostCents: 300, Description: "Otro", SignedCents: []int64{300, -150, -150}},
		{CostCents: 9000, Description: "FinalP", SignedCents: []int64{-2250, -4500, 6750}},
	}
	net := []int64{0, 0, 0}
	for _, row := range rows {
		exps, ok := Decompose(row)
		if !ok {
			t.Fatalf("Decompose %q: not ok", row.Description)
		}
		for _, e := range exps {
			for i, s := range e.Shares {
				net[i] -= s
			}
			net[e.PayerIdx] += e.AmountCents
		}
	}
	want := []int64{2050, -6650, 4600}
	if !equalInts(net, want) {
		t.Errorf("net = %v, want %v (Splitwise Total balance row)", net, want)
	}
}

func TestMapCategory(t *testing.T) {
	resolver := func(seedLabels map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) {
			id, ok := seedLabels[k]
			return id, ok
		}
	}
	// Mirrors the seeded dothesplit labels we actually need for these tests
	// (lowercased, since the resolver is keyed that way).
	seed := map[string]string{
		"groceries":  "id-groc",
		"dining out": "id-din",
		"internet":   "id-net",
		"phone":      "id-phone",
		"tv":         "id-tv",
		"bus":        "id-bus",
		"train":      "id-train",
		"other":      "id-other",
	}
	cases := []struct {
		name        string
		category    string
		description string
		want        string
	}{
		{"Groceries plain", "Groceries", "Kaufland", "id-groc"},
		{"Dining out plain", "Dining out", "Sushi", "id-din"},
		{"Entertainment - Other -> other", "Entertainment - Other", "Cinema", "id-other"},
		{"Life - Other -> other", "Life - Other", "Whatever", "id-other"},
		{"Home - Other -> other", "Home - Other", "Mailbox", "id-other"},
		{"Transportation - Other -> other", "Transportation - Other", "Uber", "id-other"},
		{"Utilities - Other -> other", "Utilities - Other", "Sewage", "id-other"},
		{"Food and drinks - Other -> other", "Food and drinks - Other", "Vending", "id-other"},
		{"General -> other", "General", "Cash withdrawal", "id-other"},
		{"TV/Phone/Internet w/ Internet -> internet", "TV/Phone/Internet", "Vodafone Internet", "id-net"},
		{"TV/Phone/Internet w/ Phone -> phone", "TV/Phone/Internet", "iPhone bill", "id-phone"},
		{"TV/Phone/Internet w/ TV -> tv", "TV/Phone/Internet", "TV-Steuer", "id-tv"},
		{"TV/Phone/Internet default -> internet", "TV/Phone/Internet", "Cable subscription", "id-net"},
		{"Bus/train w/ Train -> train", "Bus/train", "DB Train Berlin", "id-train"},
		{"Bus/train default -> bus", "Bus/train", "Flixbus to Munich", "id-bus"},
		{"Unknown -> other", "Definitely Not A Category", "x", "id-other"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MapCategory(c.category, c.description, resolver(seed), "id-other")
			if got != c.want {
				t.Errorf("MapCategory(%q, %q) = %s, want %s", c.category, c.description, got, c.want)
			}
		})
	}
}

func TestParseDecimalToCents(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"0", 0, true},
		{"0.00", 0, true},
		{"1", 100, true},
		{"1.5", 150, true},
		{"1.50", 150, true},
		{"-1.50", -150, true},
		{"+1.50", 150, true},
		{"123.45", 12345, true},
		{".50", 0, false},
		{"1.", 0, false},
		{"1.555", 0, false},
		{"abc", 0, false},
		{"", 0, false},
		{"-", 0, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, ok := parseDecimalToCents(c.in)
			if ok != c.ok || (ok && got != c.want) {
				t.Errorf("parseDecimalToCents(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
			}
		})
	}
}

// equalStrings / equalInts are tiny helpers since we don't use slices.Equal here.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalInts(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
