package server_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSearchEndpoint covers the cross-group /v1/search contract: substring
// match in description and notes (expenses) plus note (settlements), result
// ordering across groups, the optional group_id filter, soft-delete exclusion,
// non-member groups silently filtered out, the min-length 400, and the
// optional category_id filter (which restricts to expenses in that category
// and excludes settlements entirely).
func TestSearchEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	userA, cookieA := registerUser(t, base, "search-a@test.dev", "passwordpassword", "Alice")
	userB, cookieB := registerUser(t, base, "search-b@test.dev", "passwordpassword", "Bob")
	_, cookieC := registerUser(t, base, "search-c@test.dev", "passwordpassword", "Carol")
	_ = userB

	// Two groups Alice belongs to. Carol is in neither.
	resp, gA := request(t, "POST", base+"/v1/groups",
		map[string]any{"name": "GroupAlpha"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	groupA := gA["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupA+"/members",
		map[string]any{"email": "search-b@test.dev"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, gB := request(t, "POST", base+"/v1/groups",
		map[string]any{"name": "GroupBeta"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	groupB := gB["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupB+"/members",
		map[string]any{"email": "search-b@test.dev"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// A group Alice does NOT belong to (Carol owns it). Used to verify a stray
	// group_id from another instance is silently ignored, not an error.
	resp, gC := request(t, "POST", base+"/v1/groups",
		map[string]any{"name": "GroupCarol"}, cookieC)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	groupC := gC["id"].(string)

	t0 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	mkExpense := func(group, cookieDescription, payerID, desc, notes string, amt int64, off int) string {
		body := map[string]any{
			"description":  desc,
			"amount_cents": amt,
			"payer_id":     payerID,
			"mode":         "equal",
			"incurred_at":  t0.Add(time.Duration(off) * time.Hour).Format(time.RFC3339Nano),
			"splits": []map[string]any{
				{"user_id": userA["id"]}, {"user_id": userB["id"]},
			},
		}
		if notes != "" {
			body["notes"] = notes
		}
		var c = cookieA
		if cookieDescription == "B" {
			c = cookieB
		}
		resp, e := request(t, "POST", base+"/v1/groups/"+group+"/expenses", body, c)
		require.Equal(t, http.StatusCreated, resp.StatusCode, e)
		return e["id"].(string)
	}

	// GroupAlpha: 2 hits for "pizza" (one via description, one via notes).
	pizzaDesc := mkExpense(groupA, "A", userA["id"].(string), "Pizza night",  "", 1000, 1)
	pizzaNote := mkExpense(groupA, "A", userA["id"].(string), "Friday dinner", "Pizza for everyone", 2000, 2)
	// A non-matching expense to make sure we filter on the substring.
	_ = mkExpense(groupA, "A", userA["id"].(string), "Train tickets", "", 500, 3)

	// GroupBeta: 1 hit for "pizza" (in description) + 1 settlement note hit.
	pizzaB := mkExpense(groupB, "A", userA["id"].(string), "Stone-baked pizza", "", 1500, 4)
	resp, sBody := request(t, "POST", base+"/v1/groups/"+groupB+"/settlements", map[string]any{
		"to_user_id":   userA["id"],
		"amount_cents": 700,
		"note":         "PIZZA tab leftovers",
		"settled_at":   t0.Add(5 * time.Hour).Format(time.RFC3339Nano),
	}, cookieB)
	require.Equal(t, http.StatusCreated, resp.StatusCode, sBody)
	settlePizza := sBody["id"].(string)

	// A non-matching settlement.
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupB+"/settlements", map[string]any{
		"to_user_id":   userA["id"],
		"amount_cents": 200,
		"note":         "rent",
		"settled_at":   t0.Add(6 * time.Hour).Format(time.RFC3339Nano),
	}, cookieB)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// A soft-deleted expense that *would* match, to confirm exclusion.
	deleted := mkExpense(groupA, "A", userA["id"].(string), "Pizza takeout (cancelled)", "", 999, 7)
	resp, _ = request(t, "DELETE", base+"/v1/expenses/"+deleted, nil, cookieA)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	collectIDs := func(items []any) (expIDs, setIDs []string) {
		for _, raw := range items {
			it := raw.(map[string]any)
			if e, ok := it["expense"].(map[string]any); ok {
				expIDs = append(expIDs, e["id"].(string))
			}
			if s, ok := it["settlement"].(map[string]any); ok {
				setIDs = append(setIDs, s["id"].(string))
			}
		}
		return
	}

	// 1) Cross-group, no filter: 4 hits (3 expenses + 1 settlement).
	resp, body := request(t, "GET", base+"/v1/search?q=pizza", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	items := body["items"].([]any)
	require.Len(t, items, 4)
	expIDs, setIDs := collectIDs(items)
	require.ElementsMatch(t, []string{pizzaDesc, pizzaNote, pizzaB}, expIDs)
	require.ElementsMatch(t, []string{settlePizza}, setIDs)
	// Ordering: occurred_at descending across groups. settlePizza (off=5) is
	// newest, pizzaB (off=4) next, pizzaNote (off=2), pizzaDesc (off=1) last.
	gotIDs := make([]string, 0, len(items))
	for _, raw := range items {
		it := raw.(map[string]any)
		if e, ok := it["expense"].(map[string]any); ok {
			gotIDs = append(gotIDs, e["id"].(string))
		} else if s, ok := it["settlement"].(map[string]any); ok {
			gotIDs = append(gotIDs, s["id"].(string))
		}
	}
	require.Equal(t, []string{settlePizza, pizzaB, pizzaNote, pizzaDesc}, gotIDs)
	// Groups field returns both Alice's groups (member-only).
	gs := body["groups"].([]any)
	gotGroups := map[string]bool{}
	for _, g := range gs {
		gotGroups[g.(map[string]any)["id"].(string)] = true
	}
	require.Equal(t, map[string]bool{groupA: true, groupB: true}, gotGroups)

	// 2) group_id filter narrows to GroupBeta.
	u := base + "/v1/search?q=pizza&group_id=" + url.QueryEscape(groupB)
	resp, body = request(t, "GET", u, nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	items = body["items"].([]any)
	require.Len(t, items, 2)
	expIDs, setIDs = collectIDs(items)
	require.ElementsMatch(t, []string{pizzaB}, expIDs)
	require.ElementsMatch(t, []string{settlePizza}, setIDs)

	// 3) Stray group_id the actor doesn't belong to is silently ignored: when
	// they pass *only* the foreign id, the effective set is empty, which
	// returns an empty result without 4xx.
	u = base + "/v1/search?q=pizza&group_id=" + url.QueryEscape(groupC)
	resp, body = request(t, "GET", u, nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, body["items"])
	require.Empty(t, body["groups"])

	// 4) Carol can't see Alice's hits at all. She is in groupC only, which has
	// no expenses, so the feed is empty even with a generic substring.
	resp, body = request(t, "GET", base+"/v1/search?q=pizza", nil, cookieC)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, body["items"])

	// 5) Min-length: q < 2 chars is a 400, not a 200 with everything.
	resp, _ = request(t, "GET", base+"/v1/search?q=p", nil, cookieA)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// 6) ILIKE wildcards are escaped: a literal "%" in the query should match
	// nothing rather than acting as a wildcard.
	resp, body = request(t, "GET", base+"/v1/search?q="+url.QueryEscape("%%"), nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, body["items"])

	// --- category_id filter ---
	// Pick two distinct categories to assign to fresh pizza expenses, then
	// verify category_id narrows to the matching expense and excludes the
	// "PIZZA tab leftovers" settlement.
	resp, cats := requestList(t, "GET", base+"/v1/categories", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var groceriesID, trainID, otherID string
	for _, c := range cats {
		switch c["slug"] {
		case "groceries":
			groceriesID = c["id"].(string)
		case "train":
			trainID = c["id"].(string)
		case "other":
			otherID = c["id"].(string)
		}
	}
	require.NotEmpty(t, groceriesID)
	require.NotEmpty(t, trainID)
	require.NotEmpty(t, otherID)

	mkExpenseCat := func(group, payerID, desc string, amt int64, off int, catID string) string {
		body := map[string]any{
			"description":  desc,
			"amount_cents": amt,
			"payer_id":     payerID,
			"category_id":  catID,
			"mode":         "equal",
			"incurred_at":  t0.Add(time.Duration(off) * time.Hour).Format(time.RFC3339Nano),
			"splits": []map[string]any{
				{"user_id": userA["id"]}, {"user_id": userB["id"]},
			},
		}
		resp, e := request(t, "POST", base+"/v1/groups/"+group+"/expenses", body, cookieA)
		require.Equal(t, http.StatusCreated, resp.StatusCode, e)
		return e["id"].(string)
	}
	pizzaGroceries := mkExpenseCat(groupA, userA["id"].(string), "Pizza grocery run", 1100, 10, groceriesID)
	pizzaTrain := mkExpenseCat(groupB, userA["id"].(string), "Pizza on the train", 1200, 11, trainID)

	// 7) category_id narrows to expenses in that category; settlement is
	// excluded even though its note matches the substring.
	u = base + "/v1/search?q=pizza&category_id=" + url.QueryEscape(groceriesID)
	resp, body = request(t, "GET", u, nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	expIDs, setIDs = collectIDs(body["items"].([]any))
	require.ElementsMatch(t, []string{pizzaGroceries}, expIDs)
	require.Empty(t, setIDs, "settlements must be excluded when category_id is set")

	// 8) Unknown category id silently yields zero matches.
	u = base + "/v1/search?q=pizza&category_id=" + url.QueryEscape("00000000-0000-0000-0000-000000000001")
	resp, body = request(t, "GET", u, nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, body["items"])

	// 9) q + group_id + category_id intersect: only pizzaTrain (in GroupBeta
	// AND tagged 'train') survives.
	u = base + "/v1/search?q=pizza" +
		"&group_id=" + url.QueryEscape(groupB) +
		"&category_id=" + url.QueryEscape(trainID)
	resp, body = request(t, "GET", u, nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	expIDs, setIDs = collectIDs(body["items"].([]any))
	require.ElementsMatch(t, []string{pizzaTrain}, expIDs)
	require.Empty(t, setIDs)

	// 10) Malformed category_id is a 400 (not silently dropped) - matches the
	// group_id parsing convention in the same handler.
	resp, _ = request(t, "GET", base+"/v1/search?q=pizza&category_id=not-a-uuid", nil, cookieA)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// --- available_category_ids ---
	// The set of distinct categories present among matching expenses must be
	// returned so the client can hide categories with no hits from the
	// category picker. The original "Pizza night", "Friday dinner", "Stone-
	// baked pizza" expenses were created without an explicit category, so
	// they default to "other"; pizzaGroceries → groceries; pizzaTrain → train.
	collectAvailable := func(body map[string]any) []string {
		raw, _ := body["available_category_ids"].([]any)
		out := make([]string, 0, len(raw))
		for _, x := range raw {
			out = append(out, x.(string))
		}
		return out
	}

	// 11) No category filter: all three categories in the q-matching set are
	// reported.
	resp, body = request(t, "GET", base+"/v1/search?q=pizza", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.ElementsMatch(t, []string{otherID, groceriesID, trainID}, collectAvailable(body))

	// 12) `available_category_ids` is INDEPENDENT of the active category
	// filter - the client needs to know what other categories the user could
	// switch to even after they've narrowed.
	u = base + "/v1/search?q=pizza&category_id=" + url.QueryEscape(groceriesID)
	resp, body = request(t, "GET", u, nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.ElementsMatch(t, []string{otherID, groceriesID, trainID}, collectAvailable(body))

	// 13) But it DOES respect the group_id filter: GroupBeta only contains
	// pizzaB (other) and pizzaTrain (train); groceries is GroupAlpha-only.
	u = base + "/v1/search?q=pizza&group_id=" + url.QueryEscape(groupB)
	resp, body = request(t, "GET", u, nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.ElementsMatch(t, []string{otherID, trainID}, collectAvailable(body))

	// 14) A query with no matches returns an empty list (not null).
	resp, body = request(t, "GET", base+"/v1/search?q=zzznomatchzzz", nil, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, collectAvailable(body))
	require.NotNil(t, body["available_category_ids"])
}
