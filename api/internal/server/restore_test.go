package server_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRestoreExpenseAndSettlement covers the soft-delete restore lifecycle:
//   - GET of a deleted item now returns 200 with deleted_at set (no longer 404),
//   - POST .../restore brings it back (200, deleted_at null) and it reappears in
//     the group listing,
//   - restoring an already-active item is a 409 conflict,
//   - the activity feed records the *.restored events with the acting user.
func TestRestoreExpenseAndSettlement(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	alice, cookieA := registerUser(t, base, "rest-a@test.dev", "passwordpassword", "Alice")
	bob, cookieB := registerUser(t, base, "rest-b@test.dev", "passwordpassword", "Bob")

	resp, group := request(t, "POST", base+"/v1/groups", map[string]any{
		"name": "Restore", "default_currency": "EUR",
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	groupID := group["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/members",
		map[string]any{"email": "rest-b@test.dev"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, cats := requestList(t, "GET", base+"/v1/categories", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	categoryID := cats[0]["id"].(string)

	// --- Expense: create, delete, restore ---
	resp, exp := request(t, "POST", base+"/v1/groups/"+groupID+"/expenses", map[string]any{
		"description":  "Dinner",
		"amount_cents": 2000,
		"payer_id":     alice["id"],
		"category_id":  categoryID,
		"mode":         "equal",
		"splits":       []map[string]any{{"user_id": alice["id"]}, {"user_id": bob["id"]}},
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, exp)
	expenseID := exp["id"].(string)

	// Restoring an active expense is a conflict.
	resp, _ = request(t, "POST", base+"/v1/expenses/"+expenseID+"/restore", nil, cookieA)
	require.Equal(t, http.StatusConflict, resp.StatusCode)

	resp, _ = request(t, "DELETE", base+"/v1/expenses/"+expenseID, nil, cookieA)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// GET a deleted expense now succeeds and surfaces deleted_at.
	resp, deletedExp := request(t, "GET", base+"/v1/expenses/"+expenseID, nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode, deletedExp)
	require.NotEmpty(t, deletedExp["deleted_at"], "deleted expense must expose deleted_at")

	// It is filtered out of the active group listing.
	resp, listed := requestList(t, "GET", base+"/v1/groups/"+groupID+"/expenses", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, listed, "deleted expense must not appear in the active listing")

	// Restore (by Bob, any member) → 200, deleted_at cleared.
	resp, restoredExp := request(t, "POST", base+"/v1/expenses/"+expenseID+"/restore", nil, cookieB)
	require.Equal(t, http.StatusOK, resp.StatusCode, restoredExp)
	require.Nil(t, restoredExp["deleted_at"], "restored expense must have null deleted_at")

	resp, listed = requestList(t, "GET", base+"/v1/groups/"+groupID+"/expenses", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, listed, 1, "restored expense reappears in the active listing")

	// --- Settlement: create, delete, restore ---
	resp, settle := request(t, "POST", base+"/v1/groups/"+groupID+"/settlements", map[string]any{
		"to_user_id":   alice["id"],
		"amount_cents": 1500,
		"note":         "payback",
	}, cookieB)
	require.Equal(t, http.StatusCreated, resp.StatusCode, settle)
	settlementID := settle["id"].(string)

	resp, _ = request(t, "DELETE", base+"/v1/settlements/"+settlementID, nil, cookieB)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp, deletedSt := request(t, "GET", base+"/v1/settlements/"+settlementID, nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode, deletedSt)
	require.NotEmpty(t, deletedSt["deleted_at"], "deleted settlement must expose deleted_at")

	resp, restoredSt := request(t, "POST", base+"/v1/settlements/"+settlementID+"/restore", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode, restoredSt)
	require.Nil(t, restoredSt["deleted_at"], "restored settlement must have null deleted_at")

	// Restoring it again is a conflict.
	resp, _ = request(t, "POST", base+"/v1/settlements/"+settlementID+"/restore", nil, cookieA)
	require.Equal(t, http.StatusConflict, resp.StatusCode)

	// --- Activity feed records both restore events with the actor ---
	items, _ := activityItems(t, base, groupID, "", cookieA)
	byAction := map[string]map[string]any{}
	for _, raw := range items {
		m := raw.(map[string]any)
		byAction[m["action"].(string)] = m
	}

	expRestored := byAction["expense.restored"]
	require.NotNil(t, expRestored, "expense.restored event must be recorded")
	require.Equal(t, "Dinner", expRestored["description"])
	require.Equal(t, bob["id"], expRestored["actor"].(map[string]any)["user_id"])

	stRestored := byAction["settlement.restored"]
	require.NotNil(t, stRestored, "settlement.restored event must be recorded")
	require.Equal(t, alice["id"], stRestored["actor"].(map[string]any)["user_id"])
}
