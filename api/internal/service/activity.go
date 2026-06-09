package service

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

const (
	activityDefaultLimit = 50
	activityMaxLimit     = 100
)

type ActivityService struct {
	groups   *GroupService
	activity *repo.ActivityRepo
}

func NewActivityService(g *GroupService, a *repo.ActivityRepo) *ActivityService {
	return &ActivityService{groups: g, activity: a}
}

type ActivityPage struct {
	Items      []repo.ActivityHydrated
	NextCursor string
}

// List returns one page of the group activity feed, newest first. It enforces
// membership and emits an opaque cursor continuing strictly after the last row.
func (s *ActivityService) List(ctx context.Context, actorID, groupID uuid.UUID, limit int, cursor string) (*ActivityPage, error) {
	if err := s.groups.RequireMember(ctx, groupID, actorID); err != nil {
		return nil, err
	}
	after, err := decodeActivityCursor(cursor)
	if err != nil {
		return nil, ErrBadCursor
	}
	if limit <= 0 {
		limit = activityDefaultLimit
	}
	if limit > activityMaxLimit {
		limit = activityMaxLimit
	}
	rows, err := s.activity.ListByGroup(ctx, groupID, limit+1, after)
	if err != nil {
		return nil, err
	}
	page := &ActivityPage{}
	if len(rows) > limit {
		last := rows[limit-1]
		rows = rows[:limit]
		page.NextCursor = encodeActivityCursor(last)
	}
	page.Items = rows
	return page, nil
}

// Cursor format: base64url(created_at | id), created_at as RFC3339Nano.
func encodeActivityCursor(r repo.ActivityHydrated) string {
	raw := r.OccurredAt.UTC().Format(time.RFC3339Nano) + "|" + r.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeActivityCursor(s string) (*repo.ActivityRow, error) {
	if s == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return nil, errors.New("malformed cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, err
	}
	return &repo.ActivityRow{CreatedAt: createdAt, ID: id}, nil
}
