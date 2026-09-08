package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ecommerce/backend/internal/domain"
	"ecommerce/backend/internal/model"
)

// seedCategory inserts a category and returns it with the id Postgres generated.
func seedCategory(t *testing.T, db *gorm.DB, name, slug string) *model.Category {
	t.Helper()

	c := &model.Category{Name: name, Slug: slug}
	if err := db.Create(c).Error; err != nil {
		t.Fatalf("seeding category %q: %v", slug, err)
	}
	return c
}

// seedProduct inserts a product in the given category.
func seedProduct(t *testing.T, db *gorm.DB, categoryID uuid.UUID, name, slug string, stock int) *model.Product {
	t.Helper()

	p := &model.Product{
		CategoryID: categoryID,
		Name:       name,
		Slug:       slug,
		PriceCents: 1999,
		Stock:      stock,
	}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("seeding product %q: %v", slug, err)
	}
	return p
}

// TestProductRepositoryCatalogueReads covers the public read paths.
//
// One container shared by subtests rather than one per test: starting Postgres
// is the expensive part, and these cases only read, so they cannot disturb each
// other.
func TestProductRepositoryCatalogueReads(t *testing.T) {
	db := newTestDB(t)
	repo := NewProductRepository(db)
	ctx := context.Background()

	category := seedCategory(t, db, "Kitchen", "kitchen")
	mug := seedProduct(t, db, category.ID, "Blue Mug", "blue-mug", 5)
	seedProduct(t, db, category.ID, "Red Mug", "red-mug", 5)
	seedProduct(t, db, category.ID, "Green Mug", "green-mug", 5)

	t.Run("a page is limited while the total stays unpaged", func(t *testing.T) {
		products, total, err := repo.List(ctx, 2, 0)
		if err != nil {
			t.Fatalf("listing products: %v", err)
		}
		if len(products) != 2 {
			t.Errorf("got %d products on the page, want 2", len(products))
		}
		// The count must describe the whole catalogue, not the page — it is
		// what the client turns into a page count.
		if total != 3 {
			t.Errorf("got total %d, want 3", total)
		}
	})

	t.Run("the second page continues where the first stopped", func(t *testing.T) {
		first, _, err := repo.List(ctx, 2, 0)
		if err != nil {
			t.Fatalf("listing page 1: %v", err)
		}
		second, _, err := repo.List(ctx, 2, 2)
		if err != nil {
			t.Fatalf("listing page 2: %v", err)
		}

		// Asserted as a set rather than by position: the ordering between rows
		// created in the same millisecond is not something this test should
		// depend on. What matters is that paging visits every row once.
		seen := make(map[uuid.UUID]int)
		for _, p := range append(first, second...) {
			seen[p.ID]++
		}
		if len(seen) != 3 {
			t.Errorf("paging returned %d distinct products, want 3", len(seen))
		}
		for id, n := range seen {
			if n != 1 {
				t.Errorf("product %s appeared %d times across two pages", id, n)
			}
		}
	})

	t.Run("find by slug preloads the category", func(t *testing.T) {
		found, err := repo.FindBySlug(ctx, "blue-mug")
		if err != nil {
			t.Fatalf("finding by slug: %v", err)
		}
		if found.ID != mug.ID {
			t.Errorf("got product %s, want %s", found.ID, mug.ID)
		}
		if found.Category == nil {
			t.Fatal("category was not preloaded; rendering it would be an N+1")
		}
		if found.Category.Slug != "kitchen" {
			t.Errorf("got category %q, want kitchen", found.Category.Slug)
		}
	})

	t.Run("an unknown slug is a domain not-found", func(t *testing.T) {
		_, err := repo.FindBySlug(ctx, "no-such-product")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got error %v, want domain.ErrNotFound", err)
		}
		// Nothing above the repository may have to know what an ORM is.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			t.Error("the gorm error leaked out of the repository layer")
		}
	})
}

