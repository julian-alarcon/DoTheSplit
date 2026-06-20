package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const splitwiseHeader = "Date,Description,Category,Cost,Currency,Alice,Bob\n"

func TestImportSplitwise_DryRunAndCommit(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	// Alice is the caller; Bob will be created as a stub during import.
	_, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")

	csv := splitwiseHeader +
		"2024-01-01,Coffee,Dining out,4.00,EUR,-2.00,2.00\n" + // Bob paid (Bob creditor: positive)
		"2024-01-02,Hotel,Hotel,200.00,EUR,100.00,-100.00\n" + // Alice paid
		"2024-01-03,Cat litter,Pets,10.00,EUR,-5.00,5.00\n" + // Bob paid
		"2024-01-04,Bad row,General,10.00,EUR,5.00,5.00\n" + // same sign — skipped downstream
		",Total balance, , ,EUR,93.00,-93.00\n"

	body := map[string]any{
		"csv":              csv,
		"group_name":       "Prost",
		"default_currency": "EUR",
		"members": []map[string]any{
			{"csv_name": "Alice", "email": "alice@example.com"},
			{"csv_name": "Bob", "email": "bob-stub@example.com"},
		},
		"dry_run": true,
	}

	// --- Dry run ---
	resp, out := request(t, "POST", base+"/v1/imports/splitwise", body, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode, out)
	require.Equal(t, "Prost", out["group_name"])
	require.Equal(t, "EUR", out["default_currency"])
	require.EqualValues(t, 3, out["expense_count"])
	require.EqualValues(t, 1, out["skipped_count"])
	preview, _ := out["preview"].([]any)
	require.Len(t, preview, 3)
	first, _ := preview[0].(map[string]any)
	require.Equal(t, "Coffee", first["description"])
	require.EqualValues(t, 400, first["amount_cents"])
	require.Equal(t, "Bob", first["payer_csv_name"])
	require.Nil(t, out["group_id"], "dry_run must not create a group")

	// --- Commit ---
	body["dry_run"] = false
	resp, out = request(t, "POST", base+"/v1/imports/splitwise", body, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, out)
	gid, _ := out["group_id"].(string)
	require.NotEmpty(t, gid)

	// Read back the group; Alice must be a member; expenses must total 3.
	resp, exps := requestList(t, "GET", base+"/v1/groups/"+gid+"/expenses", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, exps, 3)
}

