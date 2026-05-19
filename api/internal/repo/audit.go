package repo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditEntry struct {
	ID            uuid.UUID
	ActorUserID   uuid.UUID
	TargetUserID  *uuid.UUID
	TargetGroupID *uuid.UUID
	Action        string
	IP            *string
	UserAgent     *string
	Success       bool
	Metadata      json.RawMessage
	CreatedAt     time.Time
}

type AuditRepo struct {
	pool *pgxpool.Pool
}

func NewAuditRepo(p *pgxpool.Pool) *AuditRepo { return &AuditRepo{pool: p} }

// Insert records an audit row. q may be nil to use the pool, or a pgx.Tx to
// participate in an existing transaction (so the audit row is committed
// atomically with the action it describes).
func (r *AuditRepo) Insert(ctx context.Context, q Querier, e *AuditEntry) error {
	if q == nil {
		q = poolQuerier{r.pool}
	}
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

// AuditFilter narrows a List query.
type AuditFilter struct {
	Action string
}

// List returns paginated audit entries newest first.
func (r *AuditRepo) List(ctx context.Context, f AuditFilter, limit, offset int) ([]AuditEntry, int, error) {
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
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
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
