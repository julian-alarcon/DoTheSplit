package server_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStrictJSONRejectsUnknownFields asserts the contract documented in
// AGENTS.md: every request body uses additionalProperties: false. A typo'd
// or smuggled field must surface as a 400 from bindStrictJSON before the
// service ever sees it - this is what blocks mass-assignment attacks
// (TestRegisterRejectsRoleField is the canonical regression for this).
//
// We exercise a representative set of every shape: public auth, public
// setup, authenticated me/groups/expenses/settlements/recurring. The
// "extra" field name varies per row to avoid accidental valid-field
// collisions in future refactors.
func TestStrictJSONRejectsUnknownFields(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	// Establish two members + one group + one expense + one settlement so
	// every endpoint under test has live IDs to point at.
	alice, aCookie := registerUser(t, base, "alice@test.dev", "passwordpassword", "Alice")
	bob, _ := registerUser(t, base, "bob@test.dev", "passwordpassword", "Bob")

	resp, group := request(t, "POST", base+"/v1/groups", map[string]any{
		"name":             "Trip",
		"default_currency": "EUR",
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	groupID := group["id"].(string)

	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/members", map[string]any{
		"email": "bob@test.dev",
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, cats := requestList(t, "GET", base+"/v1/categories", nil, aCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	categoryID := cats[0]["id"].(string)

	resp, expense := request(t, "POST", base+"/v1/groups/"+groupID+"/expenses", map[string]any{
		"description":  "Dinner",
		"amount_cents": 2000,
		"payer_id":     alice["id"],
		"category_id":  categoryID,
		"mode":         "equal",
		"splits": []map[string]any{
			{"user_id": alice["id"]},
			{"user_id": bob["id"]},
		},
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, expense)
	expenseID := expense["id"].(string)

	cases := []struct {
		name       string
		method     string
		path       string
		auth       *http.Cookie // nil = anonymous
		body       map[string]any
		extraField string
	}{
		// Public auth
		{
			name:   "register",
			method: "POST",
			path:   "/v1/auth/register",
			body: map[string]any{
				"email": "x@test.dev", "password": "passwordpassword", "display_name": "X",
			},
			extraField: "role", // mass-assignment regression
		},
		{
			name:   "login",
			method: "POST",
			path:   "/v1/auth/login",
			body: map[string]any{
				"email": "alice@test.dev", "password": "passwordpassword",
			},
			extraField: "remember_me",
		},
		// Authenticated me
		{
			name:   "patch_me",
			method: "PATCH",
			path:   "/v1/me",
			auth:   aCookie,
			body: map[string]any{
				"display_name": "Alice Renamed",
			},
			extraField: "is_admin",
		},
		{
			name:   "change_password",
			method: "POST",
			path:   "/v1/me/password",
			auth:   aCookie,
			body: map[string]any{
				"old_password": "passwordpassword", "new_password": "newpasswordnew",
			},
			extraField: "user_id",
		},
		{
			name:   "set_avatar",
			method: "PUT",
			path:   "/v1/me/avatar",
			auth:   aCookie,
			body: map[string]any{
				"png_base64": "iVBORw0KGgo=",
			},
			extraField: "scale",
		},
		// Groups
		{
			name:   "create_group",
			method: "POST",
			path:   "/v1/groups",
			auth:   aCookie,
			body: map[string]any{
				"name": "G2", "default_currency": "EUR",
			},
			extraField: "owner_id",
		},
		{
			name:   "patch_group",
			method: "PATCH",
			path:   "/v1/groups/" + groupID,
			auth:   aCookie,
			body: map[string]any{
				"name": "Renamed",
			},
			extraField: "created_by",
		},
		{
			name:   "add_member",
			method: "POST",
			path:   "/v1/groups/" + groupID + "/members",
			auth:   aCookie,
			body: map[string]any{
				"email": "ghost@test.dev",
			},
			extraField: "role",
		},
		// Expenses
		{
			name:   "create_expense",
			method: "POST",
			path:   "/v1/groups/" + groupID + "/expenses",
			auth:   aCookie,
			body: map[string]any{
				"description":  "T",
				"amount_cents": 100,
				"payer_id":     alice["id"],
				"category_id":  categoryID,
				"mode":         "equal",
				"splits": []map[string]any{
					{"user_id": alice["id"]},
				},
			},
			extraField: "deleted_at",
		},
		{
			name:   "update_expense",
			method: "PATCH",
			path:   "/v1/expenses/" + expenseID,
			auth:   aCookie,
			body: map[string]any{
				"description": "Updated",
			},
			extraField: "group_id",
		},
		// Settlements
		{
			name:   "create_settlement",
			method: "POST",
			path:   "/v1/groups/" + groupID + "/settlements",
			auth:   aCookie,
			body: map[string]any{
				"to_user_id":   bob["id"],
				"amount_cents": 100,
			},
			extraField: "approved_by",
		},
		// Recurring
		{
			name:   "create_recurring",
			method: "POST",
			path:   "/v1/groups/" + groupID + "/recurring-expenses",
			auth:   aCookie,
			body: map[string]any{
				"description":  "Rent",
				"amount_cents": 100,
				"payer_id":     alice["id"],
				"category_id":  categoryID,
				"mode":         "equal",
				"splits":       []map[string]any{{"user_id": alice["id"]}},
				"cadence":      "monthly",
				"next_run_at":  "2030-01-01T12:00:00Z",
			},
			extraField: "owner",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{}
			for k, v := range tc.body {
				body[k] = v
			}
			body[tc.extraField] = "smuggled"

			resp, out := request(t, tc.method, base+tc.path, body, tc.auth)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode,
				"%s %s with unknown field %q must be 400, got %d body=%v",
				tc.method, tc.path, tc.extraField, resp.StatusCode, out)
		})
	}
}
