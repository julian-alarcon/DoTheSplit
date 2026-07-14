package server_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRecurringTickMaterializesAndAdvances drives the worker's main loop:
// create a recurring template due in the past via the public API, run
// RecurringService.Tick() the way the in-process worker does, and assert
// (a) exactly one expense row appears, (b) next_run_at advances by one
// cadence step, (c) Tick() is idempotent in the same window.
func TestRecurringTickMaterializesAndAdvances(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	// Two members so an "equal" split has something to divide.
	alice, aCookie := registerUser(t, base, "alice@test.dev", "passwordpassword", "Alice")
	bob, _ := registerUser(t, base, "bob@test.dev", "passwordpassword", "Bob")

	resp, group := request(t, "POST", base+"/v1/groups", map[string]any{
		"name":             "House",
		"default_currency": "EUR",
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	groupID := group["id"].(string)

	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/members", map[string]any{
		"email": "bob@test.dev",
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	_ = bob

	// Pick any seeded category.
	resp, cats := requestList(t, "GET", base+"/v1/categories", nil, aCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, cats)
	categoryID := cats[0]["id"].(string)

	// Schedule a daily template due 30 seconds ago. Tick() filters on
	// next_run_at <= now() so anything in the past qualifies.
	dueAt := time.Now().Add(-30 * time.Second).UTC()
	resp, body := request(t, "POST", base+"/v1/groups/"+groupID+"/recurring-expenses", map[string]any{
		"payer_id":     alice["id"].(string),
		"category_id":  categoryID,
		"amount_cents": 1000,
		"currency":     "EUR",
		"description":  "Rent",
		"mode":         "equal",
		"splits": []map[string]any{
			{"user_id": alice["id"].(string)},
			{"user_id": bob["id"].(string)},
		},
		"cadence":     "daily",
		"next_run_at": dueAt.Format(time.RFC3339),
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, body)
	templateID := body["id"].(string)

	// Run the worker's tick path directly. The in-process worker calls exactly
	// this method on each tick.
	created, err := ts.recurringSvc.Tick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, created, "tick must materialize the one due template")

	// Exactly one expense exists for the group.
	resp, items := requestList(t, "GET", base+"/v1/groups/"+groupID+"/expenses", nil, aCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, items, 1, "tick materialized expense should be visible via /expenses")
	require.EqualValues(t, 1000, items[0]["amount_cents"])
	require.Equal(t, "Rent", items[0]["description"])

	// next_run_at must have advanced by exactly one cadence step (24h).
	nextRun, err := ts.raw().RecurringNextRun(context.Background(), templateID)
	require.NoError(t, err)
	advance := nextRun.Sub(dueAt)
	require.InDelta(t, 24*time.Hour, advance, float64(time.Second), "daily cadence must advance by 24h")

	// Idempotency: ticking again immediately materializes nothing because
	// next_run_at is now in the future.
	created, err = ts.recurringSvc.Tick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, created, "second tick in the same window must be a no-op")
}

// TestRecurringTickCadenceVariants asserts that each supported cadence
// advances next_run_at by the expected calendar offset. Daily/weekly/biweekly
// are exact-day arithmetic; monthly/yearly use AddDate's calendar semantics.
func TestRecurringTickCadenceVariants(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	alice, aCookie := registerUser(t, base, "alice@test.dev", "passwordpassword", "Alice")

	resp, group := request(t, "POST", base+"/v1/groups", map[string]any{
		"name":             "Solo",
		"default_currency": "EUR",
	}, aCookie)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	groupID := group["id"].(string)

	resp, cats := requestList(t, "GET", base+"/v1/categories", nil, aCookie)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	categoryID := cats[0]["id"].(string)

	// Use a fixed past date that does not span a DST transition in any
	// likely host timezone (CET/CEST shifts at end of March and end of
	// October). January→January is safely DST-stable in both hemispheres.
	due := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		cadence string
		delta   func(time.Time) time.Time
	}{
		{"daily", func(t time.Time) time.Time { return t.AddDate(0, 0, 1) }},
		{"weekly", func(t time.Time) time.Time { return t.AddDate(0, 0, 7) }},
		{"biweekly", func(t time.Time) time.Time { return t.AddDate(0, 0, 14) }},
		{"monthly", func(t time.Time) time.Time { return t.AddDate(0, 1, 0) }},
		{"yearly", func(t time.Time) time.Time { return t.AddDate(1, 0, 0) }},
	}

	for _, tc := range cases {
		t.Run(tc.cadence, func(t *testing.T) {
			resp, body := request(t, "POST", base+"/v1/groups/"+groupID+"/recurring-expenses", map[string]any{
				"payer_id":     alice["id"].(string),
				"category_id":  categoryID,
				"amount_cents": 100,
				"currency":     "EUR",
				"description":  "T-" + tc.cadence,
				"mode":         "equal",
				"splits":       []map[string]any{{"user_id": alice["id"].(string)}},
				"cadence":      tc.cadence,
				"next_run_at":  due.Format(time.RFC3339),
			}, aCookie)
			require.Equal(t, http.StatusCreated, resp.StatusCode, body)
			templateID := body["id"].(string)

			created, err := ts.recurringSvc.Tick(context.Background())
			require.NoError(t, err)
			require.GreaterOrEqual(t, created, 1)

			nextRun, err := ts.raw().RecurringNextRun(context.Background(), templateID)
			require.NoError(t, err)
			require.True(t, nextRun.UTC().Equal(tc.delta(due)),
				"%s: expected %s, got %s", tc.cadence, tc.delta(due), nextRun.UTC())
		})
	}
}
