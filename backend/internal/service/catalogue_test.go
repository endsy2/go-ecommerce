package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"ecommerce/backend/internal/domain"
	"ecommerce/backend/internal/model"
)

// The fakes below follow the same pattern as fakeUserFinder in auth_test.go:
// ordinary structs, no mocking framework, no generated code. Each records what
// it was handed, so a test can assert WHAT the service decided to store rather
// than merely that it stored something.

type fakeProductStore struct {
	existing  *model.Product
	findErr   error
	createErr error
	updateErr error
	deleteErr error

	created *model.Product
	updated *model.Product
	deleted uuid.UUID
}

func (f *fakeProductStore) List(context.Context, int, int) ([]model.Product, int64, error) {
	if f.existing == nil {
		return nil, 0, f.findErr
	}
	return []model.Product{*f.existing}, 1, f.findErr
}

func (f *fakeProductStore) FindBySlug(context.Context, string) (*model.Product, error) {
	return f.existing, f.findErr
}

// FindByID falls back to the product just created, because the service re-reads
// after a write to pick up database-generated values.
func (f *fakeProductStore) FindByID(context.Context, uuid.UUID) (*model.Product, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.existing != nil {
		return f.existing, nil
	}
	return f.created, nil
}

func (f *fakeProductStore) Create(_ context.Context, p *model.Product) error {
	if f.createErr != nil {
		return f.createErr
	}
	p.ID = uuid.New()
	f.created = p
	return nil
}

func (f *fakeProductStore) Update(_ context.Context, p *model.Product) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = p
	return nil
}

func (f *fakeProductStore) Delete(_ context.Context, id uuid.UUID) error {
	f.deleted = id
	return f.deleteErr
}

type fakeCategoryLookup struct {
	category *model.Category
	err      error
}

func (f fakeCategoryLookup) FindByID(context.Context, uuid.UUID) (*model.Category, error) {
	return f.category, f.err
}

func testCategory() *model.Category {
	c := &model.Category{Name: "Kitchen", Slug: "kitchen"}
	c.ID = uuid.New()
	return c
}

func TestProductServiceCreate(t *testing.T) {
	category := testCategory()

	tests := []struct {
		name       string
		input      ProductInput
		lookup     fakeCategoryLookup
		wantSlug   string
		wantPrice  int64
		wantStock  int
		wantErrIs  error
		wantNoCall bool
	}{
		{
			name:      "slug is derived from the name",
			input:     ProductInput{Name: "Blue Ceramic Mug!", PriceCents: 1999, Stock: 4},
			lookup:    fakeCategoryLookup{category: category},
			wantSlug:  "blue-ceramic-mug",
			wantPrice: 1999,
			wantStock: 4,
		},
		{
			// A client may choose the slug, but it still goes through Slugify:
			// it is going into a URL either way.
			name:      "a supplied slug is normalised",
			input:     ProductInput{Name: "Mug", Slug: "Garden Tools", PriceCents: 500, Stock: 1},
			lookup:    fakeCategoryLookup{category: category},
			wantSlug:  "garden-tools",
			wantPrice: 500,
			wantStock: 1,
		},
		{
			// Zero is a real price and a real stock level. This is the case a
			// binding:"required" on a plain int64 would have rejected, and the
			// reason both dto fields are pointers.
			name:      "zero price and zero stock are stored, not treated as missing",
			input:     ProductInput{Name: "Free Sample", PriceCents: 0, Stock: 0},
			lookup:    fakeCategoryLookup{category: category},
			wantSlug:  "free-sample",
			wantPrice: 0,
			wantStock: 0,
		},
		{
			name:       "a name with no usable characters is rejected",
			input:      ProductInput{Name: "!!!", PriceCents: 100, Stock: 1},
			lookup:     fakeCategoryLookup{category: category},
			wantErrIs:  domain.ErrValidation,
			wantNoCall: true,
		},
		{
			// The category is a field in the request body, so a missing one is
			// an unprocessable body (422), not a missing product (404).
			name:       "an unknown category is a validation error, not a 404",
			input:      ProductInput{Name: "Mug", PriceCents: 100, Stock: 1},
			lookup:     fakeCategoryLookup{err: domain.NotFoundf("category not found")},
			wantErrIs:  domain.ErrValidation,
			wantNoCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeProductStore{}
			svc := NewProductService(store, tt.lookup)

			tt.input.CategoryID = category.ID
			product, err := svc.Create(context.Background(), tt.input)

			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("got error %v, want %v", err, tt.wantErrIs)
				}
				if tt.wantNoCall && store.created != nil {
					t.Error("the product was stored despite the request being rejected")
				}
				return
			}

			if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			}
			if store.created == nil {
				t.Fatal("nothing was handed to the repository")
			}
			if store.created.Slug != tt.wantSlug {
				t.Errorf("got slug %q, want %q", store.created.Slug, tt.wantSlug)
			}
			if store.created.PriceCents != tt.wantPrice {
				t.Errorf("got price %d cents, want %d", store.created.PriceCents, tt.wantPrice)
			}
			if store.created.Stock != tt.wantStock {
				t.Errorf("got stock %d, want %d", store.created.Stock, tt.wantStock)
			}
			if product == nil {
				t.Error("got a nil product on success")
			}
		})
	}
}

