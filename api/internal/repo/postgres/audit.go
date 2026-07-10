package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type AuditRepo struct{ pool *pgxpool.Pool }

// Insert records an audit row. tx may be nil to use the pool, or a repo.Tx to
// participate in an existing transaction (so the audit row is committed
// atomically with the action it describes).
func (r *AuditRepo) Insert(ctx context.Context, tx repo.Tx, e *repo.AuditEntry) error {
	q := resolve(r.pool, tx)
	var meta any
	if len(e.Metadata) > 0 {
		meta = []byte(e.Metadata)
	}
	return q.QueryRow(ctx, `
		INSERT INTO admin_audit
			(actor_user_id, target_user_id, target_group_id, action, ip, user_agent, success, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`, e.ActorUserID, e.TargetUserID, e.TargetGroupID, e.Action, e.IP, e.UserAgent, e.Success, meta).
		Scan(&e.ID, &e.CreatedAt)
}

// List returns paginated audit entries newest first.
func (r *AuditRepo) List(ctx context.Context, f repo.AuditFilter, limit, offset int) ([]repo.AuditEntry, int, error) {
	args := []any{}
	where := ""
	if f.Action != "" {
		args = append(args, f.Action)
		where = "WHERE action = $1"
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM admin_audit `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, `
		SELECT id, actor_user_id, target_user_id, target_group_id, action, ip, user_agent, success, metadata, created_at
		FROM admin_audit
		`+where+`
		ORDER BY created_at DESC
		LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args))+`
	`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []repo.AuditEntry
	for rows.Next() {
		var e repo.AuditEntry
		var meta []byte
		if err := rows.Scan(&e.ID, &e.ActorUserID, &e.TargetUserID, &e.TargetGroupID, &e.Action,
			&e.IP, &e.UserAgent, &e.Success, &meta, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		if len(meta) > 0 {
			e.Metadata = json.RawMessage(meta)
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func itoa(n int) string {
	// strconv-free for tiny positive ints used in SQL placeholder construction.
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
