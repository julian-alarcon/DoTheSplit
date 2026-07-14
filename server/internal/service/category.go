package service

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/server/internal/repo"
)

// DefaultCategorySlug is used whenever a category isn't specified.
const DefaultCategorySlug = "other"

// ErrUnknownCategory is returned when a caller passes a category_id that
// doesn't exist in the seeded list.
var ErrUnknownCategory = errors.New("unknown category")

// CategoryService caches the seeded category set in memory. The table is
// effectively read-only in v1, so we load once and reuse.
type CategoryService struct {
	cats repo.CategoryRepo

	mu      sync.RWMutex
	loaded  bool
	byID    map[uuid.UUID]repo.Category
	bySlug  map[string]repo.Category
	ordered []repo.Category
}

func NewCategoryService(c repo.CategoryRepo) *CategoryService {
	return &CategoryService{cats: c}
}

func (s *CategoryService) load(ctx context.Context) error {
	s.mu.RLock()
	loaded := s.loaded
	s.mu.RUnlock()
	if loaded {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return nil
	}
	list, err := s.cats.List(ctx)
	if err != nil {
		return err
	}
	byID := make(map[uuid.UUID]repo.Category, len(list))
	bySlug := make(map[string]repo.Category, len(list))
	for _, c := range list {
		byID[c.ID] = c
		bySlug[c.Slug] = c
	}
	s.byID, s.bySlug, s.ordered, s.loaded = byID, bySlug, list, true
	return nil
}

func (s *CategoryService) List(ctx context.Context) ([]repo.Category, error) {
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	return s.ordered, nil
}

// Resolve returns the category with the given ID, or the default ("other") when id is nil.
func (s *CategoryService) Resolve(ctx context.Context, id *uuid.UUID) (repo.Category, error) {
	if err := s.load(ctx); err != nil {
		return repo.Category{}, err
	}
	if id == nil {
		return s.bySlug[DefaultCategorySlug], nil
	}
	c, ok := s.byID[*id]
	if !ok {
		return repo.Category{}, ErrUnknownCategory
	}
	return c, nil
}

// DefaultID returns the UUID of the default ("other") category.
func (s *CategoryService) DefaultID(ctx context.Context) (uuid.UUID, error) {
	if err := s.load(ctx); err != nil {
		return uuid.Nil, err
	}
	return s.bySlug[DefaultCategorySlug].ID, nil
}
