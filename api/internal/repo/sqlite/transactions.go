package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type TransactionRepo struct{ s *Store }

// ListByGroup returns up to `limit` (occurred_at DESC, created_at DESC, id DESC)
// rows from the merged expenses + settlements feed for the group, optionally
// continuing strictly after the given cursor row. Soft-deleted rows are
// excluded. Caller passes `limit + 1` if it wants to detect a next page.
//
// The two `group_id = ?` occurrences inside the UNION come first (bound in the
// query text order), then the optional cursor tuple, then the LIMIT.
func (r *TransactionRepo) ListByGroup(ctx context.Context, groupID uuid.UUID, limit int, after *repo.TransactionRow) ([]repo.TransactionRow, error) {
	args := []any{groupID, groupID}
	cursorPredicate := ""
	if after != nil {
		// Lexicographic comparison on the (occurred_at, created_at, id) tuple,
		// strictly less than the cursor (we already returned that row).
		args = append(args, tsVal(after.OccurredAt), tsVal(after.CreatedAt), after.ID)
		cursorPredicate = "AND (occurred_at, created_at, id) < (?, ?, ?)"
	}
	args = append(args, limit)
	query := fmt.Sprintf(`
		SELECT kind, occurred_at, created_at, id FROM (
			SELECT 'expense' AS kind, incurred_at AS occurred_at, created_at, id
			FROM expenses
			WHERE group_id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT 'settlement' AS kind, settled_at AS occurred_at, created_at, id
			FROM settlements
			WHERE group_id = ? AND deleted_at IS NULL
		) feed
		WHERE TRUE %s
		ORDER BY occurred_at DESC, created_at DESC, id DESC
		LIMIT ?
	`, cursorPredicate)
	query = strings.TrimSpace(query)
	rows, err := r.s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.TransactionRow
	for rows.Next() {
		var row repo.TransactionRow
		var kind, occurred, created string
		if err := rows.Scan(&kind, &occurred, &created, &row.ID); err != nil {
			return nil, err
		}
		row.Kind = repo.TransactionKind(kind)
		row.OccurredAt = scanTS(occurred)
		row.CreatedAt = scanTS(created)
		out = append(out, row)
	}
	return out, rows.Err()
}
