package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type SearchRepo struct{ pool *pgxpool.Pool }

// SearchTransactions returns up to `limit` non-deleted expense+settlement rows in
// `groupIDs` whose description/notes (expense) or note (settlement) contain
// `q` as a case-insensitive substring. Caller is responsible for restricting
// `groupIDs` to groups the actor belongs to.
//
// q is matched with ILIKE %q% on the underlying TEXT columns; the `%` and
// `_` wildcards in the input are escaped so the user can't injection-break
// the match.
//
// When `categoryID` is non-nil, the result is restricted to expenses with that
// category id, and settlements are excluded entirely (settlements have no
// category).
func (r *SearchRepo) SearchTransactions(ctx context.Context, groupIDs []uuid.UUID, q string, categoryID *uuid.UUID, limit int) ([]repo.SearchRow, error) {
	if len(groupIDs) == 0 || strings.TrimSpace(q) == "" {
		return nil, nil
	}
	pattern := "%" + escapeLike(q) + "%"
	var (
		query string
		args  []any
	)
	if categoryID != nil {
		query = `
			SELECT 'expense'::text AS kind, group_id, incurred_at AS occurred_at, created_at, id
			FROM expenses
			WHERE group_id = ANY($1)
			  AND deleted_at IS NULL
			  AND category_id = $4
			  AND (description ILIKE $2 ESCAPE '\' OR notes ILIKE $2 ESCAPE '\')
			ORDER BY occurred_at DESC, created_at DESC, id DESC
			LIMIT $3
		`
		args = []any{groupIDs, pattern, limit, *categoryID}
	} else {
		query = `
			SELECT kind, group_id, occurred_at, created_at, id FROM (
				SELECT 'expense'::text AS kind, group_id, incurred_at AS occurred_at, created_at, id
				FROM expenses
				WHERE group_id = ANY($1)
				  AND deleted_at IS NULL
				  AND (description ILIKE $2 ESCAPE '\' OR notes ILIKE $2 ESCAPE '\')
				UNION ALL
				SELECT 'settlement'::text AS kind, group_id, settled_at AS occurred_at, created_at, id
				FROM settlements
				WHERE group_id = ANY($1)
				  AND deleted_at IS NULL
				  AND note ILIKE $2 ESCAPE '\'
			) hits
			ORDER BY occurred_at DESC, created_at DESC, id DESC
			LIMIT $3
		`
		args = []any{groupIDs, pattern, limit}
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search transaction: %w", err)
	}
	defer rows.Close()
	var out []repo.SearchRow
	for rows.Next() {
		var row repo.SearchRow
		var kind string
		if err := rows.Scan(&kind, &row.GroupID, &row.OccurredAt, &row.CreatedAt, &row.ID); err != nil {
			return nil, err
		}
		row.Kind = repo.TransactionKind(kind)
		out = append(out, row)
	}
	return out, rows.Err()
}

// AvailableCategories returns the set of distinct category_ids on non-deleted
// expenses in `groupIDs` whose description/notes match `q` (case-insensitive
// substring). The active category_id filter is *not* applied here on purpose -
// the client uses this list to populate its category filter picker, so it
// must reflect every category the user could switch to within the current
// q + group scope.
func (r *SearchRepo) AvailableCategories(ctx context.Context, groupIDs []uuid.UUID, q string) ([]uuid.UUID, error) {
	if len(groupIDs) == 0 || strings.TrimSpace(q) == "" {
		return nil, nil
	}
	pattern := "%" + escapeLike(q) + "%"
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT category_id
		FROM expenses
		WHERE group_id = ANY($1)
		  AND deleted_at IS NULL
		  AND (description ILIKE $2 ESCAPE '\' OR notes ILIKE $2 ESCAPE '\')
	`, groupIDs, pattern)
	if err != nil {
		return nil, fmt.Errorf("available categories: %w", err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// escapeLike turns a free-text query into a literal pattern. We register a
// custom ESCAPE character (\) at the SQL site so a stray % or _ in the user's
// query matches literally instead of acting as a wildcard.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
