package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SearchRow is a hit in the merged expenses+settlements feed, restricted to
// rows in the supplied groups whose searchable text contains the query
// substring (case-insensitive). The service hydrates full payloads via the
// existing FindByIDs helpers.
type SearchRow struct {
	Kind       ActivityKind
	GroupID    uuid.UUID
	OccurredAt time.Time
	CreatedAt  time.Time
	ID         uuid.UUID
}

type SearchRepo struct {
	pool *pgxpool.Pool
}

func NewSearchRepo(p *pgxpool.Pool) *SearchRepo { return &SearchRepo{pool: p} }

// SearchActivity returns up to `limit` non-deleted expense+settlement rows in
// `groupIDs` whose description/notes (expense) or note (settlement) contain
// `q` as a case-insensitive substring. Caller is responsible for restricting
// `groupIDs` to groups the actor belongs to.
//
// q is matched with ILIKE %q% on the underlying TEXT columns; the `%` and
// `_` wildcards in the input are escaped so the user can't injection-break
// the match.
func (r *SearchRepo) SearchActivity(ctx context.Context, groupIDs []uuid.UUID, q string, limit int) ([]SearchRow, error) {
	if len(groupIDs) == 0 || strings.TrimSpace(q) == "" {
		return nil, nil
	}
	pattern := "%" + escapeLike(q) + "%"
	query := `
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
	rows, err := r.pool.Query(ctx, query, groupIDs, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("search activity: %w", err)
	}
	defer rows.Close()
	var out []SearchRow
	for rows.Next() {
		var row SearchRow
		var kind string
		if err := rows.Scan(&kind, &row.GroupID, &row.OccurredAt, &row.CreatedAt, &row.ID); err != nil {
			return nil, err
		}
		row.Kind = ActivityKind(kind)
		out = append(out, row)
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
