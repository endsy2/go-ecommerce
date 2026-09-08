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

type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

// List returns one page of products, newest first, and the unpaged total.
//
// Preload("Category") is what keeps this to a fixed number of queries: GORM
// collects the category ids from the page and fetches them all with one
// WHERE id IN (...). Reading product.Category inside a loop instead would be
// the N+1 CLAUDE.md tells us to flag.
//
// Soft-deleted products are excluded automatically — model.Product carries a
// gorm.DeletedAt, so GORM appends `deleted_at IS NULL` to both statements.
func (r *ProductRepository) List(ctx context.Context, limit, offset int) ([]model.Product, int64, error) {
	var (
		products []model.Product
		total    int64
	)

	if err := r.db.WithContext(ctx).Model(&model.Product{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting products: %w", err)
	}

	err := r.db.WithContext(ctx).
		Preload("Category").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&products).Error
	if err != nil {
		return nil, 0, fmt.Errorf("listing products: %w", err)
	}

	return products, total, nil
}

// FindBySlug looks a product up by the identifier the storefront uses.
func (r *ProductRepository) FindBySlug(ctx context.Context, slug string) (*model.Product, error) {
	var product model.Product

	err := r.db.WithContext(ctx).
		Preload("Category").
		First(&product, "slug = ?", slug).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.NotFoundf("product not found")
	}
	if err != nil {
		return nil, fmt.Errorf("finding product by slug %q: %w", slug, err)
	}

	return &product, nil
}

// FindByID looks a product up by primary key, which is how admin writes address
// it — the id survives a rename, the slug is what a rename would have moved.
func (r *ProductRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	var product model.Product

	err := r.db.WithContext(ctx).
		Preload("Category").
		First(&product, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.NotFoundf("product not found")
	}
	if err != nil {
		return nil, fmt.Errorf("finding product by id %s: %w", id, err)
	}

	return &product, nil
}

// Create inserts a product, reading the generated id back into p.
func (r *ProductRepository) Create(ctx context.Context, p *model.Product) error {
	err := r.db.WithContext(ctx).Create(p).Error

	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.Conflictf("a product with slug %q already exists", p.Slug)
	}
	// The category was checked before this call, so a violation here means it
	// was deleted in between. Reporting it as a validation failure rather than
	// a 500 tells the caller the truth: the body no longer names a real row.
	if errors.Is(err, gorm.ErrForeignKeyViolated) {
		return domain.Validationf("category does not exist")
	}
	if err != nil {
		return fmt.Errorf("creating product: %w", err)
	}

	return nil
}

// Update replaces a product's mutable columns. Slug is not among them: it is
// fixed at creation so that existing links keep working.
//
// The map form matters here more than anywhere else in this package. Handed a
// struct, GORM would skip every zero value — and "stock is now 0" is exactly
// the update a shop makes when something sells out. It would be silently
// dropped.
func (r *ProductRepository) Update(ctx context.Context, p *model.Product) error {
	res := r.db.WithContext(ctx).
		Model(&model.Product{}).
		Where("id = ?", p.ID).
		Updates(map[string]any{
			"category_id": p.CategoryID,
			"name":        p.Name,
			"description": p.Description,
			"price_cents": p.PriceCents,
			"stock":       p.Stock,
		})

	if errors.Is(res.Error, gorm.ErrForeignKeyViolated) {
		return domain.Validationf("category does not exist")
	}
	if res.Error != nil {
		return fmt.Errorf("updating product %s: %w", p.ID, res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NotFoundf("product not found")
	}

	return nil
}

// Delete soft-deletes a product: GORM turns this into an UPDATE that stamps
// deleted_at, because model.Product carries a gorm.DeletedAt.
//
// Never a hard delete. order_items reference products with ON DELETE RESTRICT,
// so removing the row would either fail or, worse, take purchase history with
// it. A withdrawn product simply stops appearing in the catalogue.
func (r *ProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&model.Product{}, "id = ?", id)

	if res.Error != nil {
		return fmt.Errorf("deleting product %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NotFoundf("product not found")
	}

	return nil
}
