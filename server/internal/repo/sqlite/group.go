package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

type GroupRepo struct{ s *Store }

// Create inserts the group and adds the creator as a member in its own
// transaction. Callers that already hold a transaction (e.g. the importer)
// should use CreateTx instead.
func (r *GroupRepo) Create(ctx context.Context, name, defaultCurrency string, creatorID uuid.UUID) (*repo.Group, error) {
	tx, err := r.s.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	g, err := r.createTx(ctx, native(tx).tx, name, defaultCurrency, creatorID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return g, nil
}

// CreateTx inserts the group and adds the creator as a member on the
// caller-owned transaction.
func (r *GroupRepo) CreateTx(ctx context.Context, tx repo.Tx, name, defaultCurrency string, creatorID uuid.UUID) (*repo.Group, error) {
	return r.createTx(ctx, native(tx).tx, name, defaultCurrency, creatorID)
}

func (r *GroupRepo) createTx(ctx context.Context, q dbtx, name, defaultCurrency string, creatorID uuid.UUID) (*repo.Group, error) {
	g := &repo.Group{Name: name, DefaultCurrency: defaultCurrency, CreatedBy: creatorID}
	g.ID = uuid.New()
	g.CreatedAt = time.Now().UTC()
	if _, err := q.ExecContext(ctx, `
		INSERT INTO groups (id, name, default_currency, created_by, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, g.ID, name, defaultCurrency, creatorID, tsVal(g.CreatedAt)); err != nil {
		return nil, err
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO group_members (group_id, user_id, joined_at) VALUES (?, ?, ?)
	`, g.ID, creatorID, tsVal(time.Now().UTC())); err != nil {
		return nil, err
	}
	return g, nil
}

func (r *GroupRepo) FindByID(ctx context.Context, id uuid.UUID) (*repo.Group, error) {
	var g repo.Group
	var created string
	var rawSplit []byte
	err := r.s.db.QueryRowContext(ctx, `
		SELECT id, name, default_currency, created_by, created_at, default_split FROM groups WHERE id = ?
	`, id).Scan(&g.ID, &g.Name, &g.DefaultCurrency, &g.CreatedBy, &created, &rawSplit)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g.CreatedAt = scanTS(created)
	if g.DefaultSplit, err = repo.ScanDefaultSplit(rawSplit); err != nil {
		return nil, err
	}
	return &g, nil
}

// ListForUser returns groups the user belongs to, newest first. unread_count is
// the number of activity events newer than the member's last_read_activity_at
// marker (all of them when the marker is NULL), excluding the user's own
// actions, computed in a correlated subquery so the listing stays one query.
func (r *GroupRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]repo.Group, error) {
	rows, err := r.s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.default_currency, g.created_by, g.created_at, g.default_split,
		       (SELECT count(*) FROM activity_events ae
		         WHERE ae.group_id = g.id
		           AND (m.last_read_activity_at IS NULL OR ae.created_at > m.last_read_activity_at)
		           AND (ae.actor_id IS NULL OR ae.actor_id <> m.user_id)) AS unread_count
		FROM groups g
		JOIN group_members m ON m.group_id = g.id
		WHERE m.user_id = ?
		ORDER BY g.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.Group
	for rows.Next() {
		var g repo.Group
		var created string
		var rawSplit []byte
		if err := rows.Scan(&g.ID, &g.Name, &g.DefaultCurrency, &g.CreatedBy, &created, &rawSplit, &g.UnreadCount); err != nil {
			return nil, err
		}
		g.CreatedAt = scanTS(created)
		if g.DefaultSplit, err = repo.ScanDefaultSplit(rawSplit); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// MarkActivityRead advances the member's last-read marker to now(), zeroing
// their unread_count for the group. Returns ErrNotFound if the user isn't a
// member (no membership row to update).
func (r *GroupRepo) MarkActivityRead(ctx context.Context, groupID, userID uuid.UUID) error {
	res, err := r.s.db.ExecContext(ctx, `
		UPDATE group_members SET last_read_activity_at = ?
		WHERE group_id = ? AND user_id = ?
	`, tsVal(time.Now().UTC()), groupID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

func (r *GroupRepo) Update(ctx context.Context, id uuid.UUID, in repo.UpdateInput) (*repo.Group, error) {
	if in.Name == nil && in.DefaultCurrency == nil && in.DefaultSplit == nil && in.CreatedBy == nil {
		return r.FindByID(ctx, id)
	}

	// Pre-marshal the JSON payload so we can pass nil for "leave alone" or a
	// json.RawMessage for "set value". A separate flag tells SQL whether to
	// overwrite or COALESCE.
	var splitJSON any
	splitProvided := in.DefaultSplit != nil
	if splitProvided {
		if len(*in.DefaultSplit) == 0 {
			splitJSON = nil // SQL NULL → clears the column
		} else {
			b, err := json.Marshal(*in.DefaultSplit)
			if err != nil {
				return nil, err
			}
			splitJSON = b
		}
	}

	var g repo.Group
	var created string
	var rawSplit []byte
	err := r.s.db.QueryRowContext(ctx, `
		UPDATE groups SET
			name             = COALESCE(?, name),
			default_currency = COALESCE(?, default_currency),
			default_split    = CASE WHEN ? THEN ? ELSE default_split END,
			created_by       = COALESCE(?, created_by)
		WHERE id = ?
		RETURNING id, name, default_currency, created_by, created_at, default_split
	`, in.Name, in.DefaultCurrency, splitProvided, splitJSON, in.CreatedBy, id).
		Scan(&g.ID, &g.Name, &g.DefaultCurrency, &g.CreatedBy, &created, &rawSplit)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g.CreatedAt = scanTS(created)
	if g.DefaultSplit, err = repo.ScanDefaultSplit(rawSplit); err != nil {
		return nil, err
	}
	return &g, nil
}

// ClearDefaultSplit unconditionally nulls out the default_split column. Used
// by the service when a 3rd member joins a group with a pinned default.
func (r *GroupRepo) ClearDefaultSplit(ctx context.Context, id uuid.UUID) error {
	_, err := r.s.db.ExecContext(ctx, `UPDATE groups SET default_split = NULL WHERE id = ?`, id)
	return err
}

// Delete removes the group. Cascades to members, expenses, splits, settlements, recurring.
func (r *GroupRepo) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.s.db.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// ListAll returns a paginated view of all groups in the instance with member
// and (non-deleted) expense counts. Admin-only.
func (r *GroupRepo) ListAll(ctx context.Context, limit, offset int) ([]repo.AdminGroupRow, int, error) {
	var total int
	if err := r.s.db.QueryRowContext(ctx, `SELECT count(*) FROM groups`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.default_currency, g.created_by, g.created_at, g.default_split,
		       (SELECT count(*) FROM group_members  m WHERE m.group_id = g.id),
		       (SELECT count(*) FROM expenses       e WHERE e.group_id = g.id AND e.deleted_at IS NULL)
		FROM groups g
		ORDER BY g.created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []repo.AdminGroupRow
	for rows.Next() {
		var row repo.AdminGroupRow
		var created string
		var rawSplit []byte
		if err := rows.Scan(&row.ID, &row.Name, &row.DefaultCurrency, &row.CreatedBy, &created, &rawSplit,
			&row.MemberCount, &row.ExpenseCount); err != nil {
			return nil, 0, err
		}
		row.CreatedAt = scanTS(created)
		if row.DefaultSplit, err = repo.ScanDefaultSplit(rawSplit); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// ListMembers returns members + their display names for a group.
func (r *GroupRepo) ListMembers(ctx context.Context, groupID uuid.UUID) ([]repo.GroupMember, error) {
	rows, err := r.s.db.QueryContext(ctx, `
		SELECT m.group_id, m.user_id, u.display_name, m.joined_at, u.avatar_updated_at, u.deleted_at
		FROM group_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.group_id = ?
		ORDER BY m.joined_at
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.GroupMember
	for rows.Next() {
		var m repo.GroupMember
		var joined string
		var avatarUpdated, deleted *string
		if err := rows.Scan(&m.GroupID, &m.UserID, &m.DisplayName, &joined, &avatarUpdated, &deleted); err != nil {
			return nil, err
		}
		m.JoinedAt = scanTS(joined)
		m.AvatarUpdatedAt = scanTSPtr(avatarUpdated)
		m.DeletedAt = scanTSPtr(deleted)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *GroupRepo) IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.s.db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM group_members WHERE group_id = ? AND user_id = ?)
	`, groupID, userID).Scan(&exists)
	return exists, err
}

// HasTransactions reports whether the group has at least one non-deleted expense
// or settlement. Used to lock the default_currency once a ledger exists - the
// money columns store amounts in that currency, so swapping it would silently
// reinterpret historical totals.
func (r *GroupRepo) HasTransactions(ctx context.Context, groupID uuid.UUID) (bool, error) {
	var exists bool
	err := r.s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM expenses    WHERE group_id = ? AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM settlements WHERE group_id = ? AND deleted_at IS NULL
			LIMIT 1
		)
	`, groupID, groupID).Scan(&exists)
	return exists, err
}

// ShareAnyGroup reports whether two users are in at least one group together.
// Callers of /v1/users/{id}/avatar rely on this for authorization.
func (r *GroupRepo) ShareAnyGroup(ctx context.Context, a, b uuid.UUID) (bool, error) {
	if a == b {
		return true, nil
	}
	var exists bool
	err := r.s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM group_members ma
			JOIN group_members mb ON mb.group_id = ma.group_id
			WHERE ma.user_id = ? AND mb.user_id = ?
		)
	`, a, b).Scan(&exists)
	return exists, err
}

// RemoveMember deletes the membership row. Existing expenses, splits, and
// settlements remain - they reference users(id) directly, not the membership
// row, so the ledger is preserved. Returns ErrNotFound if the row didn't exist.
func (r *GroupRepo) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	res, err := r.s.db.ExecContext(ctx, `
		DELETE FROM group_members WHERE group_id = ? AND user_id = ?
	`, groupID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// AddMemberTx inserts a membership on the caller-owned transaction, idempotently.
// Unlike AddMember it does not read the row back; the importer ignores the
// returned member, so this avoids a needless round-trip and keeps the insert in
// the caller's transaction.
func (r *GroupRepo) AddMemberTx(ctx context.Context, tx repo.Tx, groupID, userID uuid.UUID) error {
	_, err := native(tx).tx.ExecContext(ctx, `
		INSERT INTO group_members (group_id, user_id, joined_at) VALUES (?, ?, ?)
		ON CONFLICT DO NOTHING
	`, groupID, userID, tsVal(time.Now().UTC()))
	return err
}

// AddMember inserts a membership. Returns the new member's row (with display name).
// If the user is already a member, returns the existing row.
func (r *GroupRepo) AddMember(ctx context.Context, groupID, userID uuid.UUID) (*repo.GroupMember, error) {
	_, err := r.s.db.ExecContext(ctx, `
		INSERT INTO group_members (group_id, user_id, joined_at) VALUES (?, ?, ?)
		ON CONFLICT DO NOTHING
	`, groupID, userID, tsVal(time.Now().UTC()))
	if err != nil {
		return nil, err
	}
	var m repo.GroupMember
	var joined string
	var avatarUpdated, deleted *string
	err = r.s.db.QueryRowContext(ctx, `
		SELECT m.group_id, m.user_id, u.display_name, m.joined_at, u.avatar_updated_at, u.deleted_at
		FROM group_members m JOIN users u ON u.id = m.user_id
		WHERE m.group_id = ? AND m.user_id = ?
	`, groupID, userID).Scan(&m.GroupID, &m.UserID, &m.DisplayName, &joined, &avatarUpdated, &deleted)
	if err != nil {
		return nil, err
	}
	m.JoinedAt = scanTS(joined)
	m.AvatarUpdatedAt = scanTSPtr(avatarUpdated)
	m.DeletedAt = scanTSPtr(deleted)
	return &m, nil
}
