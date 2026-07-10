package sqlite

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

type AuditRepo struct{ s *Store }

// Insert records an audit row. tx may be nil to use the pool, or a repo.Tx to
// participate in an existing transaction (so the audit row is committed
// atomically with the action it describes).
func (r *AuditRepo) Insert(ctx context.Context, tx repo.Tx, e *repo.AuditEntry) error {
	q := r.s.resolve(tx)
	e.ID = uuid.New()
	e.CreatedAt = time.Now().UTC()
	var meta any
	if len(e.Metadata) > 0 {
		meta = string(e.Metadata)
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO admin_audit
			(id, actor_user_id, target_user_id, target_group_id, action, ip, user_agent, success, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.ID, e.ActorUserID, e.TargetUserID, e.TargetGroupID, e.Action, e.IP, e.UserAgent, e.Success, meta, tsVal(e.CreatedAt))
	return err
}

// List returns paginated audit entries newest first.
func (r *AuditRepo) List(ctx context.Context, f repo.AuditFilter, limit, offset int) ([]repo.AuditEntry, int, error) {
	args := []any{}
	where := ""
	if f.Action != "" {
		args = append(args, f.Action)
		where = "WHERE action = ?"
	}
	var total int
	if err := r.s.db.QueryRowContext(ctx, `SELECT count(*) FROM admin_audit `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := r.s.db.QueryContext(ctx, `
		SELECT id, actor_user_id, target_user_id, target_group_id, action, ip, user_agent, success, metadata, created_at
		FROM admin_audit
		`+where+`
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []repo.AuditEntry
	for rows.Next() {
		var e repo.AuditEntry
		var meta *string
		var created string
		if err := rows.Scan(&e.ID, &e.ActorUserID, &e.TargetUserID, &e.TargetGroupID, &e.Action,
			&e.IP, &e.UserAgent, &e.Success, &meta, &created); err != nil {
			return nil, 0, err
		}
		if meta != nil && *meta != "" {
			e.Metadata = json.RawMessage(*meta)
		}
		e.CreatedAt = scanTS(created)
		out = append(out, e)
	}
	return out, total, rows.Err()
}
