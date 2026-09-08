package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"ecommerce/backend/internal/domain"
	"ecommerce/backend/internal/model"
	"ecommerce/backend/internal/util"
)

// productStore is the slice of repository.ProductRepository this service uses.
type productStore interface {
	List(ctx context.Context, limit, offset int) ([]model.Product, int64, error)
	FindBySlug(ctx context.Context, slug string) (*model.Product, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error)
	Create(ctx context.Context, p *model.Product) error
	Update(ctx context.Context, p *model.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// categoryLookup is deliberately narrower than categoryStore: this service only
// ever needs to check that a category exists. Asking for one method instead of
// six is what keeps a test fake three lines long.
type categoryLookup interface {
	FindByID(ctx context.Context, id uuid.UUID) (*model.Category, error)
}

// ProductInput is the validated data a product is created or replaced with.
//
// A struct rather than six positional parameters, so a call site cannot swap
// PriceCents and Stock — both are numbers, and the compiler would not notice.
type ProductInput struct {
	CategoryID  uuid.UUID
	Name        string
	Slug        string
	Description string
	PriceCents  int64
	Stock       int
}

type ProductService struct {
	products   productStore
	categories categoryLookup
}

func NewProductService(products productStore, categories categoryLookup) *ProductService {
	return &ProductService{products: products, categories: categories}
}

// List returns one page of the catalogue.
func (s *ProductService) List(ctx context.Context, limit, offset int) ([]model.Product, int64, error) {
	return s.products.List(ctx, limit, offset)
}

// GetBySlug returns the product behind a storefront URL.
func (s *ProductService) GetBySlug(ctx context.Context, slug string) (*model.Product, error) {
	return s.products.FindBySlug(ctx, slug)
}

// Create adds a product to the catalogue.
func (s *ProductService) Create(ctx context.Context, in ProductInput) (*model.Product, error) {
	if err := s.requireCategory(ctx, in.CategoryID); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(in.Name)

	slug := in.Slug
	if strings.TrimSpace(slug) == "" {
		slug = name
	}
	slug = util.Slugify(slug)

	if slug == "" {
		return nil, domain.Validationf("name must contain at least one letter or digit")
	}

	product := &model.Product{
		CategoryID:  in.CategoryID,
		Name:        name,
		Slug:        slug,
		Description: strings.TrimSpace(in.Description),
		PriceCents:  in.PriceCents,
		Stock:       in.Stock,
	}
	if err := s.products.Create(ctx, product); err != nil {
		return nil, err
	}

	// Re-read so the response carries the preloaded category and the
	// database-generated timestamps.
	return s.products.FindByID(ctx, product.ID)
}

// Update replaces a product. The slug is untouched, so links keep working.
func (s *ProductService) Update(ctx context.Context, id uuid.UUID, in ProductInput) (*model.Product, error) {
	if _, err := s.products.FindByID(ctx, id); err != nil {
		return nil, err
	}
	if err := s.requireCategory(ctx, in.CategoryID); err != nil {
		return nil, err
	}

	updated := &model.Product{
		Base:        model.Base{ID: id},
		CategoryID:  in.CategoryID,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		PriceCents:  in.PriceCents,
		Stock:       in.Stock,
	}
	if err := s.products.Update(ctx, updated); err != nil {
		return nil, err
	}

	return s.products.FindByID(ctx, id)
}

// Delete withdraws a product from sale. The repository soft-deletes it.
func (s *ProductService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.products.Delete(ctx, id)
}

// requireCategory checks the category named in a request body exists.
//
// A missing one becomes a 422, not a 404: the product endpoint was found, and
// it is the body that is unprocessable. Returning 404 here would suggest the
// product itself does not exist, which on a create is nonsense.
func (s *ProductService) requireCategory(ctx context.Context, id uuid.UUID) error {
	_, err := s.categories.FindByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Validationf("category does not exist")
	}
	return err
}