func TestImportSplitwise_ThreeMembers(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL
	_, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")

	// Three rows mirroring the trip-to-tokio test fixture:
	// 1. Single-creditor (Alice paid for everyone): 1 expense.
	// 2. Single-creditor on a different user (Bob paid): 1 expense.
	// 3. Multi-creditor (Bob and Carol both paid): 2 expenses with [k/K] suffix.
	csv := "Date,Description,Category,Cost,Currency,Alice,Bob,Carol\n" +
		"2024-01-01,Brunch,Dining out,90.00,EUR,67.50,-45.00,-22.50\n" + // alice paid 90 (creditor: positive)
		"2024-01-02,Otro,General,3.00,EUR,-3.00,1.50,1.50\n" + // bob+carol paid (multi-creditor)
		"2024-01-03,FinalP,Childcare,90.00,EUR,22.50,45.00,-67.50\n" // alice+bob paid (multi-creditor)

	body := map[string]any{
		"csv":              csv,
		"group_name":       "Trip",
		"default_currency": "EUR",
		"members": []map[string]any{
			{"csv_name": "Alice", "email": "alice@example.com"},
			{"csv_name": "Bob", "email": "bob@example.com"},
			{"csv_name": "Carol", "email": "carol@example.com"},
		},
		"dry_run": true,
	}
	resp, out := request(t, "POST", base+"/v1/imports/splitwise", body, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode, out)
	// Row 1 -> 1 expense; row 2 -> 2 expenses; row 3 -> 2 expenses. Total 5.
	require.EqualValues(t, 5, out["expense_count"])
	preview, _ := out["preview"].([]any)
	require.Len(t, preview, 5)

	// Commit and verify the group has the three members + 5 expenses.
	body["dry_run"] = false
	resp, out = request(t, "POST", base+"/v1/imports/splitwise", body, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, out)
	gid, _ := out["group_id"].(string)
	require.NotEmpty(t, gid)

	resp, exps := requestList(t, "GET", base+"/v1/groups/"+gid+"/expenses", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, exps, 5)

	// The group listing reports 3 members and the imported group is reachable.
	resp, all := requestList(t, "GET", base+"/v1/groups", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var found bool
	for _, g := range all {
		if g["id"] == gid {
			found = true
			members, _ := g["members"].([]any)
			require.Len(t, members, 3)
		}
	}
	require.True(t, found, "imported group should appear in /v1/groups")

	// At least one expense should have the [k/K] suffix from the multi-creditor decomposition.
	var sawSuffix bool
	for _, e := range exps {
		desc, _ := e["description"].(string)
		if strings.Contains(desc, "[1/2]") || strings.Contains(desc, "[2/2]") {
			sawSuffix = true
		}
	}
	require.True(t, sawSuffix, "expected a [k/K]-suffixed description from multi-creditor decomposition")
}

func TestImportSplitwise_MixedCurrenciesAreSurfaced(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL
	_, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")

	// Two rows, two different currencies. dothesplit groups are
	// single-currency: the response must list both ISO codes in
	// `csv_currencies` so the UI can warn, and the preview rows must carry
	// the chosen group currency (not the per-row CSV currency) since that
	// is what the committed rows will end up with.
	csv := splitwiseHeader +
		"2024-01-01,Coffee,Dining out,4.00,EUR,-2.00,2.00\n" +
		"2024-01-02,Hotel,Hotel,200.00,USD,100.00,-100.00\n"
	body := map[string]any{
		"csv":              csv,
		"group_name":       "Mixed",
		"default_currency": "EUR",
		"members": []map[string]any{
			{"csv_name": "Alice", "email": "alice@example.com"},
			{"csv_name": "Bob", "email": "bob-stub@example.com"},
		},
		"dry_run": true,
	}
	resp, out := request(t, "POST", base+"/v1/imports/splitwise", body, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode, out)

	csvCurrencies, _ := out["csv_currencies"].([]any)
	require.Len(t, csvCurrencies, 2)
	require.Equal(t, "EUR", csvCurrencies[0])
	require.Equal(t, "USD", csvCurrencies[1])

	// Every preview row reports the chosen group currency.
	preview, _ := out["preview"].([]any)
	require.GreaterOrEqual(t, len(preview), 2)
	for _, row := range preview {
		m, _ := row.(map[string]any)
		require.Equal(t, "EUR", m["currency"])
	}

	// Commit and verify the committed expenses use the group currency too.
	body["dry_run"] = false
	resp, out = request(t, "POST", base+"/v1/imports/splitwise", body, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, out)
	gid, _ := out["group_id"].(string)
	resp, exps := requestList(t, "GET", base+"/v1/groups/"+gid+"/expenses", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, exps, 2)
	for _, e := range exps {
		require.Equal(t, "EUR", e["currency"], "imported expenses must ride the group currency")
	}
}

func TestImportSplitwise_Settlements(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL
	_, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")

	// Mirrors the vacuna-5g shape: one expense plus a Payment row that
	// closes part of the resulting debt. The Payment row must produce a
	// settlement, not an expense.
	csv := splitwiseHeader +
		"2024-01-01,Coffee,Dining out,4.00,EUR,-2.00,2.00\n" + // Bob paid; Alice owes 2
		"2024-01-02,Alice paid Bob,Payment,2.00,EUR,2.00,-2.00\n" // Alice settles 2 with Bob
	body := map[string]any{
		"csv":              csv,
		"group_name":       "Vacuna",
		"default_currency": "EUR",
		"members": []map[string]any{
			{"csv_name": "Alice", "email": "alice@example.com"},
			{"csv_name": "Bob", "email": "bob-stub@example.com"},
		},
		"dry_run": true,
	}

	// Dry run: 1 expense, 1 settlement, 0 skipped, balances both zero.
	resp, out := request(t, "POST", base+"/v1/imports/splitwise", body, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode, out)
	require.EqualValues(t, 1, out["expense_count"])
	require.EqualValues(t, 1, out["settlement_count"])
	require.EqualValues(t, 0, out["skipped_count"])
	stPrev, _ := out["settlement_preview"].([]any)
	require.Len(t, stPrev, 1)
	first, _ := stPrev[0].(map[string]any)
	require.Equal(t, "Alice", first["from_csv_name"])
	require.Equal(t, "Bob", first["to_csv_name"])
	require.EqualValues(t, 200, first["amount_cents"])
	bals, _ := out["balances"].([]any)
	require.Len(t, bals, 2)
	for _, b := range bals {
		m, _ := b.(map[string]any)
		require.EqualValues(t, 0, m["net_cents"], "balances must net to zero after expense + matching settlement")
	}

	// Commit: group has 1 expense and 1 settlement; the /balances endpoint
	// agrees with the projection (zero on both sides).
	body["dry_run"] = false
	resp, out = request(t, "POST", base+"/v1/imports/splitwise", body, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, out)
	gid, _ := out["group_id"].(string)
	require.NotEmpty(t, gid)

	resp, exps := requestList(t, "GET", base+"/v1/groups/"+gid+"/expenses", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, exps, 1, "Payment row must NOT be imported as an expense")

	resp, sts := requestList(t, "GET", base+"/v1/groups/"+gid+"/settlements", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, sts, 1)
	require.EqualValues(t, 200, sts[0]["amount_cents"])

	// /balances should report zero for both members (expense + settlement cancel).
	resp, balOut := request(t, "GET", base+"/v1/groups/"+gid+"/balances", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	netList, _ := balOut["net"].([]any)
	require.Len(t, netList, 2)
	for _, n := range netList {
		row, _ := n.(map[string]any)
		require.EqualValues(t, 0, row["net_cents"])
	}
}

func TestImportSplitwise_BadHeader(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL
	_, cookieA := registerUser(t, base, "a@test.dev", "passwordpassword", "Alice")
	body := map[string]any{
		"csv":              "Day,Description,Category,Cost,Currency,Alice,Bob\n",
		"group_name":       "X",
		"default_currency": "EUR",
		"members": []map[string]any{
			{"csv_name": "Alice", "email": "a@test.dev"},
			{"csv_name": "Bob", "email": "b@test.dev"},
		},
		"dry_run": true,
	}
	resp, _ := request(t, "POST", base+"/v1/imports/splitwise", body, cookieA)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestImportSplitwise_Oversized(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL
	_, cookieA := registerUser(t, base, "a@test.dev", "passwordpassword", "Alice")
	csv := splitwiseHeader + strings.Repeat("a", 300*1024)
	body := map[string]any{
		"csv":              csv,
		"group_name":       "X",
		"default_currency": "EUR",
		"members": []map[string]any{
			{"csv_name": "Alice", "email": "a@test.dev"},
			{"csv_name": "Bob", "email": "b@test.dev"},
		},
		"dry_run": true,
	}
	// The OpenAPI maxLength rejects this at JSON decode time only if the
	// generator enforces it. Our handler also enforces it via Parse; we
	// accept either path as long as the response is 4xx.
	resp, _ := request(t, "POST", base+"/v1/imports/splitwise", body, cookieA)
	require.GreaterOrEqual(t, resp.StatusCode, 400)
	require.Less(t, resp.StatusCode, 500)
}

func TestImportSplitwise_StubUserNoEnumeration(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	_, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")
	// Pre-register Bob so he's a real user for the second request.
	_, _ = registerUser(t, base, "bob-real@example.com", "passwordpassword", "Bob")

	csv := splitwiseHeader + "2024-01-01,Coffee,Dining out,4.00,EUR,-2.00,2.00\n"

	mk := func(email string) map[string]any {
		return map[string]any{
			"csv":              csv,
			"group_name":       "Prost",
			"default_currency": "EUR",
			"members": []map[string]any{
				{"csv_name": "Alice", "email": "alice@example.com"},
				{"csv_name": "Bob", "email": email},
			},
			"dry_run": true,
		}
	}

	respReal, outReal := request(t, "POST", base+"/v1/imports/splitwise", mk("bob-real@example.com"), cookieA)
	respStub, outStub := request(t, "POST", base+"/v1/imports/splitwise", mk("nobody@example.com"), cookieA)

	require.Equal(t, http.StatusOK, respReal.StatusCode)
	require.Equal(t, http.StatusOK, respStub.StatusCode)
	// The two responses differ only in the echoed email; everything else
	// (counts, preview content, ordering) must match exactly so the
	// caller can't tell whether the address was already registered.
	require.Equal(t, outReal["expense_count"], outStub["expense_count"])
	require.Equal(t, outReal["skipped_count"], outStub["skipped_count"])
	require.Equal(t, outReal["group_name"], outStub["group_name"])
	require.Equal(t, outReal["preview"], outStub["preview"])
}
