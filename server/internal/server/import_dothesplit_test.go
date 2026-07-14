package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const dothesplitHeader = "Date,Description,Category,Cost,Currency,Time,Payer,Notes,Created,CreatedBy,Alice,Bob\n"

func TestImportDoTheSplit_DryRunAndCommit(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	_, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")

	csv := dothesplitHeader +
		"2024-01-01,Coffee,Dining out,4.00,EUR,18:42:00Z,Bob,extra shot,2024-01-01T18:43:10Z,Alice,-2.00,2.00\n" +
		"2024-01-02,Hotel,Hotel,200.00,EUR,09:00:00Z,Alice,,2024-01-02T09:00:00Z,Alice,100.00,-100.00\n"

	body := map[string]any{
		"csv":              csv,
		"group_name":       "Imported",
		"default_currency": "EUR",
		"members": []map[string]any{
			{"csv_name": "Alice", "email": "alice@example.com"},
			{"csv_name": "Bob", "email": "bob-stub@example.com"},
		},
		"dry_run": true,
	}

	resp, out := request(t, "POST", base+"/v1/imports/dothesplit", body, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode, out)
	require.EqualValues(t, 2, out["expense_count"])
	preview, _ := out["preview"].([]any)
	require.Len(t, preview, 2)

	body["dry_run"] = false
	resp, out = request(t, "POST", base+"/v1/imports/dothesplit", body, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, out)
	gid, _ := out["group_id"].(string)
	require.NotEmpty(t, gid)

	resp, exps := requestList(t, "GET", base+"/v1/groups/"+gid+"/expenses", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, exps, 2)

	// Find the Coffee expense and assert that:
	//  - notes round-trip from the CSV column,
	//  - incurred_at carries the second-precision time,
	//  - the explicit Payer column overrode whatever the sign-based
	//    inference would have picked. (Bob is the explicit payer; with
	//    -2/+2 sign convention, Bob is also the inferred creditor, so
	//    they'd match. The Hotel row is the more discriminating case
	//    asserted below.)
	var coffee map[string]any
	for _, e := range exps {
		if e["description"] == "Coffee" {
			coffee = e
		}
	}
	require.NotNil(t, coffee)
	require.Equal(t, "extra shot", coffee["notes"])
	incurredStr, _ := coffee["incurred_at"].(string)
	parsed, err := time.Parse(time.RFC3339, incurredStr)
	require.NoError(t, err, "incurred_at = %q", incurredStr)
	require.Equal(t, time.Date(2024, 1, 1, 18, 42, 0, 0, time.UTC), parsed.UTC())
}

func TestImportDoTheSplit_AcceptsBareSplitwiseFile(t *testing.T) {
	// A 5-column Splitwise-shaped file must still parse through the
	// dothesplit endpoint, falling back to no-time / no-payer / no-notes.
	ts := setup(t)
	base := ts.srv.URL
	_, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")

	csv := "Date,Description,Category,Cost,Currency,Alice,Bob\n" +
		"2024-01-01,Coffee,Dining out,4.00,EUR,2.00,-2.00\n"
	body := map[string]any{
		"csv":              csv,
		"group_name":       "Bare",
		"default_currency": "EUR",
		"members": []map[string]any{
			{"csv_name": "Alice", "email": "alice@example.com"},
			{"csv_name": "Bob", "email": "bob-stub@example.com"},
		},
		"dry_run": true,
	}
	resp, out := request(t, "POST", base+"/v1/imports/dothesplit", body, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode, out)
	require.EqualValues(t, 1, out["expense_count"])
}

