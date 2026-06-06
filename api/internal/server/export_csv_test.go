package server_test

import (
	"bytes"
	"encoding/csv"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// downloadCSV issues an authenticated GET to the export endpoint and
// returns the parsed CSV. Helper because the endpoint streams text/csv
// rather than JSON, so the standard request() helper does not fit.
func downloadCSV(t *testing.T, base, groupID string, cookie *http.Cookie) (*http.Response, [][]string, []byte) {
	t.Helper()
	req, err := http.NewRequest("GET", base+"/v1/groups/"+groupID+"/export.csv", nil)
	require.NoError(t, err)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := testHTTPClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp, nil, body
	}
	r := csv.NewReader(bytes.NewReader(body))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	require.NoError(t, err)
	return resp, rows, body
}

func TestExportGroupCSV_GoldenPath(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	userA, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")
	userB, _ := registerUser(t, base, "bob@example.com", "passwordpassword", "Bob")

	resp, group := request(t, "POST", base+"/v1/groups",
		map[string]any{"name": "Trip"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	gid := group["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+gid+"/members",
		map[string]any{"email": "bob@example.com"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Two expenses, each split equally, plus a partial settlement.
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

	resp, rows, _ := downloadCSV(t, base, gid, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/csv; charset=utf-8", resp.Header.Get("Content-Type"))
	cd := resp.Header.Get("Content-Disposition")
	require.Contains(t, cd, "attachment")
	require.Contains(t, cd, "trip_")
	require.Contains(t, cd, "_export.csv")

	require.GreaterOrEqual(t, len(rows), 4) // header + 2 expenses + 1 settlement + footer
	require.Equal(t,
		[]string{"Date", "Description", "Category", "Cost", "Currency", "Time", "Payer", "Notes", "Created", "CreatedBy", "Alice", "Bob"},
		rows[0],
	)

	body := rows[1 : len(rows)-1]
	require.Len(t, body, 3)

	hotel := body[0]
	require.Equal(t, "Hotel", hotel[1])
	require.Equal(t, "200.00", hotel[3])
	require.Equal(t, "EUR", hotel[4])
	require.Equal(t, "Alice", hotel[6])
	require.Equal(t, "good view", hotel[7])
	require.Equal(t, "100.00", hotel[10])
	require.Equal(t, "-100.00", hotel[11])

	coffee := body[1]
	require.Equal(t, "Coffee", coffee[1])
	require.Equal(t, "6.00", coffee[3])
	require.Equal(t, "Bob", coffee[6])
	require.Equal(t, "-3.00", coffee[10])
	require.Equal(t, "3.00", coffee[11])

	settle := body[2]
	require.Equal(t, "partial", settle[1])
	require.Equal(t, "Payment", settle[2])
	require.Equal(t, "50.00", settle[3])
	require.Equal(t, "Bob", settle[6])
	// from-user (Bob) gains the amount; to-user (Alice) loses it.
	require.Equal(t, "-50.00", settle[10])
	require.Equal(t, "50.00", settle[11])

	footer := rows[len(rows)-1]
	require.Equal(t, "Total balance", footer[1])
	// Alice net = +100 - 3 - 50 = +47. Bob = mirror.
	require.Equal(t, "47.00", footer[10])
	require.Equal(t, "-47.00", footer[11])
}

func TestExportGroupCSV_NotMemberForbidden(t *testing.T) {
	ts := setup(t)
	base := ts.srv.URL

	_, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")
	_, cookieC := registerUser(t, base, "carol@example.com", "passwordpassword", "Carol")
	_, _ = registerUser(t, base, "bob@example.com", "passwordpassword", "Bob")

	resp, group := request(t, "POST", base+"/v1/groups",
		map[string]any{"name": "Trip"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode, group)
	gid := group["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+gid+"/members",
		map[string]any{"email": "bob@example.com"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, _, _ = downloadCSV(t, base, gid, cookieC)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestExportGroupCSV_HeaderShape(t *testing.T) {
	// The export must keep the strict 5-column Splitwise prefix, then
	// the optional named band, then the per-member columns. This locks
	// the contract that the legacy Splitwise importer relies on (it
	// skips the optional middle band and treats only the trailing
	// columns as users).
	ts := setup(t)
	base := ts.srv.URL
	_, cookieA := registerUser(t, base, "alice@example.com", "passwordpassword", "Alice")
	_, _ = registerUser(t, base, "bob@example.com", "passwordpassword", "Bob")
	resp, group := request(t, "POST", base+"/v1/groups",
		map[string]any{"name": "Trip"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	gid := group["id"].(string)
	resp, _ = request(t, "POST", base+"/v1/groups/"+gid+"/members",
		map[string]any{"email": "bob@example.com"}, cookieA)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, rows, raw := downloadCSV(t, base, gid, cookieA)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	for i, want := range []string{"Date", "Description", "Category", "Cost", "Currency"} {
		require.Equal(t, want, rows[0][i])
	}
	for _, name := range []string{"Time", "Payer", "Notes", "Created", "CreatedBy"} {
		require.Contains(t, strings.Join(rows[0], ","), name, name)
	}
	// The body always ends with a Total balance footer regardless of
	// whether there is any expense data.
	require.Contains(t, string(raw), "Total balance")
}
