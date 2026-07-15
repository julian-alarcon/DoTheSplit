package server_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestImportGroupExpenses_RestoresCreatorAndTimestamp verifies the group-append
// importer honors the optional Created/CreatedBy columns: the appended expense
// is authored by the named member (Bob) with the original timestamp, even though
// Alice runs the import.
func TestImportGroupExpenses_RestoresCreatorAndTimestamp(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	_, aCookie := registerUser(t, base, "alice@test.dev", "passwordpassword", "Alice")
	_, _ = registerUser(t, base, "bob@test.dev", "passwordpassword", "Bob")

	resp, group := request(t, "POST", base+"/v1/groups", map[string]any{
		"name": "Trip", "default_currency": "EUR",
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	groupID := group["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/members",
		map[string]any{"email": "bob@test.dev"}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// CreatedBy names Bob and Created carries an explicit historical timestamp.
	csv := "Date,Description,Category,Cost,Currency,Payer,Notes,Created,CreatedBy\n" +
		"2026-06-01,Pizza,Food,42.00,EUR,Bob,,2026-06-01T19:31:07Z,Bob\n"
	body := map[string]any{"csv": csv, "dry_run": false}
	resp, out := request(t, "POST", base+"/v1/groups/"+groupID+"/imports/expenses", body, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, out)

	items, _ := activityItems(t, base, groupID, "", aCookie)
	require.Len(t, items, 1)
	ev := items[0].(map[string]any)
	require.Equal(t, "expense.created", ev["action"])
	require.Equal(t, "Bob", ev["actor"].(map[string]any)["display_name"],
		"CreatedBy column must set the activity actor to the named member")
	require.Equal(t,
		truncSecond(t, "2026-06-01T19:31:07Z"),
		truncSecond(t, ev["occurred_at"].(string)),
		"Created column must pin the event timestamp")
}

// TestImportGroupExpenses_EqualSplit drives the most common path: a
// 3-member group with no pinned default split. The importer should
// parse the CSV, fall back to equal splits across all members, and
// resolve payers by display name (case-insensitive). The minimal row
// (only Date/Description/Cost) and a row with an explicit Payer
// column must both land as expenses.
func TestImportGroupExpenses_EqualSplit(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	alice, aCookie := registerUser(t, base, "alice@test.dev", "passwordpassword", "Alice")
	_, _ = registerUser(t, base, "bob@test.dev", "passwordpassword", "Bob")
	_, _ = registerUser(t, base, "carol@test.dev", "passwordpassword", "Carol")

	resp, group := request(t, "POST", base+"/v1/groups", map[string]any{
		"name":             "Trip",
		"default_currency": "EUR",
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	groupID := group["id"].(string)

	for _, email := range []string{"bob@test.dev", "carol@test.dev"} {
		resp, _ := request(t, "POST", base+"/v1/groups/"+groupID+"/members", map[string]any{
			"email": email,
		}, aCookie)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	csv := "Date,Description,Category,Cost,Currency,Payer,Notes\n" +
		"2026-06-01,Pizza,Food,42.00,EUR,Bob,Friday night\n" +
		"2026-06-02,Cab,,18.50,,,\n" +
		"2026-06-03,Beer,,9.00,,Eve,\n" // Eve unknown - skipped

	body := map[string]any{"csv": csv, "dry_run": true}
	resp, out := request(t, "POST", base+"/v1/groups/"+groupID+"/imports/expenses", body, aCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode, out)
	require.EqualValues(t, 2, out["expense_count"])
	require.EqualValues(t, 1, out["skipped_count"])

	body["dry_run"] = false
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/imports/expenses", body, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, exps := requestList(t, "GET", base+"/v1/groups/"+groupID+"/expenses", nil, aCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, exps, 2)

	// Pizza row: payer is Bob (lookup by display name), splits equal across 3.
	var pizza, cab map[string]any
	for _, e := range exps {
		switch e["description"] {
		case "Pizza":
			pizza = e
		case "Cab":
			cab = e
		}
	}
	require.NotNil(t, pizza)
	require.NotNil(t, cab)
	// Equal split across 3 members: 42.00 -> shares 1400/1400/1400 = 4200.
	splits, _ := pizza["splits"].([]any)
	require.Len(t, splits, 3)
	var sum int64
	for _, s := range splits {
		m := s.(map[string]any)
		v, ok := m["share_cents"].(float64)
		require.True(t, ok)
		sum += int64(v)
	}
	require.EqualValues(t, 4200, sum)
	// Cab row: empty payer falls back to the importer (Alice). Currency
	// falls back to the group's default (EUR).
	require.Equal(t, alice["id"], cab["payer_id"])
	require.Equal(t, "EUR", cab["currency"])
	// Pizza notes round-trip.
	require.Equal(t, "Friday night", pizza["notes"])
}

// TestImportGroupExpenses_PinnedDefaultSplit covers the 2-member
// percent-split path. Default split 70/30 must produce shares 70/30 of
// the row amount.
func TestImportGroupExpenses_PinnedDefaultSplit(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	alice, aCookie := registerUser(t, base, "alice@test.dev", "passwordpassword", "Alice")
	bob, _ := registerUser(t, base, "bob@test.dev", "passwordpassword", "Bob")

	resp, group := request(t, "POST", base+"/v1/groups", map[string]any{
		"name":             "Pair",
		"default_currency": "EUR",
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	groupID := group["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/members", map[string]any{
		"email": "bob@test.dev",
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Pin a 70/30 default split.
	resp, _ = request(t, "PATCH", base+"/v1/groups/"+groupID, map[string]any{
		"default_split": []map[string]any{
			{"user_id": alice["id"], "basis_points": 7000},
			{"user_id": bob["id"], "basis_points": 3000},
		},
	}, aCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	csv := "Date,Description,Category,Cost,Currency\n" +
		"2026-06-01,Rent,,100.00,EUR\n"
	body := map[string]any{"csv": csv, "dry_run": false}
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/imports/expenses", body, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, exps := requestList(t, "GET", base+"/v1/groups/"+groupID+"/expenses", nil, aCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, exps, 1)
	splits, _ := exps[0]["splits"].([]any)
	require.Len(t, splits, 2)
	byUser := map[string]int64{}
	for _, s := range splits {
		m := s.(map[string]any)
		byUser[m["user_id"].(string)] = int64(m["share_cents"].(float64))
	}
	require.EqualValues(t, 7000, byUser[alice["id"].(string)])
	require.EqualValues(t, 3000, byUser[bob["id"].(string)])
}

// TestImportGroupExpenses_BadCSVHeader returns 400.
func TestImportGroupExpenses_BadCSVHeader(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL
	_, aCookie := registerUser(t, base, "alice@test.dev", "passwordpassword", "Alice")
	resp, group := request(t, "POST", base+"/v1/groups", map[string]any{
		"name": "G", "default_currency": "EUR",
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	groupID := group["id"].(string)

	body := map[string]any{"csv": "nope,not,a,header\n"}
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/imports/expenses", body, aCookie)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