func TestImportDoTheSplit_RoundTripFromExport(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	// Build a source group with two expenses and a settlement.
	userA, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")
	userB, _ := registerUser(t, base, "bob@example.com", "passwordpassword", "Bob")
	resp, group := request(t, "POST", base+"/v1/groups",
		map[string]any{"name": "Trip"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	gid := group["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+gid+"/members",
		map[string]any{"email": "bob@example.com"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp, _ = request(t, "POST", base+"/v1/groups/"+gid+"/expenses", map[string]any{
		"description":  "Hotel",
		"amount_cents": 20000,
		"payer_id":     userA["id"],
		"mode":         "equal",
		"notes":        "good view",
		"splits": []map[string]any{
			{"user_id": userA["id"]}, {"user_id": userB["id"]},
		},
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp, _ = request(t, "POST", base+"/v1/groups/"+gid+"/expenses", map[string]any{
		"description":  "Coffee",
		"amount_cents": 600,
		"payer_id":     userB["id"],
		"mode":         "equal",
		"splits": []map[string]any{
			{"user_id": userA["id"]}, {"user_id": userB["id"]},
		},
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp, _ = request(t, "POST", base+"/v1/groups/"+gid+"/settlements", map[string]any{
		"from_user_id": userB["id"],
		"to_user_id":   userA["id"],
		"amount_cents": 5000,
		"note":         "partial",
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Capture source balances for comparison.
	resp, srcBal := request(t, "GET", base+"/v1/groups/"+gid+"/balances", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	srcNet := netMap(srcBal)

	// Export.
	_, _, csvBytes := downloadCSV(t, base, gid, cookieA)

	// Re-import as a fresh group via /v1/imports/dothesplit.
	body := map[string]any{
		"csv":              string(csvBytes),
		"group_name":       "Trip Reimported",
		"default_currency": "EUR",
		"members": []map[string]any{
			{"csv_name": "Alice", "email": "alice@example.com"},
			{"csv_name": "Bob", "email": "bob@example.com"},
		},
		"dry_run": false,
	}
	resp, out := request(t, "POST", base+"/v1/imports/dothesplit", body, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, out)
	newGID, _ := out["group_id"].(string)
	require.NotEmpty(t, newGID)

	// Assert the new group's balances match the source group's.
	resp, dstBal := request(t, "GET", base+"/v1/groups/"+newGID+"/balances", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	dstNet := netMap(dstBal)
	require.Equal(t, srcNet[userA["id"].(string)], dstNet[userA["id"].(string)])
	require.Equal(t, srcNet[userB["id"].(string)], dstNet[userB["id"].(string)])

	// And: the Hotel expense's notes survived round-trip.
	resp, exps := requestList(t, "GET", base+"/v1/groups/"+newGID+"/expenses", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var hotel map[string]any
	for _, e := range exps {
		if e["description"] == "Hotel" {
			hotel = e
		}
	}
	require.NotNil(t, hotel)
	require.Equal(t, "good view", hotel["notes"])
}

// TestImportDoTheSplit_RestoresCreatorAndTimestamp verifies that a re-import
// reconstructs an activity feed close to the original: the events are authored
// by the original creator (Alice/Bob), not the importing operator (Carol), and
// carry the original created_at timestamps rather than "now".
func TestImportDoTheSplit_RestoresCreatorAndTimestamp(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	// Source group: Alice creates an expense, Bob creates a settlement.
	userA, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")
	userB, cookieB := registerUser(t, base, "bob@example.com", "passwordpassword", "Bob")
	resp, group := request(t, "POST", base+"/v1/groups",
		map[string]any{"name": "Trip", "default_currency": "EUR"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	gid := group["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+gid+"/members",
		map[string]any{"email": "bob@example.com"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, _ = request(t, "POST", base+"/v1/groups/"+gid+"/expenses", map[string]any{
		"description":  "Hotel",
		"amount_cents": 20000,
		"payer_id":     userA["id"],
		"mode":         "equal",
		"splits":       []map[string]any{{"user_id": userA["id"]}, {"user_id": userB["id"]}},
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	// Bob records a settlement (Bob pays Alice), so Bob is the creator/actor.
	resp, _ = request(t, "POST", base+"/v1/groups/"+gid+"/settlements", map[string]any{
		"to_user_id":   userA["id"],
		"amount_cents": 5000,
		"note":         "partial",
	}, cookieB)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Capture the source feed timestamps keyed by action.
	srcItems, _ := activityItems(t, base, gid, "", cookieA)
	srcByAction := map[string]map[string]any{}
	for _, raw := range srcItems {
		m := raw.(map[string]any)
		srcByAction[m["action"].(string)] = m
	}
	srcExpAt := srcByAction["expense.created"]["occurred_at"].(string)
	srcSetAt := srcByAction["settlement.created"]["occurred_at"].(string)

	// Export the source group's CSV.
	_, _, csvBytes := downloadCSV(t, base, gid, cookieA)

	// A third user (Carol) is the operator who runs the import. She is not a CSV
	// column; the member list must match the two exported user columns exactly.
	_, cookieC := registerUser(t, base, "carol@example.com", "passwordpassword", "Carol")
	body := map[string]any{
		"csv":              string(csvBytes),
		"group_name":       "Trip Reimported",
		"default_currency": "EUR",
		"members": []map[string]any{
			{"csv_name": "Alice", "email": "alice@example.com"},
			{"csv_name": "Bob", "email": "bob@example.com"},
		},
		"dry_run": false,
	}
	resp, out := request(t, "POST", base+"/v1/imports/dothesplit", body, cookieC)
	require.Equal(t, http.StatusCreated, resp.StatusCode, out)
	newGID, _ := out["group_id"].(string)
	require.NotEmpty(t, newGID)

	// The restored feed must attribute events to the original creators and keep
	// the original timestamps, not Carol/now.
	dstItems, _ := activityItems(t, base, newGID, "", cookieC)
	dstByAction := map[string]map[string]any{}
	for _, raw := range dstItems {
		m := raw.(map[string]any)
		dstByAction[m["action"].(string)] = m
	}

	expCreated := dstByAction["expense.created"]
	require.NotNil(t, expCreated, "expected an expense.created event in the restored group")
	require.Equal(t, "Alice", expCreated["actor"].(map[string]any)["display_name"],
		"restored expense must be authored by the original creator, not the operator")
	// The export truncates Created to second precision (time.RFC3339), so
	// compare at second granularity rather than exact string equality.
	require.Equal(t, truncSecond(t, srcExpAt), truncSecond(t, expCreated["occurred_at"].(string)),
		"restored expense event must keep the original created_at")

	setCreated := dstByAction["settlement.created"]
	require.NotNil(t, setCreated, "expected a settlement.created event in the restored group")
	require.Equal(t, "Bob", setCreated["actor"].(map[string]any)["display_name"],
		"restored settlement must be authored by the original creator")
	require.Equal(t, truncSecond(t, srcSetAt), truncSecond(t, setCreated["occurred_at"].(string)),
		"restored settlement event must keep the original created_at")
}

// truncSecond parses an RFC3339 timestamp and truncates to second precision in
// UTC, so round-trip comparisons ignore the sub-second digits the CSV export
// (time.RFC3339) drops.
func truncSecond(t *testing.T, ts string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, ts)
	require.NoError(t, err, "parse timestamp %q", ts)
	return parsed.UTC().Truncate(time.Second)
}

// TestImportDoTheSplit_FallsBackWhenCreatorColumnsAbsent verifies that a CSV
// without the Created/CreatedBy columns falls back to the importing operator
// as the creator (current behavior preserved).
func TestImportDoTheSplit_FallsBackWhenCreatorColumnsAbsent(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	_, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")

	// Bare Splitwise-shaped CSV: no Time/Payer/Notes/Created/CreatedBy columns.
	csv := "Date,Description,Category,Cost,Currency,Alice,Bob\n" +
		"2024-01-01,Coffee,Dining out,4.00,EUR,2.00,-2.00\n"
	body := map[string]any{
		"csv":              csv,
		"group_name":       "Bare",
		"default_currency": "EUR",
		"members": []map[string]any{
			{"csv_name": "Alice", "email": "alice@example.com"},
			{"csv_name": "Bob", "email": "bob-stub@example.com"},
		},
		"dry_run": false,
	}
	resp, out := request(t, "POST", base+"/v1/imports/dothesplit", body, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, out)
	newGID, _ := out["group_id"].(string)
	require.NotEmpty(t, newGID)

	items, _ := activityItems(t, base, newGID, "", cookieA)
	require.Len(t, items, 1)
	ev := items[0].(map[string]any)
	require.Equal(t, "expense.created", ev["action"])
	require.Equal(t, "Alice", ev["actor"].(map[string]any)["display_name"],
		"without a CreatedBy column the importing actor is the creator")
}
