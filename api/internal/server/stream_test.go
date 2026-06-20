package server_test

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestActivityStream verifies the SSE endpoint pushes an event to a connected
// group member when another member creates an expense. It also exercises the
// non-member 403 (the membership gate runs before any stream bytes).
func TestActivityStream(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	userA, cookieA := registerUser(t, base, "stream-a@test.dev", "passwordpassword", "StreamA")
	_, cookieB := registerUser(t, base, "stream-b@test.dev", "passwordpassword", "StreamB")

	resp, gBody := request(t, "POST", base+"/v1/groups", map[string]any{"name": "StreamGroup"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, gBody)
	groupID := gBody["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/members",
		map[string]any{"email": "stream-b@test.dev"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// --- Non-member is rejected with 403 before any stream output. ---
	_, cookieC := registerUser(t, base, "stream-c@test.dev", "passwordpassword", "StreamC")
	t.Run("non_member_forbidden", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "GET", base+"/v1/groups/"+groupID+"/events", nil)
		require.NoError(t, err)
		applyAuth(req, cookieC)
		r, err := testHTTPClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = r.Body.Close() }()
		require.Equal(t, http.StatusForbidden, r.StatusCode)
	})

	// --- Member B opens the stream; A creates an expense; B receives it. ---
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/v1/groups/"+groupID+"/events", nil)
	require.NoError(t, err)
	applyAuth(req, cookieB)
	streamResp, err := testHTTPClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = streamResp.Body.Close() }()
	require.Equal(t, http.StatusOK, streamResp.StatusCode)
	require.Contains(t, streamResp.Header.Get("Content-Type"), "text/event-stream")

	reader := bufio.NewReader(streamResp.Body)

	// First read the initial ": connected" comment so we know the subscription
	// is registered (and the LISTEN is live) before A inserts the row, avoiding
	// a race where the notify fires before B is subscribed.
	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Contains(t, line, ": connected")

	// A creates an expense in a separate goroutine (the read below blocks).
	go func() {
		_, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/expenses", map[string]any{
			"description":  "StreamedExpense",
			"amount_cents": 1000,
			"payer_id":     userA["id"],
			"mode":         "equal",
			"splits":       []map[string]any{{"user_id": userA["id"]}},
		}, cookieA)
	}()

	// Read frames until we see the activity data line (skipping the blank line
	// after ": connected" and any heartbeat). The context timeout guards a hang.
	var got string
	for {
		l, rerr := reader.ReadString('\n')
		require.NoError(t, rerr)
		if strings.HasPrefix(l, "data:") {
			got = l
			break
		}
	}
	cancel() // close the stream now that we have our frame
	require.Contains(t, got, "expense.created")
	require.Contains(t, got, groupID)
}

// TestUnreadActivityCount verifies the per-member unread badge: another
// member's action bumps unread_count, the member's own action does not, and
// marking read zeroes it.
func TestUnreadActivityCount(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs Docker/testcontainers")
	}
	ts := setup(t)
	base := ts.srv.URL

	userA, cookieA := registerUser(t, base, "unread-a@test.dev", "passwordpassword", "UnreadA")
	_, cookieB := registerUser(t, base, "unread-b@test.dev", "passwordpassword", "UnreadB")

	resp, gBody := request(t, "POST", base+"/v1/groups", map[string]any{"name": "UnreadGroup"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, gBody)
	groupID := gBody["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/members",
		map[string]any{"email": "unread-b@test.dev"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// A creates an expense. From B's perspective that's 1 unread; from A's own
	// perspective it's 0 (own actions never badge).
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/expenses", map[string]any{
		"description":  "Lunch",
		"amount_cents": 1000,
		"payer_id":     userA["id"],
		"mode":         "equal",
		"splits":       []map[string]any{{"user_id": userA["id"]}},
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	require.EqualValues(t, 1, unreadCount(t, base, cookieB, groupID), "B should see A's action as unread")
	require.EqualValues(t, 0, unreadCount(t, base, cookieA, groupID), "A's own action must not badge")

	// B opens the activity log (marks read) → count drops to 0.
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/activity/read", nil, cookieB)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.EqualValues(t, 0, unreadCount(t, base, cookieB, groupID), "marking read should zero the count")

	// A creates another expense → B is unread again (1), A still 0.
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/expenses", map[string]any{
		"description":  "Dinner",
		"amount_cents": 2000,
		"payer_id":     userA["id"],
		"mode":         "equal",
		"splits":       []map[string]any{{"user_id": userA["id"]}},
	}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.EqualValues(t, 1, unreadCount(t, base, cookieB, groupID))
	require.EqualValues(t, 0, unreadCount(t, base, cookieA, groupID))

	// Non-member marking read → 403.
	_, cookieC := registerUser(t, base, "unread-c@test.dev", "passwordpassword", "UnreadC")
	resp, _ = request(t, "POST", base+"/v1/groups/"+groupID+"/activity/read", nil, cookieC)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// unreadCount reads the caller's unread_count for groupID off GET /v1/groups.
func unreadCount(t *testing.T, base string, cred *http.Cookie, groupID string) float64 {
	t.Helper()
	resp, groups := requestList(t, "GET", base+"/v1/groups", nil, cred)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	for _, g := range groups {
		if g["id"] == groupID {
			require.Contains(t, g, "unread_count", "Group payload must carry unread_count")
			return g["unread_count"].(float64)
		}
	}
	t.Fatalf("group %s not found in list for caller", groupID)
	return -1
}