func TestProductRepositoryWrites(t *testing.T) {
	db := newTestDB(t)
	repo := NewProductRepository(db)
	ctx := context.Background()

	category := seedCategory(t, db, "Kitchen", "kitchen")

	t.Run("a duplicate slug is a conflict", func(t *testing.T) {
		seedProduct(t, db, category.ID, "Taken", "taken-slug", 1)

		err := repo.Create(ctx, &model.Product{
			CategoryID: category.ID,
			Name:       "Another",
			Slug:       "taken-slug",
			PriceCents: 100,
			Stock:      1,
		})
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("got error %v, want domain.ErrConflict", err)
		}
	})

	t.Run("a category that does not exist is a validation error", func(t *testing.T) {
		err := repo.Create(ctx, &model.Product{
			CategoryID: uuid.New(),
			Name:       "Orphan",
			Slug:       "orphan",
			PriceCents: 100,
			Stock:      1,
		})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("got error %v, want domain.ErrValidation", err)
		}
	})

	t.Run("an update can set stock to zero", func(t *testing.T) {
		// The regression this guards: handed a struct, GORM skips zero-valued
		// fields, so "it sold out" would be silently dropped. The repository
		// uses a column map precisely so this write lands.
		p := seedProduct(t, db, category.ID, "Sells Out", "sells-out", 7)

		p.Stock = 0
		p.PriceCents = 0
		if err := repo.Update(ctx, p); err != nil {
			t.Fatalf("updating product: %v", err)
		}

		reloaded, err := repo.FindByID(ctx, p.ID)
		if err != nil {
			t.Fatalf("reloading product: %v", err)
		}
		if reloaded.Stock != 0 {
			t.Errorf("got stock %d, want 0", reloaded.Stock)
		}
		if reloaded.PriceCents != 0 {
			t.Errorf("got price %d, want 0", reloaded.PriceCents)
		}
	})

	t.Run("updating a product that does not exist is a not-found", func(t *testing.T) {
		missing := &model.Product{Base: model.Base{ID: uuid.New()}, CategoryID: category.ID, Name: "Ghost"}
		if err := repo.Update(ctx, missing); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got error %v, want domain.ErrNotFound", err)
		}
	})

	t.Run("delete is soft and removes the product from the catalogue", func(t *testing.T) {
		p := seedProduct(t, db, category.ID, "Withdrawn", "withdrawn", 3)

		if err := repo.Delete(ctx, p.ID); err != nil {
			t.Fatalf("deleting product: %v", err)
		}

		if _, err := repo.FindByID(ctx, p.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("a deleted product is still readable: %v", err)
		}

		// The row must still be there: order_items reference products, so a
		// past order has to stay readable after the product is withdrawn.
		var survivor model.Product
		if err := db.Unscoped().First(&survivor, "id = ?", p.ID).Error; err != nil {
			t.Fatalf("the row was hard-deleted: %v", err)
		}
		if !survivor.DeletedAt.Valid {
			t.Error("deleted_at was not stamped")
		}
	})

	t.Run("deleting a product that does not exist is a not-found", func(t *testing.T) {
		if err := repo.Delete(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got error %v, want domain.ErrNotFound", err)
		}
	})
}

func TestCategoryRepositoryDelete(t *testing.T) {
	db := newTestDB(t)
	repo := NewCategoryRepository(db)
	ctx := context.Background()

	t.Run("a category that still has products cannot be deleted", func(t *testing.T) {
		// products.category_id is ON DELETE RESTRICT, declared in migration
		// 000003. This test only means anything because newTestDB applies the
		// real migrations: a bare AutoMigrate would not create that rule.
		category := seedCategory(t, db, "Kitchen", "kitchen")
		seedProduct(t, db, category.ID, "Blue Mug", "blue-mug", 1)

		err := repo.Delete(ctx, category.ID)
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("got error %v, want domain.ErrConflict", err)
		}
	})

	t.Run("an empty category is deleted", func(t *testing.T) {
		category := seedCategory(t, db, "Empty", "empty")

		if err := repo.Delete(ctx, category.ID); err != nil {
			t.Fatalf("deleting an empty category: %v", err)
		}
		if _, err := repo.FindByID(ctx, category.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("the category is still readable: %v", err)
		}
	})

	t.Run("deleting a category that does not exist is a not-found", func(t *testing.T) {
		if err := repo.Delete(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("got error %v, want domain.ErrNotFound", err)
		}
	})
}
