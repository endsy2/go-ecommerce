package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ecommerce/backend/internal/domain"
	"ecommerce/backend/internal/model"
)

type CategoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// List returns one page of categories ordered by name, and the unpaged total.
//
// Two queries rather than one. The page and the count answer different
// questions, and a window function to get both would repeat the total on every
// row. This is a fixed pair, not an N+1: it costs two queries whether the page
// holds one row or a hundred.
func (r *CategoryRepository) List(ctx context.Context, limit, offset int) ([]model.Category, int64, error) {
	var (
		categories []model.Category
		total      int64
	)

	if err := r.db.WithContext(ctx).Model(&model.Category{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting categories: %w", err)
	}

	err := r.db.WithContext(ctx).
		Order("name ASC").
		Limit(limit).
		Offset(offset).
		Find(&categories).Error
	if err != nil {
		return nil, 0, fmt.Errorf("listing categories: %w", err)
	}

	return categories, total, nil
}

// FindBySlug looks a category up by its public identifier.
func (r *CategoryRepository) FindBySlug(ctx context.Context, slug string) (*model.Category, error) {
	var category model.Category

	err := r.db.WithContext(ctx).First(&category, "slug = ?", slug).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.NotFoundf("category not found")
	}
	if err != nil {
		return nil, fmt.Errorf("finding category by slug %q: %w", slug, err)
	}

	return &category, nil
}

// FindByID looks a category up by primary key, which is how admin writes and
// the product service address it.
func (r *CategoryRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	var category model.Category

	err := r.db.WithContext(ctx).First(&category, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.NotFoundf("category not found")
	}
	if err != nil {
		return nil, fmt.Errorf("finding category by id %s: %w", id, err)
	}

	return &category, nil
}

// Create inserts a category, reading the generated id back into c.
func (r *CategoryRepository) Create(ctx context.Context, c *model.Category) error {
	err := r.db.WithContext(ctx).Create(c).Error

	// TranslateError is on in database.NewPostgres, so a unique violation
	// arrives as this sentinel instead of a driver error carrying "23505".
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.Conflictf("a category with slug %q already exists", c.Slug)
	}
	if err != nil {
		return fmt.Errorf("creating category: %w", err)
	}

	return nil
}

// Update changes a category's mutable columns.
//
// The updates are given as a map, not a struct. GORM skips zero-valued fields
// when it is handed a struct, so a struct-based update could never clear a
// field or set a number to 0 — a trap worth avoiding everywhere, and a real
// one for the product repository below where stock is legitimately zero.
func (r *CategoryRepository) Update(ctx context.Context, c *model.Category) error {
	res := r.db.WithContext(ctx).
		Model(&model.Category{}).
		Where("id = ?", c.ID).
		Updates(map[string]any{"name": c.Name})

	if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
		return domain.Conflictf("a category with that name already exists")
	}
	if res.Error != nil {
		return fmt.Errorf("updating category %s: %w", c.ID, res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NotFoundf("category not found")
	}

	return nil
}

// Delete removes a category outright — categories are not soft-deleted.
//
// products.category_id is ON DELETE RESTRICT, so deleting a category that still
// has products fails at the database rather than orphaning the catalogue. That
// refusal is the correct answer to the request, so it becomes a 409 instead of
// a 500.
func (r *CategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&model.Category{}, "id = ?", id)

	if errors.Is(res.Error, gorm.ErrForeignKeyViolated) {
		return domain.Conflictf("category still has products")
	}
	if res.Error != nil {
		return fmt.Errorf("deleting category %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NotFoundf("category not found")
	}

	return nil
}