// TestProductServiceUpdateLeavesTheSlugAlone is the frozen-slug guard.
//
// A separate test rather than a table row: the property is about what the
// service does NOT do, which no single field of a success case can express.
func TestProductServiceUpdateLeavesTheSlugAlone(t *testing.T) {
	category := testCategory()
	existing := &model.Product{Name: "Blue Mug", Slug: "blue-mug", PriceCents: 999}
	existing.ID = uuid.New()

	store := &fakeProductStore{existing: existing}
	svc := NewProductService(store, fakeCategoryLookup{category: category})

	_, err := svc.Update(context.Background(), existing.ID, ProductInput{
		CategoryID: category.ID,
		Name:       "Navy Mug",
		PriceCents: 1299,
		Stock:      2,
	})
	if err != nil {
		t.Fatalf("got unexpected error: %v", err)
	}

	if store.updated == nil {
		t.Fatal("nothing was handed to the repository")
	}
	if store.updated.Name != "Navy Mug" {
		t.Errorf("got name %q, want the renamed value", store.updated.Name)
	}
	// The service must never populate Slug on an update, and the repository's
	// column map has no slug key either. Together that is what keeps a link to
	// /products/blue-mug working after the rename.
	if store.updated.Slug != "" {
		t.Errorf("the update carried slug %q; renaming must not move the slug", store.updated.Slug)
	}
}

func TestProductServiceDeleteDelegatesTheID(t *testing.T) {
	existing := &model.Product{Name: "Blue Mug", Slug: "blue-mug"}
	existing.ID = uuid.New()

	store := &fakeProductStore{existing: existing}
	svc := NewProductService(store, fakeCategoryLookup{category: testCategory()})

	if err := svc.Delete(context.Background(), existing.ID); err != nil {
		t.Fatalf("got unexpected error: %v", err)
	}
	if store.deleted != existing.ID {
		t.Errorf("deleted %s, want %s", store.deleted, existing.ID)
	}
}

type fakeCategoryStore struct {
	existing  *model.Category
	findErr   error
	createErr error

	created *model.Category
}

func (f *fakeCategoryStore) List(context.Context, int, int) ([]model.Category, int64, error) {
	return nil, 0, f.findErr
}

func (f *fakeCategoryStore) FindBySlug(context.Context, string) (*model.Category, error) {
	return f.existing, f.findErr
}

func (f *fakeCategoryStore) FindByID(context.Context, uuid.UUID) (*model.Category, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.existing != nil {
		return f.existing, nil
	}
	return f.created, nil
}

func (f *fakeCategoryStore) Create(_ context.Context, c *model.Category) error {
	if f.createErr != nil {
		return f.createErr
	}
	c.ID = uuid.New()
	f.created = c
	return nil
}

func (f *fakeCategoryStore) Update(context.Context, *model.Category) error { return nil }
func (f *fakeCategoryStore) Delete(context.Context, uuid.UUID) error       { return nil }

func TestCategoryServiceCreate(t *testing.T) {
	tests := []struct {
		name      string
		inputName string
		inputSlug string
		storeErr  error
		wantSlug  string
		wantErrIs error
	}{
		{
			name:      "slug is derived from the name",
			inputName: "Garden Tools",
			wantSlug:  "garden-tools",
		},
		{
			name:      "a supplied slug is normalised",
			inputName: "T-Shirts & Tops",
			inputSlug: "T Shirts",
			wantSlug:  "t-shirts",
		},
		{
			name:      "a name with no usable characters is rejected",
			inputName: "***",
			wantErrIs: domain.ErrValidation,
		},
		{
			// The repository turns a unique violation into this. The service
			// must pass it through untouched rather than swallowing it into a
			// 500.
			name:      "a duplicate slug is reported as a conflict",
			inputName: "Kitchen",
			storeErr:  domain.Conflictf("a category with slug %q already exists", "kitchen"),
			wantErrIs: domain.ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeCategoryStore{createErr: tt.storeErr}
			svc := NewCategoryService(store)

			_, err := svc.Create(context.Background(), tt.inputName, tt.inputSlug)

			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("got error %v, want %v", err, tt.wantErrIs)
				}
				return
			}

			if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			}
			if store.created.Slug != tt.wantSlug {
				t.Errorf("got slug %q, want %q", store.created.Slug, tt.wantSlug)
			}
		})
	}
}
