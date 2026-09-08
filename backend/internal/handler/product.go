package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ecommerce/backend/internal/dto"
	"ecommerce/backend/internal/model"
	"ecommerce/backend/internal/service"
)

// productService is the slice of service.ProductService this handler uses.
//
// service.ProductInput appears in the signature, so this interface names a
// service type. That is the allowed direction — handler already imports service
// to call it, and the alternative, redeclaring the same six fields in handler,
// would be two structs to keep in step for no gain.
type productService interface {
	List(ctx context.Context, limit, offset int) ([]model.Product, int64, error)
	GetBySlug(ctx context.Context, slug string) (*model.Product, error)
	Create(ctx context.Context, in service.ProductInput) (*model.Product, error)
	Update(ctx context.Context, id uuid.UUID, in service.ProductInput) (*model.Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ProductHandler struct {
	products productService
}

func NewProductHandler(products productService) *ProductHandler {
	return &ProductHandler{products: products}
}

// List answers GET /api/v1/products. Public, paginated, capped at 100 per page.
func (h *ProductHandler) List(c *gin.Context) {
	page, err := ParsePagination(c)
	if err != nil {
		HandleError(c, err)
		return
	}

	products, total, err := h.products.List(c.Request.Context(), page.Limit, page.Offset())
	if err != nil {
		HandleError(c, err)
		return
	}

	body := make([]dto.ProductResponse, 0, len(products))
	for i := range products {
		body = append(body, newProductResponse(&products[i]))
	}

	RespondList(c, body, NewMeta(page, total))
}

// GetBySlug answers GET /api/v1/products/:slug. Public.
func (h *ProductHandler) GetBySlug(c *gin.Context) {
	product, err := h.products.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		HandleError(c, err)
		return
	}

	OK(c, newProductResponse(product))
}

// Create answers POST /api/v1/products. Admin only.
func (h *ProductHandler) Create(c *gin.Context) {
	var req dto.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleBindingError(c, err)
		return
	}

	// The uuid tag has already proved this parses, so the error cannot fire.
	categoryID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		HandleError(c, err)
		return
	}

	product, err := h.products.Create(c.Request.Context(), service.ProductInput{
		CategoryID:  categoryID,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		// Safe to dereference: both fields are `binding:"required"`, so binding
		// has already rejected a body that omitted them.
		PriceCents: *req.PriceCents,
		Stock:      *req.Stock,
	})
	if err != nil {
		HandleError(c, err)
		return
	}

	Created(c, newProductResponse(product))
}

// Update answers PUT /api/v1/products/:id. Admin only.
func (h *ProductHandler) Update(c *gin.Context) {
	id, err := uuidParam(c, "id")
	if err != nil {
		HandleError(c, err)
		return
	}

	var req dto.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleBindingError(c, err)
		return
	}

	categoryID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		HandleError(c, err)
		return
	}

	product, err := h.products.Update(c.Request.Context(), id, service.ProductInput{
		CategoryID:  categoryID,
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  *req.PriceCents,
		Stock:       *req.Stock,
	})
	if err != nil {
		HandleError(c, err)
		return
	}

	OK(c, newProductResponse(product))
}

// Delete answers DELETE /api/v1/products/:id. Admin only. Soft delete.
func (h *ProductHandler) Delete(c *gin.Context) {
	id, err := uuidParam(c, "id")
	if err != nil {
		HandleError(c, err)
		return
	}

	if err := h.products.Delete(c.Request.Context(), id); err != nil {
		HandleError(c, err)
		return
	}

	NoContent(c)
}

// newProductResponse maps a model to its wire shape.
//
// PriceCents crosses as an int64 named for its unit. No formatting happens
// here: CLAUDE.md puts that at the client, so the API never has to guess a
// currency or a locale.
func newProductResponse(m *model.Product) dto.ProductResponse {
	res := dto.ProductResponse{
		ID:          m.ID.String(),
		CategoryID:  m.CategoryID.String(),
		Name:        m.Name,
		Slug:        m.Slug,
		Description: m.Description,
		PriceCents:  m.PriceCents,
		Stock:       m.Stock,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}

	// Nil whenever the query did not Preload("Category"), which is why the
	// field is a pointer: an unloaded association is absent from the JSON, not
	// an object full of empty strings pretending to be a category.
	if m.Category != nil {
		category := newCategoryResponse(m.Category)
		res.Category = &category
	}

	return res
}
