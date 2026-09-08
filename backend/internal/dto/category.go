package dto

import "time"

// CategoryResponse is the wire shape of a category.
type CategoryResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateCategoryRequest is the POST /api/v1/categories body.
//
// Slug is optional: leave it out and the server derives one from Name. It is
// accepted at all because a name does not always produce the URL a shop wants
// ("T-Shirts & Tops" becomes "t-shirts-tops"), and the slug is permanent, so
// this is the only chance to choose it.
type CreateCategoryRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
	Slug string `json:"slug" binding:"omitempty,min=1,max=100"`
}

// UpdateCategoryRequest is the PUT /api/v1/categories/:id body.
//
// There is no Slug field. The slug is frozen after creation — changing it would
// break every link already pointing at the category.
type UpdateCategoryRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
}
