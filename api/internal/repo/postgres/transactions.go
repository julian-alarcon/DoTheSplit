package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type TransactionRepo struct{ pool *pgxpool.Pool }

// ListByGroup returns up to `limit` (occurred_at DESC, created_at DESC, id DESC)
// rows from the merged expenses + settlements feed for the group, optionally
// continuing strictly after the given cursor row. Soft-deleted rows are
// excluded. Caller passes `limit + 1` if it wants to detect a next page.
func (r *TransactionRepo) ListByGroup(ctx context.Context, groupID uuid.UUID, limit int, after *repo.TransactionRow) ([]repo.TransactionRow, error) {
	args := []any{groupID}
	cursorPredicate := ""
	if after != nil {
		// Lexicographic comparison on the (occurred_at, created_at, id) tuple,
		// strictly less than the cursor (we already returned that row).
		args = append(args, after.OccurredAt, after.CreatedAt, after.ID)
		cursorPredicate = "AND (occurred_at, created_at, id) < ($2, $3, $4)"
	}
	limitArg := len(args) + 1
	args = append(args, limit)
	query := fmt.Sprintf(`
		SELECT kind, occurred_at, created_at, id FROM (
			SELECT 'expense'::text AS kind, incurred_at AS occurred_at, created_at, id
			FROM expenses
			WHERE group_id = $1 AND deleted_at IS NULL
			UNION ALL
			SELECT 'settlement'::text AS kind, settled_at AS occurred_at, created_at, id
			FROM settlements
			WHERE group_id = $1 AND deleted_at IS NULL
		) feed
		WHERE TRUE %s
		ORDER BY occurred_at DESC, created_at DESC, id DESC
		LIMIT $%d
	`, cursorPredicate, limitArg)
	query = strings.TrimSpace(query)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.TransactionRow
	for rows.Next() {
		var row repo.TransactionRow
		var kind string
		if err := rows.Scan(&kind, &row.OccurredAt, &row.CreatedAt, &row.ID); err != nil {
			return nil, err
		}
		row.Kind = repo.TransactionKind(kind)
		out = append(out, row)
	}
	return out, rows.Err()
}
