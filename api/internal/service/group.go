package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/api/internal/crypto"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
)

var (
	ErrNotMember       = errors.New("user is not a group member")
	ErrInviteeNotFound = errors.New("invitee is not registered")
	ErrNotCreator      = errors.New("only the group creator can perform this action")
	ErrBadCurrency     = errors.New("default_currency must be a 3-letter code")
)

// DefaultGroupCurrency is used when a group is created without an explicit currency.
const DefaultGroupCurrency = "EUR"

type GroupService struct {
	groups *repo.GroupRepo
	users  *repo.UserRepo
	email  *crypto.EmailCipher
}

func NewGroupService(g *repo.GroupRepo, u *repo.UserRepo, e *crypto.EmailCipher) *GroupService {
	return &GroupService{groups: g, users: u, email: e}
}

// Create a group. The creator is auto-added as a member. Empty currency → DefaultGroupCurrency.
func (s *GroupService) Create(ctx context.Context, name, defaultCurrency string, creatorID uuid.UUID) (*repo.Group, []repo.GroupMember, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil, errors.New("name is required")
	}
	cur, err := normalizeCurrency(defaultCurrency)
	if err != nil {
		return nil, nil, err
	}
	if cur == "" {
		cur = DefaultGroupCurrency
	}
	g, err := s.groups.Create(ctx, name, cur, creatorID)
	if err != nil {
		return nil, nil, err
	}
	members, err := s.groups.ListMembers(ctx, g.ID)
	if err != nil {
		return nil, nil, err
	}
	return g, members, nil
}

// Update applies a partial update (name and/or default_currency). Any group member may update.
func (s *GroupService) Update(ctx context.Context, groupID, actorID uuid.UUID, name, defaultCurrency *string) (*repo.Group, []repo.GroupMember, error) {
	if err := s.RequireMember(ctx, groupID, actorID); err != nil {
		return nil, nil, err
	}
	if name == nil && defaultCurrency == nil {
		return nil, nil, errors.New("nothing to update")
	}
	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return nil, nil, errors.New("name cannot be empty")
		}
		name = &trimmed
	}
	if defaultCurrency != nil {
		cur, err := normalizeCurrency(*defaultCurrency)
		if err != nil {
			return nil, nil, err
		}
		if cur == "" {
			return nil, nil, ErrBadCurrency
		}
		defaultCurrency = &cur
	}
	g, err := s.groups.Update(ctx, groupID, name, defaultCurrency)
	if err != nil {
		return nil, nil, err
	}
	members, err := s.groups.ListMembers(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	return g, members, nil
}

// Delete removes the group. Only the creator may delete it. Cascades via FK.
func (s *GroupService) Delete(ctx context.Context, groupID, actorID uuid.UUID) error {
	g, err := s.groups.FindByID(ctx, groupID)
	if err != nil {
		return err
	}
	if g.CreatedBy != actorID {
		return ErrNotCreator
	}
	return s.groups.Delete(ctx, groupID)
}

// normalizeCurrency uppercases a 3-letter code. Empty input returns "".
func normalizeCurrency(cur string) (string, error) {
	cur = strings.TrimSpace(cur)
	if cur == "" {
		return "", nil
	}
	if len(cur) != 3 {
		return "", ErrBadCurrency
	}
	return strings.ToUpper(cur), nil
}

func (s *GroupService) List(ctx context.Context, userID uuid.UUID) ([]repo.Group, map[uuid.UUID][]repo.GroupMember, error) {
	groups, err := s.groups.ListForUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	membersByGroup := make(map[uuid.UUID][]repo.GroupMember, len(groups))
	for _, g := range groups {
		m, err := s.groups.ListMembers(ctx, g.ID)
		if err != nil {
			return nil, nil, err
		}
		membersByGroup[g.ID] = m
	}
	return groups, membersByGroup, nil
}

// RequireMember returns ErrNotMember if userID isn't in groupID.
func (s *GroupService) RequireMember(ctx context.Context, groupID, userID uuid.UUID) error {
	ok, err := s.groups.IsMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotMember
	}
	return nil
}

// ShareAnyGroup reports whether two users are in at least one group together.
func (s *GroupService) ShareAnyGroup(ctx context.Context, a, b uuid.UUID) (bool, error) {
	return s.groups.ShareAnyGroup(ctx, a, b)
}

// AddMember looks up the invitee by email_hash; 404 if unregistered.
// Only an existing group member may add others.
func (s *GroupService) AddMember(ctx context.Context, groupID, actorID uuid.UUID, email string) (*repo.GroupMember, error) {
	if err := s.RequireMember(ctx, groupID, actorID); err != nil {
		return nil, err
	}
	invitee, err := s.users.FindByEmailHash(ctx, s.email.HashEmail(email))
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrInviteeNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.groups.AddMember(ctx, groupID, invitee.ID)
}

// Get returns a group + its members, enforcing membership.
func (s *GroupService) Get(ctx context.Context, groupID, userID uuid.UUID) (*repo.Group, []repo.GroupMember, error) {
	if err := s.RequireMember(ctx, groupID, userID); err != nil {
		return nil, nil, err
	}
	g, err := s.groups.FindByID(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	members, err := s.groups.ListMembers(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	return g, members, nil
}
