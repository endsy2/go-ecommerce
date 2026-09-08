package handler

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ecommerce/backend/internal/dto"
	"ecommerce/backend/internal/model"
)

// categoryService is the slice of service.CategoryService this handler uses.
type categoryService interface {
	List(ctx context.Context, limit, offset int) ([]model.Category, int64, error)
	GetBySlug(ctx context.Context, slug string) (*model.Category, error)
	Create(ctx context.Context, name, slug string) (*model.Category, error)
	Update(ctx context.Context, id uuid.UUID, name string) (*model.Category, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type CategoryHandler struct {
	categories categoryService
}

func NewCategoryHandler(categories categoryService) *CategoryHandler {
	return &CategoryHandler{categories: categories}
}

// List answers GET /api/v1/categories. Public.
func (h *CategoryHandler) List(c *gin.Context) {
	page, err := ParsePagination(c)
	if err != nil {
		HandleError(c, err)
		return
	}

	categories, total, err := h.categories.List(c.Request.Context(), page.Limit, page.Offset())
	if err != nil {
		HandleError(c, err)
		return
	}

	// Built with make(..., 0, len) so an empty page marshals as [] rather than
	// null. A JSON null where the client expects an array is a frontend crash
	// that only shows up on an empty database.
	body := make([]dto.CategoryResponse, 0, len(categories))
	for i := range categories {
		body = append(body, newCategoryResponse(&categories[i]))
	}

	RespondList(c, body, NewMeta(page, total))
}

// GetBySlug answers GET /api/v1/categories/:slug. Public.
func (h *CategoryHandler) GetBySlug(c *gin.Context) {
	category, err := h.categories.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		HandleError(c, err)
		return
	}

	OK(c, newCategoryResponse(category))
}

// Create answers POST /api/v1/categories. Admin only.
func (h *CategoryHandler) Create(c *gin.Context) {
	var req dto.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleBindingError(c, err)
		return
	}

	category, err := h.categories.Create(c.Request.Context(), req.Name, req.Slug)
	if err != nil {
		HandleError(c, err)
		return
	}

	Created(c, newCategoryResponse(category))
}

// Update answers PUT /api/v1/categories/:id. Admin only.
func (h *CategoryHandler) Update(c *gin.Context) {
	id, err := uuidParam(c, "id")
	if err != nil {
		HandleError(c, err)
		return
	}

	var req dto.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleBindingError(c, err)
		return
	}

	category, err := h.categories.Update(c.Request.Context(), id, req.Name)
	if err != nil {
		HandleError(c, err)
		return
	}

	OK(c, newCategoryResponse(category))
}

// Delete answers DELETE /api/v1/categories/:id. Admin only.
func (h *CategoryHandler) Delete(c *gin.Context) {
	id, err := uuidParam(c, "id")
	if err != nil {
		HandleError(c, err)
		return
	}

	if err := h.categories.Delete(c.Request.Context(), id); err != nil {
		HandleError(c, err)
		return
	}

	NoContent(c)
}

// newCategoryResponse maps a model to its wire shape, in the handler layer so
// that dto never imports model.
func newCategoryResponse(m *model.Category) dto.CategoryResponse {
	return dto.CategoryResponse{
		ID:        m.ID.String(),
		Name:      m.Name,
		Slug:      m.Slug,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
