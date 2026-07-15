package server_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// activityItems fetches the activity feed and returns the raw items slice.
func activityItems(t *testing.T, base, groupID, query string, cookie *http.Cookie) ([]any, map[string]any) {
	t.Helper()
	url := base + "/v1/groups/" + groupID + "/activity"
	if query != "" {
		url += "?" + query
	}
	resp, body := request(t, "GET", url, nil, cookie)
	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	items, _ := body["items"].([]any)
	return items, body
}

// actionsOf maps the feed into the ordered list of action strings.
func actionsOf(items []any) []string {
	out := make([]string, 0, len(items))
	for _, raw := range items {
		out = append(out, raw.(map[string]any)["action"].(string))
	}
	return out
}

// TestActivityFeed exercises the group activity feed end-to-end: every event
// type is recorded with the correct actor, the deleted events keep the target
// description, a category change carries the new slug, no-op updates emit
// nothing, and a worker-generated expense is flagged recurring with no actor.
func TestActivityFeed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	alice, cookieA := registerUser(t, base, "act-a@test.dev", "passwordpassword", "Alice")
	bob, cookieB := registerUser(t, base, "act-b@test.dev", "passwordpassword", "Bob")

	resp, group := request(t, "POST", base+"/v1/groups", map[string]any{
		"name": "Acts", "default_currency": "EUR",
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	groupID := group["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/members",
		map[string]any{"email": "act-b@test.dev"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Two categories so we can assert a category change carries the new slug.
	resp, cats := requestList(t, "GET", base+"/v1/categories", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var groceriesID, trainID, trainSlug string
	for _, c := range cats {
		switch c["slug"] {
		case "groceries":
			groceriesID = c["id"].(string)
		case "train":
			trainID = c["id"].(string)
			trainSlug = c["slug"].(string)
		}
	}
	require.NotEmpty(t, groceriesID)
	require.NotEmpty(t, trainID)

	// --- Expense lifecycle: create (A), update category (B), delete (A) ---
	resp, exp := request(t, "POST", base+"/v1/groups/"+groupID+"/expenses", map[string]any{
		"description":  "Groceries run",
		"amount_cents": 3000,
		"payer_id":     alice["id"],
		"category_id":  groceriesID,
		"mode":         "equal",
		"splits": []map[string]any{
			{"user_id": alice["id"]}, {"user_id": bob["id"]},
		},
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, exp)
	expenseID := exp["id"].(string)

	resp, _ = request(t, "PATCH", base+"/v1/expenses/"+expenseID, map[string]any{
		"category_id": trainID,
	}, cookieB)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// No-op update: same description it already has -> must NOT emit an event.
	resp, _ = request(t, "PATCH", base+"/v1/expenses/"+expenseID, map[string]any{
		"description": "Groceries run",
	}, cookieB)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, _ = request(t, "DELETE", base+"/v1/expenses/"+expenseID, nil, cookieA)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// --- Settlement lifecycle: create (B), update (A), delete (B) ---
	resp, settle := request(t, "POST", base+"/v1/groups/"+groupID+"/settlements", map[string]any{
		"to_user_id":   alice["id"],
		"amount_cents": 1500,
		"note":         "payback",
	}, cookieB)
	require.Equal(t, http.StatusCreated, resp.StatusCode, settle)
	settlementID := settle["id"].(string)

	resp, _ = request(t, "PATCH", base+"/v1/settlements/"+settlementID, map[string]any{
		"amount_cents": 1600,
	}, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, _ = request(t, "DELETE", base+"/v1/settlements/"+settlementID, nil, cookieB)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// --- The feed ---
	items, body := activityItems(t, base, groupID, "", cookieA)
	// 3 expense events + 3 settlement events; the no-op update emits nothing.
	require.Len(t, items, 6, "expected exactly 6 events (no-op update must not count)")

	// Newest first: settlement.deleted is the last action taken.
	got := actionsOf(items)
	require.Equal(t, []string{
		"settlement.deleted",
		"settlement.updated",
		"settlement.created",
		"expense.deleted",
		"expense.updated",
		"expense.created",
	}, got)

	// occurred_at strictly newest-first.
	var lastAt string
	for _, raw := range items {
		at := raw.(map[string]any)["occurred_at"].(string)
		if lastAt != "" {
			require.True(t, at <= lastAt, "feed must be newest-first; %s after %s", at, lastAt)
		}
		lastAt = at
	}

	byAction := map[string]map[string]any{}
	for _, raw := range items {
		m := raw.(map[string]any)
		byAction[m["action"].(string)] = m
	}

	// expense.created: actor Alice, links to the expense, description intact.
	created := byAction["expense.created"]
	require.Equal(t, "expense", created["target_kind"])
	require.Equal(t, expenseID, created["target_id"])
	require.Equal(t, "Groceries run", created["description"])
	require.Equal(t, alice["id"], created["actor"].(map[string]any)["user_id"])

	// expense.updated: actor Bob, new category slug surfaced.
	updated := byAction["expense.updated"]
	require.Equal(t, bob["id"], updated["actor"].(map[string]any)["user_id"])
	require.Equal(t, trainSlug, updated["category_slug"])

	// expense.deleted: still resolves the (soft-deleted) description, actor Alice.
	deleted := byAction["expense.deleted"]
	require.Equal(t, "Groceries run", deleted["description"], "deleted event must keep the description")
	require.Equal(t, alice["id"], deleted["actor"].(map[string]any)["user_id"])

	// settlement.created: actor Bob, note becomes the description, from/to
	// surfaced so the feed can render "Bob paid Alice".
	sCreated := byAction["settlement.created"]
	require.Equal(t, "settlement", sCreated["target_kind"])
	require.Equal(t, "payback", sCreated["description"])
	require.Equal(t, bob["id"], sCreated["actor"].(map[string]any)["user_id"])
	require.Equal(t, bob["id"], sCreated["from_user_id"])
	require.Equal(t, alice["id"], sCreated["to_user_id"])

	// settlement.updated: actor Alice.
	require.Equal(t, alice["id"], byAction["settlement.updated"]["actor"].(map[string]any)["user_id"])
	// settlement.deleted: still resolves, actor Bob.
	require.Equal(t, bob["id"], byAction["settlement.deleted"]["actor"].(map[string]any)["user_id"])

	require.Nil(t, body["next_cursor"], "six items fit in one default page")
}

// TestActivityRecurringAndPagination covers the worker-generated recurring
// flag (actor omitted, recurring=true) and cursor pagination with limit=1,
// plus the bad-cursor and non-member rejections.
func TestActivityRecurringAndPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	alice, cookieA := registerUser(t, base, "actr-a@test.dev", "passwordpassword", "Alice")
	bob, _ := registerUser(t, base, "actr-b@test.dev", "passwordpassword", "Bob")

	resp, group := request(t, "POST", base+"/v1/groups", map[string]any{
		"name": "Recur", "default_currency": "EUR",
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	groupID := group["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/members",
		map[string]any{"email": "actr-b@test.dev"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, cats := requestList(t, "GET", base+"/v1/categories", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	categoryID := cats[0]["id"].(string)

	// A recurring template due in the past, then run the worker tick.
	dueAt := time.Now().Add(-30 * time.Second).UTC()
	resp, tmpl := request(t, "POST", base+"/v1/groups/"+groupID+"/recurring-expenses", map[string]any{
		"payer_id":     alice["id"],
		"category_id":  categoryID,
		"amount_cents": 1000,
		"currency":     "EUR",
		"description":  "Rent",
		"mode":         "equal",
		"splits": []map[string]any{
			{"user_id": alice["id"]}, {"user_id": bob["id"]},
		},
		"cadence":     "monthly",
		"next_run_at": dueAt.Format(time.RFC3339),
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, tmpl)

	created, err := ts.recurringSvc.Tick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, created)

	items, _ := activityItems(t, base, groupID, "", cookieA)
	require.Len(t, items, 1, "the materialized expense is the only activity event")
	ev := items[0].(map[string]any)
	require.Equal(t, "expense.created", ev["action"])
	require.Equal(t, true, ev["recurring"], "worker-generated expense must be flagged recurring")
	require.Nil(t, ev["actor"], "worker rows have no actor")
	require.Equal(t, "Rent", ev["description"])

	// Add two manual expenses so there are 3 events to paginate.
	for i := 0; i < 2; i++ {
		resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/expenses", map[string]any{
			"description":  "Manual",
			"amount_cents": 500,
			"payer_id":     alice["id"],
			"mode":         "equal",
			"splits":       []map[string]any{{"user_id": alice["id"]}, {"user_id": bob["id"]}},
		}, cookieA)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	// Paginate one at a time and assert no overlap, draining all 3.
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 3; page++ {
		q := "limit=1"
		if cursor != "" {
			q += "&cursor=" + cursor
		}
		pageItems, body := activityItems(t, base, groupID, q, cookieA)
		require.Len(t, pageItems, 1)
		id := pageItems[0].(map[string]any)["id"].(string)
		require.False(t, seen[id], "duplicate event across pages")
		seen[id] = true
		if page < 2 {
			require.NotNil(t, body["next_cursor"])
			cursor = body["next_cursor"].(string)
		} else {
			require.Nil(t, body["next_cursor"], "last page has no next_cursor")
		}
	}
	require.Len(t, seen, 3)

	// Bad cursor → 400.
	resp, _ = request(t, "GET", base+"/v1/groups/"+groupID+"/activity?cursor=not-a-cursor", nil, cookieA)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Non-member → 403.
	_, cookieC := registerUser(t, base, "actr-c@test.dev", "passwordpassword", "Carol")
	resp, _ = request(t, "GET", base+"/v1/groups/"+groupID+"/activity", nil, cookieC)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}
