package server_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGroupAuthzNegativeMatrix asserts that a registered, authenticated
// user who is NOT a member of a group gets 403 (or 404 where the route
// uses an indirect ID like /expenses/{id} without surfacing the group)
// from every group-scoped endpoint. Mirrors TestAdminAuthzNegativeDestructive
// but for the membership boundary rather than the role boundary.
//
// Why this matters: services depend on GroupService.RequireMember /
// IsMember being called on EVERY group-scoped path. A regression where
// some new endpoint forgets the check would let any authenticated user
// peek at or mutate other groups. This matrix is the early-warning system.
func TestGroupAuthzNegativeMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	// Owner registers and creates a group with a real expense + settlement +
	// recurring template + revision so every detail-level endpoint has a
	// concrete ID to point at.
	owner, ownerCookie := registerUser(t, base, "owner@test.dev", "passwordpassword", "Owner")
	other, otherCookie := registerUser(t, base, "other@test.dev", "passwordpassword", "Other")

	resp, group := request(t, "POST", base+"/v1/groups", map[string]any{
		"name":             "Private",
		"default_currency": "EUR",
	}, ownerCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	groupID := group["id"].(string)

	// Add a second member to the owner's group so split has 2 users.
	_, secondCookie := registerUser(t, base, "second@test.dev", "passwordpassword", "Second")
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/members", map[string]any{
		"email": "second@test.dev",
	}, ownerCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	_ = secondCookie

	resp, cats := requestList(t, "GET", base+"/v1/categories", nil, ownerCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	categoryID := cats[0]["id"].(string)

	resp, secondUser := request(t, "GET", base+"/v1/me", nil, secondCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	secondUserID := secondUser["id"].(string)

	resp, expense := request(t, "POST", base+"/v1/groups/"+groupID+"/expenses", map[string]any{
		"description":  "Dinner",
		"amount_cents": 2000,
		"payer_id":     owner["id"],
		"category_id":  categoryID,
		"mode":         "equal",
		"splits": []map[string]any{
			{"user_id": owner["id"]},
			{"user_id": secondUserID},
		},
	}, ownerCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, expense)
	expenseID := expense["id"].(string)

	// Settled by second member; the API infers from_user from the session.
	resp, settlement := request(t, "POST", base+"/v1/groups/"+groupID+"/settlements", map[string]any{
		"to_user_id":   owner["id"],
		"amount_cents": 500,
	}, secondCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, settlement)
	settlementID := settlement["id"].(string)

	resp, recurring := request(t, "POST", base+"/v1/groups/"+groupID+"/recurring-expenses", map[string]any{
		"description":  "Rent",
		"amount_cents": 100,
		"payer_id":     owner["id"],
		"category_id":  categoryID,
		"mode":         "equal",
		"splits":       []map[string]any{{"user_id": owner["id"]}, {"user_id": secondUserID}},
		"cadence":      "monthly",
		"next_run_at":  "2030-01-01T12:00:00Z",
	}, ownerCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, recurring)
	recurringID := recurring["id"].(string)

	_ = other // available if a future check needs the user's id

	// Each entry: a request that "other" (a registered user but NOT a member
	// of the group) should be rejected on. We accept either 403 (membership
	// boundary detected) or 404 (resource indistinguishable from missing -
	// tighter for enumeration safety). Both are valid; what is NEVER allowed
	// is 200/201/204.
	cases := []struct {
		name   string
		method string
		path   string
		body   map[string]any
	}{
		// Group-scoped reads
		{"list_expenses", "GET", "/v1/groups/" + groupID + "/expenses", nil},
		{"list_settlements", "GET", "/v1/groups/" + groupID + "/settlements", nil},
		{"list_transactions", "GET", "/v1/groups/" + groupID + "/transactions", nil},
		{"list_activity", "GET", "/v1/groups/" + groupID + "/activity", nil},
		{"list_recurring", "GET", "/v1/groups/" + groupID + "/recurring-expenses", nil},
		{"get_balances", "GET", "/v1/groups/" + groupID + "/balances", nil},
		{"export_csv", "GET", "/v1/groups/" + groupID + "/export.csv", nil},

		// Group-scoped writes
		{"add_member", "POST", "/v1/groups/" + groupID + "/members", map[string]any{"email": "other@test.dev"}},
		{"remove_member", "DELETE", "/v1/groups/" + groupID + "/members/" + owner["id"].(string), nil},
		{"patch_group", "PATCH", "/v1/groups/" + groupID, map[string]any{"name": "Hijacked"}},
		{"delete_group", "DELETE", "/v1/groups/" + groupID, nil},
		{"create_expense", "POST", "/v1/groups/" + groupID + "/expenses", map[string]any{
			"description": "x", "amount_cents": 100, "payer_id": owner["id"], "category_id": categoryID,
			"mode": "equal", "splits": []map[string]any{{"user_id": owner["id"]}},
		}},
		{"create_settlement", "POST", "/v1/groups/" + groupID + "/settlements", map[string]any{
			"to_user_id": secondUserID, "amount_cents": 100,
		}},
		{"create_recurring", "POST", "/v1/groups/" + groupID + "/recurring-expenses", map[string]any{
			"description": "x", "amount_cents": 100, "payer_id": owner["id"], "category_id": categoryID,
			"mode": "equal", "splits": []map[string]any{{"user_id": owner["id"]}},
			"cadence": "monthly", "next_run_at": "2030-01-01T12:00:00Z",
		}},
		{"import_group_expenses_csv", "POST", "/v1/groups/" + groupID + "/imports/expenses", map[string]any{
			"csv": "Date,Description,Category,Cost,Currency\n2026-06-01,x,,1.00,EUR\n",
		}},

		// Detail-level routes that resolve through to a group implicitly.
		{"get_expense", "GET", "/v1/expenses/" + expenseID, nil},
		{"update_expense", "PATCH", "/v1/expenses/" + expenseID, map[string]any{"description": "hijack"}},
		{"delete_expense", "DELETE", "/v1/expenses/" + expenseID, nil},
		{"list_expense_revisions", "GET", "/v1/expenses/" + expenseID + "/revisions", nil},
		{"get_settlement", "GET", "/v1/settlements/" + settlementID, nil},
		{"update_settlement", "PATCH", "/v1/settlements/" + settlementID, map[string]any{"amount_cents": 9999}},
		{"delete_settlement", "DELETE", "/v1/settlements/" + settlementID, nil},
		{"delete_recurring", "DELETE", "/v1/recurring-expenses/" + recurringID, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := request(t, tc.method, base+tc.path, tc.body, otherCookie)
			require.Truef(t,
				resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound,
				"%s %s by non-member must be 403 or 404, got %d body=%v",
				tc.method, tc.path, resp.StatusCode, body)
		})
	}

	// Anonymous (no cookie) must always get 401 from the auth middleware
	// before authorization is even evaluated.
	t.Run("anonymous_blocked", func(t *testing.T) {
		for _, path := range []string{
			"/v1/groups/" + groupID + "/expenses",
			"/v1/groups/" + groupID + "/settlements",
			"/v1/groups/" + groupID + "/balances",
			"/v1/expenses/" + expenseID,
		} {
			resp, _ := request(t, "GET", base+path, nil, nil)
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "GET %s anon", path)
		}
	})
}
