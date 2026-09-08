package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"ecommerce/backend/internal/domain"
	"ecommerce/backend/internal/model"
	"ecommerce/backend/internal/util"
)

// categoryStore is the slice of repository.CategoryRepository this service
// uses. Declared here by the consumer, as service.userStore is: the service
// names what it needs, and a test satisfies it with a small struct instead of
// a database.
type categoryStore interface {
	List(ctx context.Context, limit, offset int) ([]model.Category, int64, error)
	FindBySlug(ctx context.Context, slug string) (*model.Category, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Category, error)
	Create(ctx context.Context, c *model.Category) error
	Update(ctx context.Context, c *model.Category) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type CategoryService struct {
	categories categoryStore
}

func NewCategoryService(categories categoryStore) *CategoryService {
	return &CategoryService{categories: categories}
}

// List returns one page of the category list.
func (s *CategoryService) List(ctx context.Context, limit, offset int) ([]model.Category, int64, error) {
	return s.categories.List(ctx, limit, offset)
}

// GetBySlug returns the category behind a storefront URL.
func (s *CategoryService) GetBySlug(ctx context.Context, slug string) (*model.Category, error) {
	return s.categories.FindBySlug(ctx, slug)
}

// Create adds a category, deriving the slug from the name when none was given.
//
// A caller-supplied slug is still run through Slugify: it is going into a URL,
// so "Garden Tools" must become "garden-tools" whether the client normalised it
// or not. Accepting it verbatim would let one client create a slug that no
// other client could produce.
func (s *CategoryService) Create(ctx context.Context, name, slug string) (*model.Category, error) {
	name = strings.TrimSpace(name)

	if strings.TrimSpace(slug) == "" {
		slug = name
	}
	slug = util.Slugify(slug)

	if slug == "" {
		return nil, domain.Validationf("name must contain at least one letter or digit")
	}

	category := &model.Category{Name: name, Slug: slug}
	if err := s.categories.Create(ctx, category); err != nil {
		return nil, err
	}

	return category, nil
}

// Update renames a category. The slug does not move — see UpdateCategoryRequest.
func (s *CategoryService) Update(ctx context.Context, id uuid.UUID, name string) (*model.Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, domain.Validationf("name must contain at least one letter or digit")
	}

	if err := s.categories.Update(ctx, &model.Category{Base: model.Base{ID: id}, Name: name}); err != nil {
		return nil, err
	}

	// Re-read rather than returning the struct we just built: the row now
	// carries a database-set updated_at, and the response should show what was
	// actually stored, not what we hoped to store.
	return s.categories.FindByID(ctx, id)
}

// Delete removes a category. The repository turns the ON DELETE RESTRICT
// refusal into a conflict, so a category with products cannot be removed.
func (s *CategoryService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.categories.Delete(ctx, id)
}
